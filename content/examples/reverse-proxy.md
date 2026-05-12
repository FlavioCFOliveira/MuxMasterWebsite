---
datePublished: 2026-05-12
---

# Reverse-proxy example

A production-shaped HTTP gateway built on top of MuxMaster and the standard library's [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy): **catch-all path routing**, **round-robin load balancing** with a lock-free atomic counter, **per-route gating** for an admin upstream, and the opt-in `PoolRequestBundle` for zero-allocation dispatch on every proxied request.

## Why this example is pool-safe

`PoolRequestBundle` recycles the per-request bundle the instant the handler returns. `httputil.ReverseProxy` **synchronously** forwards the request and waits for the upstream response before returning — it never spawns a goroutine that survives `ServeHTTP`. The `Rewrite` hook mutates the bundle's `r.URL` inline, never captures `r`, and returns before the proxy hands the response back to the client. The contract is satisfied; pooling is safe.

The original `*http.Request` is also never mutated: `Rewrite` writes through `pr.Out`, the fresh outbound request the proxy will send upstream.

## Step 1 — Construct the gateway router

```go
mux := mm.New()
mux.PoolRequestBundle = true
mux.Pre(mw.RequestID(), mw.RecovererWithLogger(log))
```

`Pre` runs once per request, before routing. `RequestID` correlates every gateway log line with the proxied response; `RecovererWithLogger` protects the proxy itself against a `panic` in any per-route wrapper.

## Step 2 — Build the upstream targets

The example wires three upstreams: a `static` server on `:9001`, an `admin` server on `:9002`, and a `/api` group that fans out across both. Targets are parsed once at boot.

```go
upstream1 := mustURL("http://127.0.0.1:9001")
upstream2 := mustURL("http://127.0.0.1:9002")

staticProxy := newProxy("static", log, upstream1)
adminProxy  := newProxy("admin",  log, upstream2)
apiBalanced := newRoundRobin("api", log, upstream1, upstream2)
```

## Step 3 — `newProxy` — a single-target proxy with `Rewrite`

`Rewrite` is the post-Go-1.20 way to retarget a `ReverseProxy`. It receives a `*httputil.ProxyRequest` whose `pr.Out` field is the fresh outbound request — mutating it is what redirects traffic at the upstream layer.

```go
func newProxy(name string, log *slog.Logger, target *url.URL) http.HandlerFunc {
    rp := &httputil.ReverseProxy{
        Rewrite: func(pr *httputil.ProxyRequest) {
            pr.Out.URL.Scheme = target.Scheme
            pr.Out.URL.Host   = target.Host
            // The catch-all param "path" contains the captured suffix.
            // E.g. for the route "/static/*path", a request to
            // "/static/css/app.css" sets path = "/css/app.css".
            pr.Out.URL.Path = mm.PathParam(pr.In, "path")
            if pr.Out.URL.Path == "" {
                pr.Out.URL.Path = "/"
            }
            pr.Out.Host = target.Host
            pr.SetXForwarded()
        },
        ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
            log.Error("proxy error", "name", name, "path", r.URL.Path, "err", err)
            http.Error(w, "Bad Gateway", http.StatusBadGateway)
        },
    }
    return rp.ServeHTTP
}
```

`pr.SetXForwarded()` populates the canonical `X-Forwarded-For`, `X-Forwarded-Host`, and `X-Forwarded-Proto` headers from the inbound request — the receiving upstream sees who originally connected. `ErrorHandler` converts transport failures into a `502 Bad Gateway` so a downed upstream never leaks a misleading 5xx from `net/http`.

## Step 4 — `newRoundRobin` — lock-free load balancing

Round-robin distribution across N upstreams using `atomic.Uint64` — no mutex on the hot path. Each backend gets its own `newProxy`-wrapped handler so logs and error counters are per-target.

```go
func newRoundRobin(name string, log *slog.Logger, targets ...*url.URL) http.HandlerFunc {
    proxies := make([]http.HandlerFunc, len(targets))
    for i, t := range targets {
        proxies[i] = newProxy(fmt.Sprintf("%s[%d]", name, i), log, t)
    }
    var counter atomic.Uint64
    return func(w http.ResponseWriter, r *http.Request) {
        idx := counter.Add(1) % uint64(len(proxies))
        proxies[idx](w, r)
    }
}
```

`atomic.Uint64.Add` is wait-free on every architecture MuxMaster supports — the gateway scales linearly with concurrent connections.

## Step 5 — Wire the routes

Catch-all parameters (`*path`) capture the full suffix and pass it through to the upstream. The same `apiBalanced` handler is registered for every relevant HTTP method.

```go
// /api/* — round-robin to :9001 + :9002
mux.GET("/api/*path", apiBalanced)
mux.POST("/api/*path", apiBalanced)
mux.PUT("/api/*path", apiBalanced)
mux.DELETE("/api/*path", apiBalanced)

// /static/* — always to :9001
mux.GET("/static/*path", staticProxy)
mux.HEAD("/static/*path", staticProxy)

// /admin/* — :9002, gated behind a token
admin := mux.Group("/admin")
admin.Use(adminAuth)
admin.GET("/*path", adminProxy)
```

The admin group uses `Use(adminAuth)` to apply the `X-Admin-Token: letmein` gate **at registration time** — the gate is wrapped into the registered handler, so it costs nothing on a request that already missed the gate.

## Try it

```bash
# Terminal 1: a fake backend on :9001
go run . backend 9001
# Terminal 2: a second fake backend on :9002
go run . backend 9002
# Terminal 3: the gateway on :8080
go run .

# In another shell
curl http://localhost:8080/api/users               # round-robin → :9001 or :9002
curl http://localhost:8080/static/x.png            # → :9001
curl -H 'X-Admin-Token: letmein' \
     http://localhost:8080/admin/dashboard         # → :9002
```

## Frequently asked questions

<section data-conversation="reverse-proxy-faq">

### Why is the proxy pool-safe when `httputil.ReverseProxy` may stream the response back?

`httputil.ReverseProxy.ServeHTTP` is synchronous — it issues the upstream request, copies the response back to the client, and **returns**. It does not spawn a goroutine that survives the call. The `Rewrite` hook mutates `pr.Out` (the fresh outbound request), never `r` itself. The lifetime contract for `PoolRequestBundle` is satisfied, so the bundle recycles cleanly the instant `ServeHTTP` returns.

### Where do I add per-upstream timeouts?

Set the `Transport` field on each `httputil.ReverseProxy`. The stdlib `http.Transport` supports `DialContext`, `ResponseHeaderTimeout`, `IdleConnTimeout`, and a per-request `*http.Request.Context()` deadline. For the gateway level, wrap each call with the `mw.Timeout(d)` middleware from `middleware/timeout.go` — it cancels the request context after `d` and returns `503 Service Unavailable` if the upstream is still in flight.

### Can I weight the round-robin?

Replace `newRoundRobin` with a weighted scheme. The simplest is a "ticket" slice (`[]http.HandlerFunc` where heavier upstreams appear more than once) plus the same `atomic.Uint64.Add` counter — no mutex, still wait-free. For dynamic weights based on health, use a smoothed weighted round robin (SWRR) implementation; both are independent of MuxMaster's routing.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/reverse-proxy/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/examples/reverse-proxy/main.go) at the v1.1.0 tag. The upstream file also contains the `runBackend` fake-backend helper used to test the gateway locally, the index page, and the graceful-shutdown wiring — follow the link for the full program.

## See also

- [Maximum performance](/docs/max-performance) — why `Rewrite` is pool-safe (the proxy returns before `ServeHTTP` exits).
- [Routing documentation](/docs/routing) — the catch-all `*path` parameter used by every proxy.
- [Server-sent events example](/examples/server-sent-events) — another pool-safe streaming pattern.
