---
title: Dependency Injection
slug: /dependency-injection
sidebar_position: 11
---

# Dependency Injection

Register your services once with `WithArgs`; structpages passes them by type to the page methods that declare them. A page never reaches for a global and never threads a dependency through its parent. Values that differ per request, such as the signed-in user, come from [request args](#request-args) the same way.

```go
store := &Store{db: db}
sessions := NewSessionManager()

sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(store, sessions, logger),
)
if err != nil {
    log.Fatal(err)
}
```

```go
func (p userPage) Props(r *http.Request, store *Store, sessions *SessionManager) (UserProps, error) {
    user, err := sessions.User(r)
    if err != nil {
        return UserProps{}, err
    }
    return store.LoadUser(r.Context(), user.ID)
}
```

Parameter order doesn't matter, and a method declares only what it uses.

## What each method can receive

| Method | Called | Can receive |
|---|---|---|
| `Props` | per request | `*http.Request`, `http.ResponseWriter`, `RenderTarget`, `*PageNode`, request args, `WithArgs` values |
| `ServeHTTP` (with extra params) | per request | `http.ResponseWriter`, `*http.Request`, `RenderTarget`, `*PageNode`, request args, `WithArgs` values |
| `Page` and other components | per request, when selected | `Props`' results, then everything `Props` can receive |
| `Middlewares` | once, at `Mount` | `*PageNode`, `WithArgs` values |
| `RequestArgs` | once, at `Mount` | `*PageNode`, `WithArgs` values |
| `Init` | once, while the tree is parsed | `*PageNode`, `WithArgs` values |

`*structpages.PageNode` is the node of the page being served or set up; the framework always supplies it.

A parameter is filled from the first source that has its type: `Props`' results (components only, each used once), then the per-request values, then request args, then `WithArgs`. [How a Request Is Rendered](./rendering.md#where-parameter-values-come-from) shows where this fits in the request flow.

## Matching rules

Each `WithArgs` value is registered under its type, and a parameter takes the registration of its type.

- **One registration per type.** Registering two values of the same type fails `Mount`. To inject several values of one underlying type, give each its own named type:

  ```go
  type APIKey string
  type DatabaseName string

  sp, err := structpages.Mount(mux, pages{}, "/", "My App",
      structpages.WithArgs(APIKey("your-api-key"), DatabaseName("mydb")),
  )

  func (p userPage) Props(r *http.Request, key APIKey, db DatabaseName) (UserProps, error) {
      …
  }
  ```

- **Pointers fill values, not the reverse.** A registered `*AppContext` also fills a parameter of type `AppContext`. A registered `AppContext` value can't fill an `*AppContext` parameter, so register the pointer.
- **An interface parameter takes the one registration that implements it.** A method can declare just what it needs, so a unit test can call it with a fake:

  ```go
  type userLister interface {
      ListUsers(ctx context.Context) ([]User, error)
  }

  // Mount registers *PostgresStore, which implements userLister.
  func (p usersPage) Props(r *http.Request, users userLister) (UsersProps, error) { … }

  // In a test, call Props directly with a fake.
  props, err := usersPage{}.Props(req, fakeUsers{})
  ```

  If more than one registration implements the interface, that's an error naming them all. Declare the concrete type, narrow the interface, or pick one by registering a pointer to an interface variable, which fills that interface ahead of the implementations:

  ```go
  var users userLister = pgStore
  structpages.WithArgs(pgStore, cacheStore, &users)
  ```

  The per-request values in the table are always used first: a `Props(w io.Writer)` gets the response writer, whatever is registered.
- **Generic types work** — type parameters, slices and maps as dependencies, aliases, function types and complex constraints. Anywhere these docs say "type", read it as "any reflect-distinguishable type".

`Mount` checks every `Props`, extended `ServeHTTP` and component parameter for ambiguity up front, so an interface parameter more than one request arg or registration implements fails at startup with the page and the types named, not on the first request to that page. For a component, `Props`' result types count as provided, since they fill its parameters first. `Init`, `Middlewares` and `RequestArgs` run during `Mount`, so their ambiguity fails it too. `Parse` skips the check.

The check knows the writer only as `http.ResponseWriter`. A parameter of another interface the real writer happens to implement, such as `io.StringWriter`, is filled by the writer at request time, but `Mount` fails if two registrations also implement it. Declare `http.ResponseWriter` or `io.Writer` for the writer.

A parameter that matches nothing fails the request with `method X requires argument of type T, but not found`. That isn't checked at `Mount`, so a page can stay mounted in a configuration, or a test, that doesn't register its dependencies.

## Request args

`WithArgs` values are fixed at `Mount`. A request arg is a function that produces a value from the request, once per request:

```go
// currentUser reads the session the auth middleware attached to the request.
func currentUser(r *http.Request) (*User, error) {
    u, ok := auth.UserFrom(r.Context())
    if !ok {
        return nil, ErrNotSignedIn // goes to the error handler
    }
    return u, nil
}

sp, err := structpages.Mount(mux, pages{}, "/", "My App",
    structpages.WithArgs(store),
    structpages.WithRequestArgs(currentUser),
)
```

```go
func (p accountPage) Props(u *User, store *Store) (AccountProps, error) {
    return store.Account(u.ID)
}
```

`Props` declares the user instead of deriving it from the request, so a test calls it with a plain value: `accountPage{}.Props(&User{ID: 7}, store)`.

- **Two shapes:** `func(*http.Request) (T, error)`, or `func(*http.Request, *PageNode) (T, error)` when the value depends on the page. `Mount` fails for any other shape, naming the argument.
- **Lazy and once per request.** A provider runs the first time a parameter needs its type and is reused for the rest of the request, so `Props` and the component it renders share one value, and a database lookup runs once.
- **Errors count as the error of the method that needed the value.** For `Props` or a component, that means a provider can return `RenderComponent(...)` (say, a sign-in prompt), `ErrSkipPageRender` after redirecting, or any error for the [error handler](./error-handling.md) to render. For an error-returning `ServeHTTP`, `RenderComponent` is honoured and other errors go to the error handler.
- **Matched like `WithArgs`:** a provider of `*T` also fills a `T` parameter, and an interface parameter takes the one provider whose type implements it. A provider that returns a nil `*T` fails a `T` parameter; declare `*T` to accept nil.
- **Optional values:** return a nil pointer, and let the page check it, when absence is normal.
- **One provider per type** within a registration. Two fail `Mount`.

### Request args for part of the tree

A page's `RequestArgs` method provides values for that page and every page below it. Like `Middlewares`, it runs once at `Mount` and can take `WithArgs` values, so providers can capture services:

```go
type channelRoutes struct {
    messages messagesPage `route:"/{channelID}/messages Messages"`
    files    filesPage    `route:"/{channelID}/files Files"`
}

func (channelRoutes) RequestArgs(chats *chat.Service) []any {
    return []any{
        func(r *http.Request) (*chat.Channel, error) {
            return chats.Channel(r.Context(), r.PathValue("channelID"))
        },
    }
}

func (messagesPage) Props(ch *chat.Channel, chats *chat.Service) (messagesProps, error) { … }
```

The channel is loaded once per request, only for pages under `channelRoutes`, and only when a parameter asks for it. That replaces group middleware that stores a value in the context for pages to read back.

The nearest provider wins: a page's own `RequestArgs` come first, then each ancestor's, then `WithRequestArgs`. So a section can shadow a mount-wide provider of the same type.

**When to use which:** a value that's the same for every request is a `WithArgs` value. A value derived from the request is a request arg. Middleware is still the place for work that must run before every handler, such as authentication or the `context` values templates read.

## Init

`Init` runs once per page while the tree is parsed, by `Mount` and by `Parse`. It can return an `error`, which aborts the mount, and can receive `WithArgs` values:

```go
type databasePage struct {
    db *sql.DB
}

func (d *databasePage) Init(store *Store) error {
    d.db = store.DB()
    return nil
}
```

Either receiver works; use a pointer when `Init` sets fields on the page, the typical case. Prefer `WithArgs` parameters for runtime dependencies: `Init` is for setup that must happen exactly once and isn't naturally a method parameter. Pages are usually stateless, and a page whose state comes from `Init` holds it only on its mounted instance, not on a zero value such as `myPage{}`.
