---
title: Error Handling
slug: /error-handling
sidebar_position: 9
---

# Error Handling

The error-returning forms of `ServeHTTP` — and **every** Props method — run against a *buffered* response writer. On a non-nil error the buffer is discarded and the error goes to the `WithErrorHandler` callback, which renders a clean error page even if the handler had already written partial output. Everything in this guide follows from that one mechanism.

## The rules

1. **Never call `http.Error` (or write `w`) in an error-returning handler or in Props.** If you write then `return err`, the write is discarded; if you write then `return nil`, you bypass the error handler. Just return the error.
2. **For a specific status code, return a typed error** that the global handler unwraps with `errors.As`. Plain errors fall through to a logged 500.
3. **API/JSON endpoints use the no-error `ServeHTTP` form** — direct `w` writes are correct there because you own the status code and skip the buffering wrapper. Write JSON error bodies, not `http.Error`.
4. **For streaming (SSE), flush with `http.NewResponseController(w)`** — it works from either `ServeHTTP` form and is the only way to guarantee unbuffered delivery through middleware.

## Typed errors for status codes

Define one error type that carries the status and message; the global handler renders it:

```go
type ErrorWithStatus struct {
    Status  int
    Title   string
    Message string
}

func (e ErrorWithStatus) Error() string { return fmt.Sprintf("%d %s: %s", e.Status, e.Title, e.Message) }
```

```go
func (p detailPage) Props(r *http.Request, store *Store) (DetailProps, error) {
    item, err := store.Load(r.Context(), r.PathValue("itemId"))
    switch {
    case errors.Is(err, ErrNotFound):
        return DetailProps{}, ErrorWithStatus{Status: http.StatusNotFound, Title: "Not found", Message: "no such item"}
    case err != nil:
        return DetailProps{}, fmt.Errorf("detail: load: %w", err) // plain error → logged 500
    }
    return DetailProps{Item: item}, nil
}
```

`errors.As` unwraps, so `fmt.Errorf("...: %w", ErrorWithStatus{...})` still resolves to its status.

## Redirects: a control-flow signal, not `http.Redirect`

Don't call `http.Redirect` from a handler in an HTMX app — during an HTMX request the XHR follows the 3xx and swaps the redirect *target's* body into the partial's swap target. Return a signal instead; the error handler sends `HX-Location` for HTMX (ajax navigation, like a boosted link) and a 303 otherwise:

```go
// Redirect is control flow, not a real error — it implements error only to
// ride the error-return path, which unwinds the render flow without writing
// the ResponseWriter directly.
type Redirect struct{ To string }

func (Redirect) Error() string { return "redirect" }
```

```go
func (p submitForm) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error {
    id, err := store.Save(r.Context(), r.FormValue("name"))
    if err != nil {
        return err
    }
    url, err := structpages.URLFor(r.Context(), detailPage{}, map[string]any{"itemId": id})
    if err != nil {
        return err
    }
    return Redirect{To: url}
}
```

Use `HX-Redirect` instead of `HX-Location` only when the destination genuinely needs a full browser load — a non-htmx endpoint, or a page with different `<head>` content/scripts.

## The global handler

Wired once at `Mount`, it owns every error response — typed statuses, redirects, cancellations, and the logged-500 fallback:

```go
structpages.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
    if errors.Is(err, context.Canceled) || r.Context().Err() != nil {
        w.WriteHeader(499) // client closed request — expected, don't log as error
        return
    }
    var redir Redirect
    if errors.As(err, &redir) {
        if r.Header.Get("HX-Request") == "true" {
            // Ajax navigation, like a boosted link. The status must stay 2xx:
            // htmx does not process response headers on 3xx responses.
            w.Header().Set("HX-Location", redir.To)
            return
        }
        http.Redirect(w, r, redir.To, http.StatusSeeOther)
        return
    }
    status, title, message := http.StatusInternalServerError, "Server error", err.Error()
    var se ErrorWithStatus
    if errors.As(err, &se) {
        status, title, message = se.Status, se.Title, se.Message
    } else {
        slog.Error("unhandled error rendering page", "error", err, "path", r.URL.Path)
    }
    // One place that knows how to render: HTMX-aware retarget, full layout vs bare page.
    renderHTTPError(w, r, status, title, message)
})
```

## JSON endpoints: the no-error form

For endpoints that serve JSON, use the no-return `ServeHTTP(w, r, deps...)` signature. It is unbuffered, the HTML error handler is never invoked, and you own the response — including errors, which are JSON like everything else. Don't reach for `http.Error`; its `text/plain` body is the wrong shape for an API client:

```go
func (p trackTime) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) {
    var body trackTimeRequest
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        writeJSONError(w, http.StatusBadRequest, "invalid request")
        return
    }
    if err := store.UpdateTime(r.Context(), body); err != nil {
        writeJSONError(w, http.StatusInternalServerError, "update failed")
        return
    }
    w.WriteHeader(http.StatusOK)
}

// The API's single error shape, defined once:
func writeJSONError(w http.ResponseWriter, status int, msg string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

## Streaming (SSE)

Picking the no-return form is not enough to guarantee writes reach the client immediately — `w` may still be wrapped by middleware. Use `http.ResponseController`, which walks the `Unwrap()` chain to find a flusher:

```go
func (p progress) ServeHTTP(w http.ResponseWriter, r *http.Request, jobs *JobService) error {
    w.Header().Set("Content-Type", "text/event-stream")
    rc := http.NewResponseController(w)
    for update := range jobs.Progress(r.Context()) {
        fmt.Fprintf(w, "event: progress\ndata: %s\n\n", update)
        if err := rc.Flush(); err != nil {
            return nil // client gone
        }
    }
    return nil
}
```

This works from *either* `ServeHTTP` form — the buffered wrapper implements `FlushError()` and `Unwrap()`. Once you've started flushing, a non-nil error can no longer produce a clean error page (bytes are on the wire) — send an `event: error` SSE frame instead and `return nil`.

## Which form to use

| Handler does… | `ServeHTTP` signature | Errors via |
|---|---|---|
| Renders HTML / HTMX partial | `(w, r, deps...) error` | `return ErrorWithStatus{…}` / `return err` |
| Redirects | `(w, r, deps...) error` | `return Redirect{To: …}` |
| Serves JSON / API | `(w, r, deps...)` *(no return)* | write `w` directly with a JSON error body |
| Streams (SSE, progress) | either form + `http.NewResponseController` | SSE `event: error` frame, then `return nil` |

Props methods always follow the first row — they are buffered and their errors flow to `WithErrorHandler`.

## ErrSkipPageRender

If a Props method writes the response itself (rare — prefer the `Redirect` signal), return `structpages.ErrSkipPageRender` to skip rendering. Only the Props error path checks this sentinel; returning it from `ServeHTTP` does nothing special.
