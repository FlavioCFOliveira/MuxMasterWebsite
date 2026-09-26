---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# Getting Started with MuxMaster

This guide walks you through building a small but realistic REST API with MuxMaster from scratch. By the end you will have a working server with route groups, middleware, path parameters, JSON responses, and centralized error handling.

## Prerequisites

- Go 1.27.1 or later ([download](https://go.dev/dl/))
- Familiarity with `net/http` and Go modules

## Install

Create a new module and add MuxMaster as a dependency:

```
mkdir myapi && cd myapi
go mod init myapi
go get github.com/FlavioCFOliveira/MuxMaster
```

## Step 1 — Hello, World

Create `main.go`:

```go
package main

import (
    "fmt"
    "log"
    "net/http"

    "github.com/FlavioCFOliveira/MuxMaster"
)

func main() {
    mux := muxmaster.New()

    mux.GET("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello, World!")
    })

    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

Run it and test it:

```
go run .
curl http://localhost:8080/
# Hello, World!
```

## Step 2 — Path Parameters

Path parameters are named segments in the URL pattern prefixed with `:`. Use `muxmaster.PathParam` to read a single value, or `muxmaster.ParamsFromContext` to read all of them.

```go
mux.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id := muxmaster.PathParam(r, "id")
    fmt.Fprintf(w, "user: %s\n", id)
})
```

```
curl http://localhost:8080/users/42
# user: 42
```

To parse the value as an integer:

```go
mux.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
    ps := muxmaster.ParamsFromContext(r.Context())
    id, err := ps.Int("id")
    if err != nil {
        http.Error(w, "invalid id", http.StatusBadRequest)
        return
    }
    fmt.Fprintf(w, "user id: %d\n", id)
})
```

## Step 3 — Add Middleware

Middleware registered with `Use` wraps every route registered **after** the `Use` call; routes registered earlier are not wrapped. The most common setup adds access logging and panic recovery at the top of `main`:

```go
import (
    "log/slog"
    "os"

    "github.com/FlavioCFOliveira/MuxMaster/middleware"
)

mux := muxmaster.New()
mux.Use(middleware.Logger(os.Stdout))
mux.Use(middleware.RecovererWithLogger(slog.Default()))
```

After restarting, every request prints one line in the format `<RFC 3339 time> <method> <path> <status> <duration>`:

```
2026-04-17T10:05:31Z GET /users/42 200 87.5µs
```

## Step 4 — Route Groups

Use groups to share a path prefix and middleware across a set of related routes:

```go
api := mux.Group("/api/v1")
api.Use(requireAPIKey) // only applies to routes in this group

api.GET("/users", listUsers)
api.POST("/users", createUser)
api.GET("/users/:id", getUser)
api.PUT("/users/:id", updateUser)
api.DELETE("/users/:id", deleteUser)
```

You can nest groups to add another layer of middleware:

```go
admin := api.Group("/admin")
admin.Use(requireAdmin)
admin.GET("/stats", getStats)
```

## Step 5 — JSON Responses

`muxmaster.JSON` marshals any value to JSON, sets `Content-Type: application/json; charset=utf-8`, and writes the status code and body in one call:

```go
type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

mux.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
    id, _ := muxmaster.ParamsFromContext(r.Context()).Int("id")
    user := User{ID: id, Name: "Alice"}
    muxmaster.JSON(w, http.StatusOK, user)
})
```

## Step 6 — Error-Returning Handlers

Repeat `if err != nil { http.Error(...); return }` quickly becomes noisy. `HandlerFuncE` lets handlers return an error instead:

```go
mux.GETE("/users/:id", func(w http.ResponseWriter, r *http.Request) error {
    id, err := muxmaster.ParamsFromContext(r.Context()).Int("id")
    if err != nil {
        return muxmaster.Error(http.StatusBadRequest, err)
    }
    user, err := db.FindUser(id)
    if err != nil {
        return muxmaster.Error(http.StatusNotFound, errors.New("user not found"))
    }
    return muxmaster.JSON(w, http.StatusOK, user)
})
```

Without an `ErrorHandler`, a returned error always produces a plain-text `500 Internal Server Error`, whatever status code the error carries. Set a custom error handler to use the status code of an `HTTPError` and produce consistent JSON error responses:

```go
mux.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
    code := http.StatusInternalServerError
    var he muxmaster.HTTPError
    if errors.As(err, &he) {
        code = he.StatusCode()
    }
    muxmaster.JSON(w, code, map[string]string{"error": err.Error()})
}
```

## Step 7 — Complete Example

Here is the full application combining everything from the steps above:

```go
package main

import (
    "errors"
    "log"
    "log/slog"
    "net/http"
    "os"

    "github.com/FlavioCFOliveira/MuxMaster"
    "github.com/FlavioCFOliveira/MuxMaster/middleware"
)

type User struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

var users = map[int]User{
    1: {ID: 1, Name: "Alice"},
    2: {ID: 2, Name: "Bob"},
}

func main() {
    mux := muxmaster.New()
    mux.Use(middleware.Logger(os.Stdout))
    mux.Use(middleware.RecovererWithLogger(slog.Default()))

    mux.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
        code := http.StatusInternalServerError
        var he muxmaster.HTTPError
        if errors.As(err, &he) {
            code = he.StatusCode()
        }
        muxmaster.JSON(w, code, map[string]string{"error": err.Error()})
    }

    api := mux.Group("/api/v1")

    api.GET("/users", func(w http.ResponseWriter, r *http.Request) {
        list := make([]User, 0, len(users))
        for _, u := range users {
            list = append(list, u)
        }
        muxmaster.JSON(w, http.StatusOK, list)
    })

    api.GETE("/users/:id", func(w http.ResponseWriter, r *http.Request) error {
        id, err := muxmaster.ParamsFromContext(r.Context()).Int("id")
        if err != nil {
            return muxmaster.Error(http.StatusBadRequest, err)
        }
        u, ok := users[id]
        if !ok {
            return muxmaster.Error(http.StatusNotFound, errors.New("user not found"))
        }
        return muxmaster.JSON(w, http.StatusOK, u)
    })

    log.Fatal(http.ListenAndServe(":8080", mux))
}
```

```
curl http://localhost:8080/api/v1/users
curl http://localhost:8080/api/v1/users/1
curl http://localhost:8080/api/v1/users/99
# {"error":"user not found"}
```

## Next Steps

- [Routing](/docs/routing) — complete pattern syntax and method reference
- [HTTP QUERY method (RFC 10008)](/docs/http-query-method) — routes for the safe, idempotent QUERY method that carries a request body
- [Middleware](/docs/middleware) — built-in middleware and writing your own
- [Groups](/docs/groups) — advanced grouping and sub-router mounting
- [Error Handling](/docs/error-handling) — centralized error patterns
- [Cookbook](/docs/cookbook) — common production patterns

## Common questions

<section data-conversation="getting-started-basics">

### How do I install MuxMaster in a new project?

Run `go get github.com/FlavioCFOliveira/MuxMaster` inside a Go module; MuxMaster v1.3.0 requires Go 1.27.1 or later.

The command adds MuxMaster to `go.mod` and `go.sum`. No other dependency is added, because the module imports only the standard library. With the default `GOTOOLCHAIN=auto`, an older local toolchain switches to Go 1.27.1 or newer automatically.

### What is the smallest working MuxMaster server?

The smallest working server is the program in Step 1: create the router with `muxmaster.New()`, register one route with `mux.GET`, and pass the router to `http.ListenAndServe`.

`*muxmaster.Mux` implements `http.Handler`, so every part of `net/http` that accepts a handler, such as `http.Server` or `httptest.NewServer`, accepts the router.

### How do I read a path parameter as an integer?

Call `muxmaster.ParamsFromContext(r.Context()).Int("id")`, which returns the parameter parsed as a base-10 `int` and an error.

`muxmaster.PathParam(r, "id")` returns the raw string. `Params` also provides `Int64`, `Uint64`, `Float64`, and `Bool`.

</section>

## Upstream source

This page mirrors [`docs/getting-started.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/docs/getting-started.md) at the v1.3.0 tag. The behaviour it describes is implemented in [`mux.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/mux.go), [`params.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/params.go), [`response.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/response.go).
