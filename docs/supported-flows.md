---
title: Page Response Patterns
slug: /supported-flows
sidebar_position: 5
---

# Page Response Patterns

There are four main shapes — choose based on what the page does. The first renders declaratively (Props method + Page method); the other three are handler methods (`ServeHTTP`).

| Shape | Entry | When to use |
|---|---|---|
| **Renders a page** | `Props(...)` + `Page(props)` | Standard pages, HTMX partials — the primary pattern |
| **Returns a partial** | `ServeHTTP(w, r, deps...) error` | Form actions that mutate then refresh a region |
| **Redirects** | `ServeHTTP(w, r, deps...) error` | Post-action navigation |
| **Serves JSON** | `ServeHTTP(w, r, deps...)` *(no error return)* | API endpoints |

## A page that renders: Props method + Page method

```go
type index struct {
    add `route:"POST /add Add"`
}

func (p index) Props(r *http.Request, target structpages.RenderTarget) ([]Todo, error) {
    switch {
    case target.Is(p.TodoList):
        return getActiveTodos(), nil // partial update — load only what it needs
    default:
        return getAllTodos(), nil // full page
    }
}

templ (p index) Page(todos []Todo) {
    @html() {
        <form hx-post={ structpages.URLFor(ctx, add{}) }
              hx-target={ structpages.IDTarget(ctx, index.TodoList) }>
            <input name="text" />
            <button>Add</button>
        </form>
        <div id={ structpages.ID(ctx, index.TodoList) }>
            @p.TodoList(todos)
        </div>
    }
}

templ (p index) TodoList(todos []Todo) {
    for _, todo := range todos {
        <div>{ todo.Text }</div>
    }
}
```

The Props method runs with a [`RenderTarget`](./htmx.md) injected, so it knows which page component will render and loads only that region's data. Initial loads render `Page`; HTMX requests targeting `index.TodoList`'s id render just the partial.

### Complex props structs

Real pages often need a props struct with many fields while individual page components take only a subset:

```go
type IndexProps struct {
    Users      []User
    Picklists  []Picklist
    Search     string
    TotalCount int
}

func (p index) Props(r *http.Request, target structpages.RenderTarget) (IndexProps, error) {
    switch {
    case target.Is(p.UserList):
        users, err := searchUsers(r.URL.Query().Get("q"))
        if err != nil {
            return IndexProps{}, err
        }
        return IndexProps{}, structpages.RenderComponent(p.UserList(users))

    default: // full page
        users, err := getAllUsers()
        if err != nil {
            return IndexProps{}, err
        }
        picklists, err := getPicklists()
        if err != nil {
            return IndexProps{}, err
        }
        return IndexProps{
            Users:      users,
            Picklists:  picklists,
            Search:     r.URL.Query().Get("q"),
            TotalCount: len(users),
        }, nil
    }
}
```

`p.UserList(users)` is a normal Go call — compile-time checked — handed to `RenderComponent` as the response. Partial page components take ONLY their specific data (`UserList([]User)`), never the full props struct.

## A handler method that returns a partial

The most common HTMX form action — mutate state, respond with the refreshed region:

```go
type add struct{}

func (a add) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
    text := r.FormValue("text")
    if text != "" {
        if err := addTodo(text); err != nil {
            return err
        }
    }
    todos, err := getActiveTodos()
    if err != nil {
        return err
    }
    return structpages.RenderComponent(index{}.TodoList(todos))
}
```

**Pass a constructed component.** Page structs are stateless, so a zero-value receiver (`index{}`) constructs a *sibling* page's component just as well as your own. The reflective method-expression form (`RenderComponent(index.TodoList)`) is reserved for components whose parameters the framework should DI-inject — see [HTMX Integration](./htmx.md).

## A handler method that redirects

Don't call `http.Redirect` directly in an HTMX app — during an HTMX request the XHR follows the 3xx and swaps the redirect *target's* body into the partial's swap target. Return a control-flow signal instead and let the global error handler send the right mechanism per request kind (`HX-Location` for HTMX, 303 otherwise):

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

The `Redirect` type and the error-handler wiring are covered in [Error Handling](./error-handling.md).

## A handler method that serves JSON

API endpoints use the **no-error** form so writes go straight to the wire (unbuffered) and the framework's HTML error handler stays out of it. You own the response — including errors, which are JSON like everything else:

```go
type trackTime struct{}

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
```

See [Error Handling](./error-handling.md) for `writeJSONError` and why `http.Error` is the wrong tool here.

## ServeHTTP signatures

Four signatures are supported. The DI forms take typed params (matched by type, any order) — there is no variadic `deps ...any`:

```go
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request)                      // standard http.Handler, unbuffered
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request) error                // buffered; error → WithErrorHandler
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store)        // DI, no return, unbuffered
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error  // DI, buffered
```

The error-returning forms run against a *buffered* writer so the error handler can discard partial output and render a clean error page. That has consequences — never write `w` then return an error. The full rules are in [Error Handling](./error-handling.md).

## See also

- [HTMX Integration](./htmx.md) — `RenderTarget`, partial selection, the id loop.
- [Error Handling](./error-handling.md) — buffering rules, typed errors, redirects, JSON.
- [examples/todo](https://github.com/jackielii/structpages/tree/main/examples/todo) — complete working example.
