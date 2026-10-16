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
    if target.Is(p.TodoList) {
        return getActiveTodos() // partial update — load only what it needs
    }
    return getAllTodos() // full page
}
```

```gsx
component (p index) Page(todos []Todo) {
	<Layout>
		<form hx-post={add{} |> url} hx-target={index.TodoList |> target}>
			<input name="text"/>
			<button>Add</button>
		</form>
		<div id={index.TodoList |> id}>
			<p.TodoList todos={todos}/>
		</div>
	</Layout>
}

component (p index) TodoList(todos []Todo) {
	{ for _, todo := range todos {
		<div>{ todo.Text }</div>
	} }
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

`p.UserList(users)` is a normal Go call — compile-time checked — handed to `RenderComponent` as the response. Two principles generalize from this:

- **Partials get partial data, never the full props struct.** When `target.Is(p.UserList)` matches, you build just that region's data; the `IndexProps{}` returned alongside `RenderComponent` is ignored.
- **The default case falls back to full props, not empty props** — browser navigation, boosted swaps, and unrecognised targets all need the whole page.

On larger pages the props struct is typically *composed of* per-pane sub-structs, with helper props methods feeding both the partial branches and a shared `fullProps` — see the worked example in [HTMX Integration](./htmx.md#rendertarget-in-props).

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

**Pass a constructed component.** `index{}.TodoList(todos)` is a normal Go call — the compiler checks argument types and counts — and page structs are stateless, so a zero-value receiver constructs a *sibling* page's component just as well as your own.

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

API endpoints use the **no-error** form, so writes go straight to the wire and the HTML error handler stays out of it. You own the response, errors included, which are JSON like everything else. The worked example is in [Error Handling](./error-handling.md#json-endpoints-the-no-error-form).

## ServeHTTP signatures

Four signatures are supported. The DI forms take typed params (matched by type, any order) — there is no variadic `deps ...any`:

```go
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request)                      // standard http.Handler, unbuffered
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request) error                // buffered; error → WithErrorHandler
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store)        // DI, no return, unbuffered
func (p T) ServeHTTP(w http.ResponseWriter, r *http.Request, store *Store) error  // DI, buffered
```

Only the error-returning forms are buffered; what that means for writing `w` is in [Error Handling](./error-handling.md#the-rules).

## See also

- [HTMX Integration](./htmx.md) — `RenderTarget`, partial selection, the id loop.
- [Error Handling](./error-handling.md) — buffering rules, typed errors, redirects, JSON.
- [examples/todo](https://github.com/jackielii/structpages/tree/main/examples/todo) — complete working example.
