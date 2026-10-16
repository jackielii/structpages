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

The Props method runs with a [`RenderTarget`](./htmx.md) injected, so it knows what the request wants and loads only that region's data. When Props returns no error, the framework renders the page component the target `Is` — here `TodoList`, filled with the returned todos — or `Page`. Initial loads render `Page`; with an htmx target selector (`WithTargetSelector(structpages.HTMXRenderTarget)`, or `HTMXv4RenderTarget` for htmx 4), HTMX requests targeting `index.TodoList`'s id render just the partial. Without a selector every request renders `Page`. The full flow is in [How a Request Is Rendered](./rendering.md).

### Complex props structs

Real pages often need a props struct with many fields while individual page components take only a subset. Return the subset as its own value: each page component's parameters are filled by type from Props' results, so `Page` takes the props struct and `UserList` takes just the users:

```go
type IndexProps struct {
    Users      []User
    Picklists  []Picklist
    Search     string
    TotalCount int
}

func (p index) Props(r *http.Request, target structpages.RenderTarget) (IndexProps, []User, error) {
    if target.Is(p.UserList) {
        users, err := searchUsers(r.URL.Query().Get("q"))
        return IndexProps{}, users, err // renders UserList(users)
    }
    // full page
    users, err := getAllUsers()
    if err != nil {
        return IndexProps{}, nil, err
    }
    picklists, err := getPicklists()
    if err != nil {
        return IndexProps{}, nil, err
    }
    return IndexProps{
        Users:      users,
        Picklists:  picklists,
        Search:     r.URL.Query().Get("q"),
        TotalCount: len(users),
    }, nil, nil // renders Page(props IndexProps)
}
```

Two principles generalize from this:

- **Partials get partial data, never the full props struct.** When `target.Is(p.UserList)` matches, you build just that region's data and return zero values for the rest. Each result fills at most one parameter, so give each share a distinct type.
- **The default case falls back to full props, not empty props** — browser navigation, boosted swaps, and unrecognised targets all need the whole page.

To render something other than what the request wants — an empty state, an inline error notice, several regions at once — return `RenderComponent(p.EmptyState())` (or several components) from Props; see [Breaking the flow on purpose](./rendering.md#breaking-the-flow-on-purpose).

On larger pages the data is typically split into per-pane values, with helper methods feeding both the partial branches and the full page — see the worked example in [HTMX Integration](./htmx.md#rendertarget-in-props).

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

**Pass a constructed component.** `index{}.TodoList(todos)` is a normal Go call — the compiler checks argument types and counts — and page structs are stateless, so a zero-value receiver constructs a *sibling* page's component just as well as your own. `RenderComponent` takes several components and renders them in order as one response, for updating several regions at once ([Updating several regions](./rendering.md#updating-several-regions)).

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

- [How a Request Is Rendered](./rendering.md) — the flow: target selector, Props, which component renders.
- [HTMX Integration](./htmx.md) — `RenderTarget`, partial selection, the id loop.
- [Error Handling](./error-handling.md) — buffering rules, typed errors, redirects, JSON.
- [examples/todo](https://github.com/jackielii/structpages/tree/main/examples/todo) — complete working example.
