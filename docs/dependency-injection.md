---
title: Dependency Injection
slug: /dependency-injection
sidebar_position: 11
---

# Dependency Injection

Register your services once with `WithArgs`; structpages passes them by type to the page methods that declare them. A page never reaches for a global and never threads a dependency through its parent.

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
| `Props` | per request | `*http.Request`, `http.ResponseWriter`, `RenderTarget`, `*PageNode`, `WithArgs` values |
| `ServeHTTP` (with extra params) | per request | `http.ResponseWriter`, `*http.Request`, `RenderTarget`, `*PageNode`, `WithArgs` values |
| `Middlewares` | once, at `Mount` | `*PageNode`, `WithArgs` values |
| `Init` | once, while the tree is parsed | `*PageNode`, `WithArgs` values |

`*structpages.PageNode` is the node of the page being served or set up; the framework always supplies it.

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

  If two registrations implement the interface, `Mount` fails and names both: declare the concrete type, or narrow the interface. The per-request values in the table are always used first: a `Props(w io.Writer)` gets the response writer, whatever is registered.
- **Generic types work** — type parameters, slices and maps as dependencies, aliases, function types and complex constraints. Anywhere these docs say "type", read it as "any reflect-distinguishable type".

`Mount` checks every `Props` and `ServeHTTP` parameter for ambiguity up front, so two registrations implementing one interface parameter fail at startup with the page and both types named, not on the first request to that page. `Parse` skips the check.

A parameter that matches nothing fails the request with `method X requires argument of type T, but not found`. That isn't checked at `Mount`, so a page can stay mounted in a configuration, or a test, that doesn't register its dependencies.

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
