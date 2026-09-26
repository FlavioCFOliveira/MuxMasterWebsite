---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# Reverse-proxy example

This program is an HTTP gateway built on MuxMaster and the standard library's [`httputil.ReverseProxy`](https://pkg.go.dev/net/http/httputil#ReverseProxy): it routes by catch-all path, balances `/api/*` across two upstreams with a lock-free atomic counter, and gates an admin upstream behind a header check. It also shows when **not** to enable `PoolRequestBundle`: from v1.2.0 onward, the gateway keeps pooling off, because a proxy built on `net/http.Transport` is not pool-safe.

## Step 1 — Construct the gateway router with pooling off

The gateway router keeps `PoolRequestBundle` set to `false`, because `net/http.Transport`, which `httputil.ReverseProxy` uses, can start a background dial goroutine under concurrent load that reads the request context after the handler has returned. With pooling on, that goroutine would read a recycled, zeroed request bundle; upstream reproduced the resulting nil-pointer crash under load (documented in the example's package comment). `Pre` registers `RequestID`, which correlates every gateway log line, and `RecovererWithLogger`, which protects the gateway against a panic in a per-route wrapper.

```go
mux := mm.New()
// PoolRequestBundle stays OFF: net/http.Transport (used internally by
// httputil.ReverseProxy) can start a background dial goroutine that
// reads the request context after this handler returns — see the
// package doc comment above for the full explanation and the crash
// evidence. Enabling pooling here is a use-after-free under load.
mux.PoolRequestBundle = false
mux.Pre(mw.RequestID(), mw.RecovererWithLogger(log))
```

The hazard applies to any reverse proxy built on `net/http.Transport`, not only to this example. The [Maximum performance guide](/docs/max-performance#special-case-libraries-that-spawn-background-goroutines) lists it among the cases where the pool must stay off.

## Step 2 — Build the upstream targets

The gateway parses its two upstream URLs once at start-up and builds three handlers: `staticProxy` always forwards to `:9001`, `adminProxy` always forwards to `:9002`, and `apiBalanced` alternates between both.

```go
upstream1 := mustURL("http://127.0.0.1:9001")
upstream2 := mustURL("http://127.0.0.1:9002")

staticProxy := newProxy("static", log, upstream1)
adminProxy := newProxy("admin", log, upstream2)
apiBalanced := newRoundRobin("api", log, upstream1, upstream2)
```

## Step 3 — `newProxy`: a single-target proxy with `Rewrite`

`newProxy` retargets each request with the `Rewrite` hook of `httputil.ReverseProxy`, which receives a `*httputil.ProxyRequest` whose `pr.Out` field is the fresh outbound request. The hook copies the catch-all parameter `path` into the outbound URL, so the upstream sees the path without the gateway prefix.

```go
func newProxy(name string, log *slog.Logger, target *url.URL) http.HandlerFunc {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// pr.Out is a fresh request the proxy will send upstream. We
			// retarget its URL to the upstream's scheme/host, preserve
			// the captured wildcard path, and propagate X-Forwarded-* headers.
			pr.Out.URL.Scheme = target.Scheme
			pr.Out.URL.Host = target.Host
			// The catch-all param "path" contains the captured suffix; if
			// the route registered "/static/*path", a request to
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

`pr.SetXForwarded()` sets the `X-Forwarded-For`, `X-Forwarded-Host`, and `X-Forwarded-Proto` headers from the inbound request, so the upstream sees who originally connected. `ErrorHandler` turns a transport failure into `502 Bad Gateway`.

## Step 4 — `newRoundRobin`: lock-free load balancing

`newRoundRobin` distributes requests across several upstreams with an `atomic.Uint64` counter, so no mutex is taken on the request path. Each upstream gets its own `newProxy` handler, so log lines name the target that served the request.

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

## Step 5 — Wire the routes

Catch-all parameters (`*path`) capture the full suffix of the request path and hand it to the proxy. The same `apiBalanced` handler is registered for each HTTP method the API accepts, and the admin routes live in a group whose `Use(adminAuth)` middleware checks the `X-Admin-Token` header.

```go
// /api/* is a catch-all that fans out across upstreams round-robin.
mux.GET("/api/*path", apiBalanced)
mux.POST("/api/*path", apiBalanced)
mux.PUT("/api/*path", apiBalanced)
mux.DELETE("/api/*path", apiBalanced)

// /static/* always goes to upstream1.
mux.GET("/static/*path", staticProxy)
mux.HEAD("/static/*path", staticProxy)

// /admin/* always goes to upstream2 — gated behind a token.
admin := mux.Group("/admin")
admin.Use(adminAuth)
admin.GET("/*path", adminProxy)
```

`Use` middleware is applied when the route is registered, so `adminAuth` wraps only the routes registered on the `admin` group after the `Use` call.

## Step 6 — Enable pooling only where it is safe

The fake backend that the example runs for local testing enables `PoolRequestBundle`, because it only reads its own request and writes its own response and never proxies through `net/http.Transport`. The contrast with Step 1 is the rule to apply: the pool is a per-router decision that depends on what the handlers do after they return.

```go
func runBackend(port string) {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mux := mm.New()
	// Unlike the gateway above, this backend only reads its own request and
	// writes its own response — it never proxies through net/http.Transport,
	// so it has none of the background-dial-goroutine hazard. Pooling is
	// genuinely safe here.
	mux.PoolRequestBundle = true
	mux.GET("/*path", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "backend :%s reached path=%s headers=%v\n",
			port, r.URL.Path, r.Header.Get("X-Forwarded-For"))
	})
	// …
}
```

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

## Common questions

<section data-conversation="reverse-proxy-faq">

### Can I enable `PoolRequestBundle` on a MuxMaster reverse proxy?

No: a MuxMaster router whose handlers proxy through `httputil.ReverseProxy` or any other `net/http.Transport` client must keep `PoolRequestBundle` set to `false`.

Under concurrent load, `net/http.Transport` can start a dial goroutine that reads the request context after the handler returns. With pooling on, that context belongs to a recycled bundle, and upstream reproduced a crash in that situation. Enable the pool only on routers whose handlers never let the request outlive them, such as the example's backends.

### Where do I add per-upstream timeouts?

Set the `Transport` field of each `httputil.ReverseProxy` to an `*http.Transport` with the timeouts you need, such as `ResponseHeaderTimeout` and `IdleConnTimeout`.

The `mw.Timeout(d)` middleware only sets a deadline on the request context; it does not write a response when the deadline passes. `httputil.ReverseProxy` sends the outbound request with that context, so the upstream call is cancelled when the deadline expires.

### Can I weight the round-robin?

Yes: replace `newRoundRobin` with a weighted scheme, for example a slice in which a heavier upstream appears more than once, indexed by the same `atomic.Uint64` counter.

Weighting is independent of MuxMaster's routing; the router only needs an `http.Handler` for each route.

</section>

## See also

- [Maximum performance](/docs/max-performance#special-case-libraries-that-spawn-background-goroutines) — why `net/http.Transport`-based proxies must not enable the pool.
- [Routing documentation](/docs/routing) — the catch-all `*path` parameter used by every proxy route.
- [Server-sent events example](/examples/server-sent-events) — a streaming handler that is pool-safe because it does not return until the stream ends.

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/reverse-proxy/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/examples/reverse-proxy/main.go) at the v1.3.0 tag. The upstream file also contains the package comment that documents the pooling crash, the `adminAuth` middleware, the index page, and the graceful-shutdown wiring.

Source: <https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/examples/reverse-proxy>
