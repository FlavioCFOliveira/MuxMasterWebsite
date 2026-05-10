# REST API example

A bookstore REST API that exercises every MuxMaster feature in one program: every HTTP method, path / regex / catch-all parameters, group composition, scoped middleware, the error-returning handler family, FastHandler + FastMiddleware, route introspection (`Routes`, `Lookup`, `Walk`), `Mount` for sub-handlers, and the production-grade middleware stack (`RealIP`, `RequestID`, `Logger`, `Recoverer`, `Timeout`, throttles, gzip, CORS, BasicAuth, NoCache, security headers). Reach for this example as the canonical "how do I structure a CRUD service?" reference.

## Step 1 — Construct the router and pin the dispatch flags

The seven flags on `*Mux` are set explicitly even when the value matches the default — for documentation purposes, the pinned flag set is the contract. Notable choices: `RedirectFixedPath = false` keeps middleware-based authentication intact (a fixed-path redirect would re-enter the dispatcher and bypass the `Use` chain), and `UseRawPath`/`UnescapePathValues` both stay `false` to let `net/http` canonicalise the URL before matching.

```go
r := mm.New()

r.RedirectTrailingSlash = true  // /books/ → /books
r.RedirectFixedPath = false     // security default — keeps middleware auth intact
r.HandleMethodNotAllowed = true // 405 with Allow header
r.HandleOPTIONS = true          // automatic OPTIONS + Allow header
r.CaseInsensitive = false       // strict case matching
r.UseRawPath = false            // use decoded path for matching
r.UnescapePathValues = false    // raw param values; opt-in if you need %2F decoded
```

`UseRawPath = true` + `UnescapePathValues = true` is the configuration `SECURITY.md` CDX-S8-002 forbids in combination — `ServeFiles` refuses to register on a router with both flags set.

## Step 2 — Wire JSON-shaped error / fallback handlers

Five override slots are populated: `PanicHandler` (last-resort panic recovery), `ErrorHandler` (central handler for `HandlerFuncE` errors), `NotFound` (JSON 404 with the offending method/path echoed back), `MethodNotAllowed` (JSON 405; the `Allow` header is set by the dispatcher), and `GlobalOPTIONS` (permissive CORS preflight for paths without their own CORS middleware).

```go
r.PanicHandler = func(w http.ResponseWriter, req *http.Request, rcv any) {
    log.Error("panic recovered", "path", req.URL.Path, "panic", rcv)
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusInternalServerError)
    _ = json.NewEncoder(w).Encode(ErrorResponse{Error: "internal server error", Code: 500})
}

r.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
    traceID := mw.GetRequestID(req.Context())
    code := http.StatusInternalServerError
    var he mm.HTTPError
    if errors.As(err, &he) {
        code = he.StatusCode()
    }
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(code)
    _ = json.NewEncoder(w).Encode(ErrorResponse{
        Error:   err.Error(),
        Code:    code,
        TraceID: traceID,
    })
}
```

The trace id propagates from `RequestID` (Step 4) into the error response, so a client error and the corresponding server log share a correlation key.

## Step 3 — Normalise paths in `Pre`

`CleanPath` belongs in `Pre` so the radix tree sees the cleaned URL. Normalising `/api//v1/books/../books` to `/api/v1/books` before route matching closes both a double-hit cache vector and a path-traversal vector.

```go
r.Pre(mw.CleanPath())
```

`StripSlashes` would be an alternative, but `CleanPath` is safer for REST APIs because it preserves the slash where it carries semantic meaning (a slash inside a path parameter, for example).

## Step 4 — Apply the global middleware stack

Seven layers of cross-cutting concerns: `RealIP` (rewrite `RemoteAddr` from `X-Forwarded-For` when the peer is a trusted proxy), `RequestID`, `Logger`, `Recoverer`, `ThrottleBacklog`, `WithValue` (inject the application version into every request context), and `Compress`.

```go
r.Use(
    mw.RealIP(&localhost, &private10, &private172, &private192),
    mw.RequestID(),
    mw.Logger(os.Stdout),
    mw.RecovererWithLogger(log),
    mw.ThrottleBacklog(200, 100, 5*time.Second),
    mw.WithValue(appVersionKey{}, "v1.0.0"),
    mw.Compress(gzip.BestSpeed),
)
```

`ThrottleBacklog(200, 100, 5s)` caps in-flight requests at 200 with a 100-request queue and a 5-second queue timeout — the queue absorbs short bursts without rejecting traffic, the timeout bounds how long a client waits on the queue.

## Step 5 — Register the public convenience routes

`/health` uses `GETFast` for zero-allocation dispatch (load balancers probe it thousands of times per second). `/version` reads the value `WithValue` injected in Step 4 — the canonical Go pattern for application-wide configuration that handlers need to read.

```go
r.GETFast("/health", func(w http.ResponseWriter, req *http.Request, _ mm.Params) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"status":"ok"}`)
})

r.GET("/version", func(w http.ResponseWriter, req *http.Request) {
    ver, _ := req.Context().Value(appVersionKey{}).(string)
    _ = mm.JSON(w, http.StatusOK, map[string]string{"version": ver})
})

r.GET("/books", func(w http.ResponseWriter, req *http.Request) {
    mm.Redirect(w, req, http.StatusMovedPermanently, "/api/v1/books")
})
```

The `/books` 301 demonstrates a canonical-URL redirect: the version-less path moves permanently to the versioned canonical so search engines and intermediaries transfer their accumulated signal to the right URL.

## Step 6 — Expose route introspection at `/debug/routes` and `/debug/lookup`

`r.Routes()` enumerates every registered route; `r.Lookup(method, path)` resolves a method/path against the dispatcher and returns the matched handler, parameters, and a found flag — invaluable for debugging dispatch decisions without touching the live traffic path.

```go
r.GET("/debug/routes", func(w http.ResponseWriter, req *http.Request) {
    _ = mm.JSON(w, http.StatusOK, r.Routes())
})

r.GET("/debug/lookup", func(w http.ResponseWriter, req *http.Request) {
    method := req.URL.Query().Get("method")
    path := req.URL.Query().Get("path")
    if method == "" || path == "" {
        _ = mm.Text(w, http.StatusBadRequest, "provide ?method=GET&path=/api/v1/books/1")
        return
    }
    handler, params, found := r.Lookup(method, path)
    type result struct {
        Found   bool      `json:"found"`
        Handler string    `json:"handler,omitempty"`
        Params  mm.Params `json:"params,omitempty"`
    }
    name := ""
    if handler != nil {
        name = fmt.Sprintf("%T", handler)
    }
    _ = mm.JSON(w, http.StatusOK, result{Found: found, Handler: name, Params: params})
})
```

Production deployments gate these endpoints behind authentication — exposing the full route table to an unauthenticated reader leaks API shape to attackers.

## Step 7 — Compose the `/api/v1` group with scoped middleware

`Group(prefix)` returns a sub-router that inherits the parent's stack and adds its own. The `/api/v1` group adds two scoped layers on top of the global stack: `Timeout(30s)` (per-request deadline) and `ThrottlePerIP(50, 10s)` (50 concurrent in-flight requests per source IP, 10-second queue timeout).

```go
v1 := r.Group("/api/v1")
v1.Use(
    mw.Timeout(30*time.Second),
    mw.ThrottlePerIP(50, 10*time.Second, nil),
)
```

`ThrottlePerIP` is the per-client equivalent of `ThrottleBacklog`: instead of capping in-flight requests across the process, it caps them per source IP, so one misbehaving client cannot starve the rest of the user base.

## Step 8 — Register the books CRUD surface

The seven canonical CRUD routes are registered on a `books := v1.Group("/books")` sub-group. The example deliberately mixes `GET`/`POST`/etc. (`http.HandlerFunc`-shaped) with their `E`-suffixed counterparts (`GETE`/`POSTE`/`DELETEE`/etc., which return an error so the central `ErrorHandler` writes the response).

```go
books := v1.Group("/books")

books.GET("", store.listBooks)              // list
books.POSTE("", store.createBook)           // create — error-returning
books.HEAD("/:id", store.headBook)          // existence probe
books.GETE("/:id", store.getBook)           // get by id
books.GET("/{id:[0-9]+}/details", store.getBookDetails) // regex param
books.PUTE("/:id", store.replaceBook)       // full replacement
books.PATCH("/:id", store.patchBook)        // partial update
books.DELETEE("/:id", store.deleteBook)     // delete
```

The regex parameter `{id:[0-9]+}` constrains the segment to numeric ids without any handler-level validation; non-numeric ids fall through to the not-found handler.

## Step 9 — Compose nested resources and multi-method registrations

Two-and-three-level path parameters (`/:id/reviews/:rid`) and `Match`/`ANY` for routes that share logic across methods.

```go
books.GET("/:id/reviews", store.listReviews)
books.POSTE("/:id/reviews", store.createReview)
books.GETE("/:id/reviews/:rid", store.getReview)

books.Match(
    []string{http.MethodGet, http.MethodHead},
    "/featured",
    http.HandlerFunc(store.featuredBooks),
)

books.ANY("/ping", func(w http.ResponseWriter, req *http.Request) {
    _ = mm.Text(w, http.StatusOK, fmt.Sprintf("pong from %s /api/v1/books/ping", req.Method))
})

books.ServeFiles("/files/*filepath", http.Dir("./static/books"))
```

`books.ServeFiles("/files/*filepath", ...)` registers a catch-all under the books group — the dispatched path is everything after `/api/v1/books/files/` (the catch-all parameter `filepath`). HEAD is registered automatically.

## Step 10 — Scope CORS to the `/authors` subset via `With`

`With(...)` returns a group whose middleware applies only to the routes registered on the returned group — useful when CORS policy is per-resource rather than global. The example narrows allowed origins to two known callers and explicit methods/headers; `MaxAge` of 3600 caches the preflight at the browser for an hour.

```go
corsOpts := mw.CORSOptions{
    AllowedOrigins: []string{"https://bookshelf.example.com", "http://localhost:3000"},
    AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
    AllowedHeaders: []string{"Content-Type", "Authorization"},
    MaxAge:         3600,
}
authors := v1.With(mw.CORS(corsOpts))
authors.GET("/authors/:id/xml", store.getAuthorXML)
```

When a path is wrapped with explicit `mw.CORS`, that middleware short-circuits OPTIONS preflight; `r.GlobalOPTIONS` from Step 2 only fires on paths without their own CORS handler.

## Step 11 — Inline-define a sub-group via `Route`, then a BasicAuth admin group

`Route(prefix, fn)` is the inline-callback variant of `Group`: the closure receives a `*Group` and registers everything that belongs under the prefix. Useful when the sub-group's lifetime is the registration site itself.

```go
v1.Route("/categories", func(g *mm.Group) {
    g.Use(mw.NoCache())
    g.GET("", listCategories)
    g.GET("/:slug", getCategoryBySlug)
})

admin := v1.Group("/admin")
admin.Use(
    mw.BasicAuth("Bookstore Admin", map[string]string{"admin": "s3cr3t!"}),
    mw.NoCache(),
    mw.SetHeader("X-Admin-Zone", "true"),
)
admin.GET("/dashboard", adminDashboard)
admin.DELETEE("/books/:id", store.adminDeleteBook)
```

The BasicAuth credentials are illustrative — production code stores hashed passwords in a database and rebuilds the middleware on rotation.

## Step 12 — Register fast routes and a `FastMiddleware`

`UseFast` registers middleware that wraps `FastHandler` routes only; `GETFast` and `HandleFast` register routes against that fast-path. `FastHandler` skips the per-request context-allocation of `http.HandlerFunc` and is appropriate for hot-path endpoints like a metrics scrape.

```go
r.UseFast(fastTimer)

r.GETFast("/metrics", metricsHandler)

r.HandleFast(http.MethodPost, "/api/v1/fast/echo", fastEcho)
```

`fastTimer` is a `FastMiddleware` (`func(FastHandler) FastHandler`) — same composition rules as standard middleware, different signature.

## Step 13 — `Mount` a legacy sub-handler at `/legacy`

`Mount(prefix, handler)` strips the prefix before forwarding to the sub-handler. Useful for migrating old APIs incrementally: the new router lives at `/`, the legacy implementation continues to handle `/legacy/*` URLs as if it were rooted at `/`.

```go
legacyMux := buildLegacyMux()
r.Mount("/legacy", legacyMux)
```

The strip-prefix behaviour lets the legacy code carry its own URL space unchanged — only the new router knows about the prefix.

## Step 14 — Walk the dispatch tree at startup for a sanity-check log line

`Walk` enumerates standard routes, `WalkFast` enumerates fast routes. The example logs counts so a startup line confirms the expected number of routes were registered — a quick smoke test against accidental drop or duplicate registration.

```go
var stdCount, fastCount int
_ = r.Walk(func(method, pattern string, _ http.Handler) error {
    stdCount++
    return nil
})
_ = r.WalkFast(func(method, pattern string, _ mm.FastHandler) error {
    fastCount++
    return nil
})
log.Info("routes registered", "std", stdCount, "fast", fastCount)
```

In production the same callback can populate a metrics gauge or write a manifest to disk for the deployment pipeline to verify against the previous build.

## Step 15 — Serve with hardened timeouts and graceful shutdown

The same shape as the graceful-shutdown example: goroutine-driven start, signal-driven drain, bounded grace period via `Shutdown(ctx)`. The timeout values match the throughput characteristics of a JSON API: short read window, moderate write window, long idle for keep-alive efficiency.

```go
srv := &http.Server{
    Addr:         ":8080",
    Handler:      r,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  120 * time.Second,
}
```

For the full goroutine-and-Shutdown(ctx) drain, see the graceful-shutdown example — this file uses the identical pattern.

## Common questions

<section data-conversation="rest-api-patterns">

### How do I structure CRUD routes for a single resource?

Mount the resource under a `Group` (e.g. `books := v1.Group("/books")`) and register `GET ""`, `POSTE ""`, `GETE "/:id"`, `PUTE "/:id"`, `PATCH "/:id"`, `DELETEE "/:id"`. The error-returning variants (`POSTE`, `GETE`, `PUTE`, `DELETEE`) hand failures to the central `ErrorHandler`, so handlers stay free of response-writing boilerplate.

### How do I validate the request body before touching the store?

Decode into a typed struct, validate with a separate function, and return `mm.Error(http.StatusUnprocessableEntity, ...)` when validation fails. The `ErrorHandler` from Step 2 turns the typed error into the JSON response with the right status. The example's `createBook` handler shows the pattern end-to-end.

### How do I version the API?

Mount each version under its own prefix — `r.Group("/api/v1")` in this example — and register the version-specific handlers on the group. Routes that survived unchanged across versions can be registered on a shared registration function; routes that diverge are registered separately. The example carries one version on purpose because adding `/v2` is documentation, not code.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/rest-api/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/rest-api/main.go) at the v1.0.1 tag. The upstream file also contains the in-process `Store` (books, authors, reviews), every CRUD handler, the `buildLegacyMux` sub-handler, the FastHandler `metricsHandler` and `fastEcho`, and the `findBookByID` lookup helper — follow the link for the full program.
