# Authn example

Two authentication strategies on the same router: HTTP Basic Auth (paired with `ThrottlePerIP` to defend against credential-stuffing per `SECURITY.md` MM-2026-0027) and an API-key middleware that hashes its keys with SHA-256 at construction time so per-request cost is one hash plus a `[32]byte` map lookup. Reach for this example when a service needs simple username-and-password or shared-key protection without a full session layer.

## Step 1 — Construct the router with global middleware

`RequestID`, `Logger`, and `Recoverer` belong on every request, public or protected. `Use` registers them at the top level so both the public routes and the auth-protected groups inherit them.

```go
r := mm.New()

r.Use(
    mw.RequestID(),
    mw.Logger(os.Stdout),
    mw.RecovererWithLogger(log),
)
```

The trace id `RequestID` injects becomes available to every handler via `mw.GetRequestID(r.Context())` and surfaces in the JSON responses below — useful for end-to-end correlation between client logs and server logs.

## Step 2 — Wire JSON-shaped error, not-found, and method-not-allowed handlers

The default handlers write plain text. An API consumer expects JSON, so the example replaces all three with handlers that emit `{"error": "..."}` and a status code mapped from `mm.HTTPError` when the underlying handler returns a typed error.

```go
r.NotFound = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    _ = mm.JSON(w, http.StatusNotFound, errMsg("not found"))
})
r.MethodNotAllowed = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    _ = mm.JSON(w, http.StatusMethodNotAllowed, errMsg("method not allowed"))
})
r.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
    code := http.StatusInternalServerError
    var he mm.HTTPError
    if errors.As(err, &he) {
        code = he.StatusCode()
    }
    _ = mm.JSON(w, code, errMsg(err.Error()))
}
```

`MethodNotAllowed` is the dispatcher's response when the URL matches a registered pattern but the HTTP method does not — covering it explicitly avoids the framework default leaking through.

## Step 3 — Register the public routes

`/health` is a high-frequency probe target — `GETFast` registers it on the `FastHandler` path so the request resolves with zero allocations. The advisory `/` page hands the reader the two protected URLs to try.

```go
r.GETFast("/health", func(w http.ResponseWriter, _ *http.Request, _ mm.Params) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"status":"ok"}`)
})

r.GET("/", func(w http.ResponseWriter, _ *http.Request) {
    _ = mm.JSON(w, http.StatusOK, map[string]string{
        "hint": "try GET /admin/dashboard (Basic Auth) or GET /api/profile (X-API-Key)",
    })
})
```

`FastHandler` and `http.HandlerFunc` coexist on the same router; the choice is per-route.

## Step 4 — Protect `/admin` with Basic Auth + per-IP throttle

`BasicAuth` alone has no rate limiting — an attacker can attempt unlimited credentials per second. Composing it with `ThrottlePerIP` (10 concurrent in-flight requests per IP, 5-second queue timeout) closes the online brute-force window without rejecting legitimate users behind shared NAT.

```go
admin := r.Group("/admin")
admin.Use(
    mw.ThrottlePerIP(10, 5*time.Second, nil),
    mw.BasicAuth("Admin Area", map[string]string{
        "admin":  "s3cr3t",
        "viewer": "readonly",
    }),
)
```

Order matters: the throttle MUST run before the auth check, so an attacker hitting the wall pays the throttle cost, not just the auth-validation cost. The credentials map is illustrative only; production code reads from a database with hashed passwords.

## Step 5 — Register the protected admin handlers

The dashboard handler returns a JSON body and includes the request id; the user-delete handler shows the error-returning shape (`DELETEE` — note the trailing `E`) so a 404 is signalled by returning `mm.Error(http.StatusNotFound, ...)` rather than calling `http.Error` manually.

```go
admin.GET("/dashboard", func(w http.ResponseWriter, r *http.Request) {
    _ = mm.JSON(w, http.StatusOK, map[string]any{
        "page":     "dashboard",
        "trace_id": mw.GetRequestID(r.Context()),
    })
})

admin.DELETEE("/users/:id", func(w http.ResponseWriter, r *http.Request) error {
    id := mm.PathParam(r, "id")
    if id == "0" {
        return mm.Error(http.StatusNotFound, fmt.Errorf("user %q not found", id))
    }
    mm.NoContent(w)
    return nil
})
```

The `ErrorHandler` from Step 2 turns the returned error into a JSON 404 — no boilerplate inside the handler.

## Step 6 — Protect `/api` with `APIKey`

`APIKey` is the alternative authentication strategy on the same router. It reads the `X-API-Key` header, looks up the value in a map of SHA-256-hashed keys built at construction time, and injects the matched identity into the request context.

```go
api := r.Group("/api")
api.Use(mw.APIKey(mw.APIKeyOptions{
    Keys: apiKeys,
    // Header defaults to "X-API-Key"; override here if you need a different one.
}))
```

Because the keys are hashed at construction time, per-request cost is one SHA-256 over the incoming header value plus a `[32]byte` map lookup — no per-request iteration or constant-time string comparison loop. Rotating keys is a process restart with the new map.

## Step 7 — Register the protected API handlers

Inside the protected group the handlers retrieve the matched identity with `mw.GetAPIKeyIdentity(ctx)`. The error-returning shape is again available — `GETE` for the item lookup mirrors `DELETEE` from Step 5.

```go
api.GET("/profile", func(w http.ResponseWriter, r *http.Request) {
    owner, _ := mw.GetAPIKeyIdentity(r.Context())
    _ = mm.JSON(w, http.StatusOK, map[string]string{
        "owner":    owner,
        "trace_id": mw.GetRequestID(r.Context()),
    })
})

api.GETE("/items/:id", func(w http.ResponseWriter, r *http.Request) error {
    id := mm.PathParam(r, "id")
    if id == "0" {
        return mm.Error(http.StatusNotFound, fmt.Errorf("item %q not found", id))
    }
    return mm.JSON(w, http.StatusOK, map[string]string{"id": id, "name": "Widget " + id})
})
```

The `_` on the second return of `GetAPIKeyIdentity` is safe inside the protected group — the middleware short-circuits with 401 before the handler runs when the header is missing or wrong.

## Step 8 — Serve with hardened timeouts and graceful shutdown

The same timeout set as the other examples — `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout` — closes the slowloris vector. Running in a goroutine and draining on `SIGINT`/`SIGTERM` is the production-recommended shape (see the graceful-shutdown example for the full pattern).

```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           r,
    ReadHeaderTimeout: 30 * time.Second,
    ReadTimeout:       60 * time.Second,
    WriteTimeout:      60 * time.Second,
    IdleTimeout:       120 * time.Second,
    MaxHeaderBytes:    1 << 20, // 1 MiB
}
```

Setting these timeouts is on the application — MuxMaster is an `http.Handler` and does not configure the surrounding `http.Server` itself.

## Common questions

<section data-conversation="authn-patterns">

### How do I protect a group of routes with HTTP Basic Auth?

Mount the protected routes under a `Group` and call `g.Use(mw.BasicAuth("realm", credentials))`. The credentials map is `username → password` for development; production code reads it from a database with hashed passwords and rebuilds the middleware on rotation.

### Why does the example pair `BasicAuth` with `ThrottlePerIP`?

`BasicAuth` alone has no rate limiting; an attacker can attempt unlimited credentials per second. `ThrottlePerIP` bounds concurrent in-flight requests per source IP — the example uses 10 with a 5-second queue timeout — which makes online brute-force practically infeasible without rejecting legitimate users behind shared NAT.

### How does `APIKey` validate the incoming header?

`APIKey` SHA-256 hashes every configured key at construction time, so the per-request cost is one SHA-256 over the incoming `X-API-Key` value plus a `[32]byte` map lookup. The matched identity is injected into the request context; handlers retrieve it with `mw.GetAPIKeyIdentity(ctx)`.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/authn/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/authn/main.go) at the v1.0.1 tag. The upstream file also includes the in-process API-key map, the `errMsg` helper, and the goroutine-driven server start with `Shutdown(ctx)` drain — follow the link for the full program.
