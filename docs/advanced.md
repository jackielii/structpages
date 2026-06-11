---
title: Advanced Features
slug: /advanced
sidebar_position: 11
---

# Advanced Features

## Initialization

Use the `Init` method for one-time setup at `Mount` time. `Init` may return an `error` and may receive injected dependencies (matched by type from `WithArgs`):

```go
type databasePage struct {
    db *sql.DB
}

func (d *databasePage) Init(store *Store) error {
    // Called once during Mount. Errors abort Mount.
    d.db = store.DB()
    return nil
}
```

Either value or pointer receiver works; use pointer if `Init` mutates the page (the typical case). Prefer `WithArgs` for runtime dependencies — `Init` is for setup that has to happen exactly once and isn't naturally a method parameter.

## Dependency injection

Register services once at `Mount`; they're matched by type into method parameters:

```go
store := &Store{db: db}
sessionManager := NewSessionManager()

sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(store, sessionManager, logger),
)
if err != nil {
    log.Fatal(err)
}
```

**Each type can only be registered once.** To inject multiple values of the same underlying type, create distinct named types:

```go
// DON'T — duplicate type string errors at Mount
_, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs("api-key", "db-name"),
)

// DO — distinct types
type APIKey string
type DatabaseName string

sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(APIKey("your-api-key"), DatabaseName("mydb")),
)

func (p userPage) Props(r *http.Request, apiKey APIKey, dbName DatabaseName) (UserProps, error) {
    // Both available with type safety
}
```

**Type matching with coercion.** The argument registry coerces between pointer and value forms and falls back to assignability. One concrete consequence: a single `*AppContext` registration can fill a parameter typed as any interface that `*AppContext` implements — register concrete types, declare interface parameters where it helps testability.

**Generic types and interface types both work** — type parameters, slices/maps as deps, aliases, function types, complex constraints, pointer semantics, and interface injection are all covered by the library's test matrix. Anywhere these docs say "type", read it as "any reflect-distinguishable type".

`*structpages.PageNode` is always available for injection — the framework adds the current node automatically.

Services are injected into any page method that declares them: `Props`, `ServeHTTP`, `Middlewares`, and `Init`.

## Dynamic references with Ref

`Ref` (a string type) references pages and page components by field name when static types aren't available — configuration-driven menus, generic components, cross-package call sites:

```go
// Reference by page field name
url, err := structpages.URLFor(ctx, structpages.Ref("homePage"))

// Qualified path, with params
url, err := structpages.URLFor(ctx, structpages.Ref("Admin.ProductPage"),
    map[string]any{"productId": "123"})

// Compose with URL fragments
url, err := structpages.URLFor(ctx, []any{structpages.Ref("userPage"), "?tab=profile"})
```

For page components, `Ref("PageName.MethodName")` works with `ID`/`IDTarget`:

```go
id, err := structpages.ID(ctx, structpages.Ref("userPage.UserList"))
// → "user-page-user-list"

target, err := structpages.IDTarget(ctx, structpages.Ref("userPage.UserList"))
// → "#user-page-user-list"
```

An unqualified method name (`Ref("UserList")`) resolves only if exactly one page has that method; ambiguity errors with the candidates listed.

### Example: configuration-driven menu

```go
type MenuItem struct {
    PageRef structpages.Ref
    Label   string
}

var menu = []MenuItem{
    {structpages.Ref("home"), "Home"},
    {structpages.Ref("users"), "Users"},
    {structpages.Ref("settings"), "Settings"},
}
```

```templ
templ Navigation() {
    <nav>
        for _, item := range menu {
            <a href={ structpages.URLFor(ctx, item.PageRef) }>{ item.Label }</a>
        }
    </nav>
}
```

Templ attributes accept `(string, error)`, so no error juggling is needed at the call site. `structpages-lint` validates Ref strings — including ones stored in struct fields like this menu — so a renamed page fails CI, and a boot-time validator catches it in production deploys:

```go
func TestMenuReferences(t *testing.T) {
    mux := http.NewServeMux()
    sp, err := structpages.Mount(mux, &pages{}, "/", "App")
    if err != nil {
        t.Fatal(err)
    }
    for _, item := range menu {
        if _, err := sp.URLFor(item.PageRef); err != nil {
            t.Errorf("invalid menu item %s: %v", item.Label, err)
        }
    }
}
```

Outside a request context (tests, init), use the methods on `*StructPages`; within handlers, use the context-based functions — the framework auto-injects the parse context via internal middleware.

## Type aliases and URLFor

Go type aliases (`type X = Y`) are identical at runtime — `reflect.TypeOf` cannot distinguish them. When the same type (or its alias) is mounted on multiple routes, a bare type lookup is ambiguous:

```go
type productPage struct{}
type featuredProduct = productPage // alias — same reflect.Type

type pages struct {
    products productPage     `route:"/products Products"`
    featured featuredProduct `route:"/featured Featured"`
}

// Errors: productPage matches two mounted nodes. Strict URLFor never
// silently picks one — the error lists both and the disambiguation forms.
url, err := structpages.URLFor(ctx, featuredProduct{})
```

Disambiguate with `Ref` by field name:

```go
url, err := structpages.URLFor(ctx, structpages.Ref("products")) // → "/products"
url, err = structpages.URLFor(ctx, structpages.Ref("featured"))  // → "/featured"

id, err := structpages.IDTarget(ctx, structpages.Ref("featured.Page")) // → "#featured-page"
```

(The `[]any{Parent{}, Leaf{}}` chain form doesn't help here — both mounts share one type — so `Ref` is the tool.) If each route uses a unique type, type-based matching needs no disambiguation at all. Type aliases are fine for routing and rendering; only type-based lookup is affected.

## Custom target selectors

See [HTMX Integration](./htmx.md#custom-target-selectors) for `WithTargetSelector`, the htmx 4 selector, and content-negotiation patterns.
