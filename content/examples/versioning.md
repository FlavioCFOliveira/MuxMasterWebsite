---
datePublished: 2026-05-12
---

# Versioning example

Two complementary API-versioning strategies on the same router: **path-based** (`/api/v1/...`, `/api/v2/...`) and **header-based** (`Accept: application/vnd.muxmaster+json;v=N`). The example pairs both strategies with **nested groups** for an admin section and the **opt-in `PoolRequestBundle`** so every dispatch — including the three-level-deep `/api/v2/admin/users/:id/audit` — runs at ~45 ns / 0 B / 0 allocs.

## Step 1 — Construct the router and enable maximum performance

Group composition has zero runtime cost in MuxMaster: middleware is wrapped at route-registration time, not on every request. A three-level-deep group dispatches at the same cost as a flat route. Enabling `PoolRequestBundle` on top recycles the per-request bundle through `sync.Pool`s, eliminating the routing allocation entirely.

```go
mux := mm.New()

// Maximum performance: every parameterised route under /v1, /v2, /admin
// dispatches at ~45 ns / 0 B / 0 allocs.
mux.PoolRequestBundle = true
```

The lifetime contract for `PoolRequestBundle` (handlers must not retain `*http.Request` past return) is documented in [Maximum performance](/docs/max-performance). This example never spawns goroutines from its handlers, so the contract holds trivially.

## Step 2 — Wire cross-cutting `Pre` middleware

`Pre` middleware wraps the whole dispatch — it runs **once per request, before routing**, so any work it does (request-ID generation, panic recovery, version negotiation) is free in the per-route cost. The version-dispatch hook rewrites the URL path in place before the radix tree gets to look at it.

```go
mux.Pre(
    mw.RequestID(),
    mw.RecovererWithLogger(log),
    acceptHeaderVersionDispatch, // header-based routing for /api/...
)
```

The order matters: `RequestID` first, so every downstream log line and panic trace carries the correlation header; `RecovererWithLogger` next, so the version-dispatch hook itself is protected; the version-negotiation last, so it never runs on a request that has already panicked.

## Step 3 — Path-based versioning with `Group`

Two top-level groups expose `/api/v1/...` and `/api/v2/...` directly. Each route in a group dispatches at the same cost as a top-level route — the prefix is folded into the radix tree at registration time.

```go
v1 := mux.Group("/api/v1")
v1.GET("/users/:id", v1GetUser)
v1.GET("/users", v1ListUsers)
v1.GET("/health", v1Health)

v2 := mux.Group("/api/v2")
v2.GET("/users/:id", v2GetUser)
v2.GET("/users", v2ListUsers)
v2.GET("/health", v2Health)
```

The `v1*` and `v2*` handlers are independent — `v2GetUser` returns a HATEOAS shape with `_links`, while `v1GetUser` returns the flat shape `v1` consumers expect. Schema diverges; routing does not.

## Step 4 — Nested groups for an admin section

`Group` returns a `*Mux`, so any group can spawn its own sub-groups with their own middleware stack. Stdlib middleware applied at the group level wraps every route under it **at registration time**, so the cost is paid once on boot and never on a request.

```go
v1Admin := v1.Group("/admin")
v1Admin.Use(adminMiddleware) // applied once at reg, free at runtime
v1Admin.GET("/dashboard", v1AdminDashboard)
v1Admin.GET("/users/:id/audit", v1AdminAudit)

v2Admin := v2.Group("/admin")
v2Admin.Use(adminMiddleware)
v2Admin.GET("/dashboard", v2AdminDashboard)
v2Admin.GET("/users/:id/audit", v2AdminAudit)
```

`adminMiddleware` is a toy `X-Admin-Token: letmein` gate — replace with real authentication (`mw.JWTAuth`, `mw.OAuth2Introspect`, or `mw.APIKey`) in production.

## Step 5 — Header-based versioning via a `Pre`-positioned URL rewrite

The header-based pattern is a `Pre` middleware that rewrites `/api/...` to `/api/vN/...` based on the request's `Accept` header **before** the radix tree dispatches it. The rewrite happens once per request, in `Pre`, so it does not appear on the per-route hot path at all.

```go
func acceptHeaderVersionDispatch(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Only apply to /api/ paths that DON'T already have a /vN/ prefix.
        if !strings.HasPrefix(r.URL.Path, "/api/") {
            next.ServeHTTP(w, r)
            return
        }
        rest := r.URL.Path[len("/api/"):]
        if strings.HasPrefix(rest, "v1/") || strings.HasPrefix(rest, "v2/") {
            next.ServeHTTP(w, r)
            return
        }

        // Parse `Accept: ...;v=N` — default to v1 if not specified.
        v := "1"
        if a := r.Header.Get("Accept"); strings.Contains(a, "v=2") {
            v = "2"
        }
        // Rewrite r.URL.Path in place. The bundle copy is mutable; the
        // original request is never modified.
        r.URL.Path = "/api/v" + v + "/" + rest
        next.ServeHTTP(w, r)
    })
}
```

The two strategies coexist on the same router. A client that pins `/api/v1/users/42` gets v1; a client that calls `/api/users/42` with `Accept: application/vnd.muxmaster+json;v=2` gets v2; a client that calls `/api/users/42` with no `Accept` header gets v1.

## Try it

```bash
# Path-based
curl http://localhost:8080/api/v1/users/42
curl http://localhost:8080/api/v2/users/42

# Header-based (rewrites to /api/v2/users/42 in Pre)
curl -H 'Accept: application/vnd.muxmaster+json;v=2' http://localhost:8080/api/users/42

# Admin (gated by X-Admin-Token)
curl -H 'X-Admin-Token: letmein' http://localhost:8080/api/v1/admin/dashboard
curl -H 'X-Admin-Token: letmein' http://localhost:8080/api/v2/admin/dashboard
```

## Frequently asked questions

<section data-conversation="versioning-faq">

### How do I add a v3 alongside v1 and v2?

Create a new top-level group: `v3 := mux.Group("/api/v3")` and register the v3 handlers on it. Extend the `acceptHeaderVersionDispatch` `Pre` middleware so `v=3` rewrites to `/api/v3/...`. The radix tree absorbs the third version at zero per-request cost — group composition is wrapped at registration, not on dispatch.

### What happens if a request has both a `/v2/` path and `Accept: ...;v=1`?

The path wins. `acceptHeaderVersionDispatch` short-circuits when `r.URL.Path` already starts with `/api/v1/` or `/api/v2/` (Step 5), so a request to `/api/v2/users/42` with `Accept: application/vnd.muxmaster+json;v=1` dispatches to the v2 handler. Path-pinned URLs are always authoritative; the `Accept` header only acts when the client did not pin a version.

### Is the in-place `r.URL.Path` rewrite safe with `PoolRequestBundle`?

Yes. Under `PoolRequestBundle = true`, `r` is the bundle's copy of the request — mutating `r.URL.Path` modifies the bundle, not the caller's original `*http.Request`. The mutation is local to the dispatch and discarded when the bundle is returned to the pool. The lifetime contract still applies: the handler must not retain `r` past return.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/versioning/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/examples/versioning/main.go) at the v1.1.0 tag. The upstream file also contains the full handler set (`v1GetUser`, `v1ListUsers`, `v2GetUser` with HATEOAS links, the two admin dashboards, two audit endpoints) and the graceful-shutdown wiring — follow the link for the full program.

## See also

- [Maximum performance](/docs/max-performance) — the `PoolRequestBundle` contract that makes Step 1 zero-allocation.
- [Groups documentation](/docs/groups) — the `Group` / `Use` / `Mount` idioms used in Steps 3 and 4.
- [REST API example](/examples/rest-api) — a single-version CRUD service, the natural starting point before versioning.
