---
title: Testing
slug: /testing
sidebar_position: 12
---

# Testing

Three layers cover a structpages app: render tests for components, request tests through the mounted mux, and URL checks that fail a broken route before it ships. The linter covers the static part of the last one in CI.

## Testing renders with a bare context

Unit tests that render components directly — without an HTTP server — need a page tree in the context so the `url` / `id` / `target` filters (`URLFor` / `ID` / `IDTarget`) resolve. Use `structpages.Parse` (builds the tree, no mux) and `sp.PageContext`:

```go
func TestProductPageRenders(t *testing.T) {
    sp, err := structpages.Parse(pages{}, "/", "App",
        structpages.WithArgs(fakeStore),
    )
    if err != nil {
        t.Fatal(err)
    }
    ctx := sp.PageContext(context.Background())

    var buf bytes.Buffer
    props := productPageProps{Product: sampleProduct}
    if err := (productPage{}).Page(props).Render(ctx, &buf); err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(buf.String(), sampleProduct.Name) {
        t.Errorf("rendered page missing product name")
    }
}
```

`Parse` takes the same options as `Mount` and builds the same tree, ids included (`WithMaxIDLength` applies too). Mux-shaped options such as middlewares are accepted but inert, since no handlers register. Parse the app's root even when the test exercises one module, so URLs to sibling modules resolve.

### No current page under PageContext

`PageContext` attaches the tree, not a matched request, so `structpages.CurrentPage(ctx)` is nil there and `structpages.Within(ctx, page)` returns `false, nil`. Active-navigation state therefore renders as "nothing active" in a bare-context test. To test it, go through a request instead (below): the current page is set while serving a `Props`/component page, though not a page that handles requests with its own `ServeHTTP`.

## Requests through Mount

To exercise routing, `Props`, DI and the target selector together, mount the tree on a fresh mux and serve requests with `httptest`. An HTMX partial is a request with `HX-Request: true` and the targeted element's id in `HX-Target` — take the id from `sp.ID`, the same value the `id` filter renders:

```go
func TestTodoListPartial(t *testing.T) {
    mux := http.NewServeMux()
    sp, err := structpages.Mount(mux, index{}, "/", "App",
        structpages.WithArgs(newFakeStore()),
    )
    if err != nil {
        t.Fatal(err)
    }
    id, err := sp.ID(index.TodoList)
    if err != nil {
        t.Fatal(err)
    }

    req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
    req.Header.Set("HX-Request", "true")
    req.Header.Set("HX-Target", id)
    rec := httptest.NewRecorder()
    mux.ServeHTTP(rec, req)

    if rec.Code != http.StatusOK {
        t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
    }
    if strings.Contains(rec.Body.String(), "<html") {
        t.Errorf("partial request rendered the full page")
    }
}
```

Leave the headers off to get the full page. With `HTMXv4RenderTarget` configured, htmx 4 sends `HX-Target` as `tag#id` (for example `div#` + `id`); see [HTMX Integration](./htmx.md).

## Every URL resolves

`URLFor` returns an error for a page that isn't mounted, a renamed `Ref` or a missing param, but only when that line runs. For URLs built from runtime data, resolve an inventory of every URL family the app generates once at startup and refuse to boot if one fails — see [URLFor & ID → Validation](./urlfor.md#validation-no-dangling-urls-in-production). Call the same function from a test so CI catches it first. [`examples/url-validation`](../examples/url-validation/) has the runnable version, with an integration test that renders pages and checks their `href`s.

## Lint in CI

`structpages-lint` resolves every static `URLFor` / `ID` / `IDTarget` / `Within` / `Ref` against the tree, including the filters in `.gsx` files, without running anything. Run it next to `go test`; in gsx projects generate first:

```bash
go tool gsx generate .
go tool structpages-lint ./...
```

It exits 1 on findings and 2 when packages fail to load. See [Lint](./lint.md).
