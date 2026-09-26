---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# Maximum-performance example

This runnable program configures MuxMaster for zero routing-layer allocations: it enables `PoolRequestBundle` and `PoolFastParams`, separates `Pre`, `Use`, and `UseFast` middleware, serves hot routes with `HandleFast`, exposes `net/http/pprof`, and adds an in-process `/bench` endpoint that compares a default and a pooled router on your own hardware. Treat this page as the operational companion to the [Maximum performance guide](/docs/max-performance), which covers the lifetime contracts and the audit.

## Step 1 — Enable both pool opt-ins

The two pool flags remove the per-request allocation on the routing layer: `PoolRequestBundle` recycles the request bundle of `Handle` routes, and `PoolFastParams` recycles the `Params` slice of `HandleFast` routes. The lifetime contract — handlers must not retain `*http.Request` (on `Handle`) or the `Params` slice (on `HandleFast`) past return — is documented in [Maximum performance](/docs/max-performance#lifetime-contract--what-you-must-not-do). Every handler in this example is written to satisfy that contract.

```go
mux := mm.New()
// …
mux.PoolRequestBundle = true
// …
mux.PoolFastParams = true
```

The elided lines are comments in the upstream file. They quote per-request timings from the v1.1.0 era; the current measurements are on the [Benchmarks](/benchmarks) page.

## Step 2 — `Pre` for cross-cutting policy

`Pre` middleware runs once per request, before route lookup, so its work (request-ID generation, panic recovery, IP rewriting) applies uniformly to both `Handle` and `HandleFast` routes. Use `Pre` for policy that must wrap every request.

```go
mux.Pre(
	mw.RequestID(),              // X-Request-Id propagation
	mw.RecovererWithLogger(log), // recover from panics in handlers
)
```

## Step 3 — `Group` and `Use` for the JSON REST API

`Use` applies standard `func(http.Handler) http.Handler` middleware at route registration time and wraps only `Handle` routes. Registering a `HandleFast` route after `Use` on the same scope panics; the panic is deliberate, because silently mixing the two would let fast routes bypass authentication, logging, or any other policy you intended to apply.

```go
v1 := mux.Group("/v1")
v1.Use(mw.Logger(os.Stdout))

// JSON REST routes — use Handle (stdlib http.Handler signature).
v1.GET("/users/:id", getUser)                              // 1 param, 0 alloc
v1.GET("/users/:id/orders/:orderID", getUserOrder)         // 2 params, 0 alloc
v1.GET("/orgs/:org/repos/:repo/issues/:num", getRepoIssue) // 3 params, 0 alloc
v1.GET("/static/*filepath", listStaticFile)                // catch-all, 0 alloc
v1.POST("/users", createUser)

// Regex-constrained route on a different prefix so it does not collide
// with the ":id" wildcard above (a regex param and a `:name` param cannot
// share the same parent in the radix tree).
v1.GET("/profiles/{id:[0-9]+}", getUserProfile) // regex-constrained

// Background work pattern — copy primitives before spawning a goroutine.
v1.POST("/events", postEvent)
```

## Step 4 — `HandleFast` and `UseFast` for the hot path

`HandleFast` routes receive their parameters as a third argument (`mm.Params`) instead of through `r.Context()`, so they do not copy the request. Standard `Use` middleware does not wrap them; the example registers `FastMiddleware` with `UseFast` and relies on `Pre` for cross-cutting policy.

```go
mux.UseFast(fastTimer(log))
mux.GETFast("/v1/health", healthFast)
mux.GETFast("/v1/metrics/:metric", metricsFast)
```

## Step 5 — Write a `FastMiddleware`

`fastTimer` is a `FastMiddleware` (`func(FastHandler) FastHandler`), the fast-path equivalent of standard middleware, and it runs only on `HandleFast` routes. Since v1.2.0 it skips the clock read and the log call entirely when debug logging is disabled:

```go
func fastTimer(log *slog.Logger) mm.FastMiddleware {
	return func(next mm.FastHandler) mm.FastHandler {
		return func(w http.ResponseWriter, r *http.Request, ps mm.Params) {
			// log's handler defaults to slog.LevelInfo (built with nil
			// HandlerOptions), so every Debug call below is dropped. Reading
			// the clock and boxing the log arguments on every fast request
			// only to discard them is pure waste — skip both when Debug is
			// not enabled.
			if !log.Enabled(r.Context(), slog.LevelDebug) {
				next(w, r, ps)
				return
			}
			start := time.Now()
			next(w, r, ps)
			log.Debug("fast", "path", r.URL.Path, "elapsed", time.Since(start))
		}
	}
}
```

## Step 6 — The background-work handler

`postEvent` shows the body-drain-before-spawn pattern: every value the goroutine needs (`body`, `requestID`, `remoteAddr`) is copied before the `go` statement. The goroutine holds no reference to `r`, so the pooled bundle can be recycled the instant `postEvent` returns.

```go
func postEvent(w http.ResponseWriter, r *http.Request) {
	// Snapshot primitives BEFORE spawning anything async.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	requestID := mw.GetRequestID(r.Context())
	remoteAddr := r.RemoteAddr

	// Now we are safe to fan out — `body`, `requestID`, `remoteAddr` are all
	// values; the bundle can be recycled the moment we return.
	go func() {
		fmt.Fprintf(os.Stderr,
			"event accepted req=%s peer=%s bytes=%d\n",
			requestID, remoteAddr, len(body))
	}()

	w.WriteHeader(http.StatusAccepted)
}
```

If the goroutine captured `r` directly, it would read a recycled bundle after `postEvent` returns — the use-after-free that the [audit checklist](/docs/max-performance#auditing-your-handlers) catches.

## Step 7 — The `/bench` endpoint: compare default and pooled dispatch

The `/bench` endpoint builds two routers — one default, one with `PoolRequestBundle` enabled — registers the same `/users/:id` route on each, and times 200 000 dispatches against each through `httptest`. It returns JSON with the nanoseconds per request, the allocations per request (from `runtime.MemStats`), and the ratio between the two.

```go
func benchHandler(w http.ResponseWriter, r *http.Request) {
	const iterations = 200_000

	// Build two muxes — one default, one with PoolRequestBundle enabled — and
	// hit the same route on each, measuring nanoseconds per request.
	mux := mm.New()
	mux.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {})

	muxPool := mm.New()
	muxPool.PoolRequestBundle = true
	muxPool.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	rec := httptest.NewRecorder()

	// Warm-up so first-call costs do not skew the result.
	for range 1000 {
		mux.ServeHTTP(rec, req)
		muxPool.ServeHTTP(rec, req)
	}

	// Snapshot allocations before each block and divide by iteration count.
	startDefault := time.Now()
	allocsDefault := runMeasured(iterations, func() { mux.ServeHTTP(rec, req) })
	nsDefault := time.Since(startDefault).Nanoseconds() / int64(iterations)

	startPool := time.Now()
	allocsPool := runMeasured(iterations, func() { muxPool.ServeHTTP(rec, req) })
	nsPool := time.Since(startPool).Nanoseconds() / int64(iterations)
	// …
}
```

The endpoint takes a single wall-clock measurement per router, so its result varies between runs and hosts. For a comparison with confidence intervals and significance tests, use `go test -bench` with `-count=10` and `benchstat`, as described in [Measuring your own configuration](/docs/max-performance#measuring-your-own-configuration).

## Step 8 — Expose `net/http/pprof` for profiling

The blank import `_ "net/http/pprof"` registers the profiler handlers on `http.DefaultServeMux`; the example forwards every `/debug/pprof/*` request to that `ServeMux` unchanged. Since v1.2.0 the example registers a catch-all route instead of `Mount`, because `Mount` strips the prefix and `net/http/pprof` matches on the full `/debug/pprof/` path, which made every profiler page return 404.

```go
mux.GET("/debug/pprof/*filepath", func(w http.ResponseWriter, r *http.Request) {
	http.DefaultServeMux.ServeHTTP(w, r)
})
```

Capture a CPU profile under load:

```bash
curl http://localhost:8080/debug/pprof/profile?seconds=10 > cpu.prof
go tool pprof -top -cum cpu.prof
```

## Try it

```bash
go run .

curl http://localhost:8080/v1/health
curl http://localhost:8080/v1/users/42
curl http://localhost:8080/v1/orgs/acme/repos/api/issues/123

# In-process comparison of the default and pooled routers
curl http://localhost:8080/bench

# Live configuration snapshot
curl http://localhost:8080/config
```

For measured results on reference hardware, see the [Benchmarks](/benchmarks) page. For this same configuration running in production, see [Built with MuxMaster](/built-with-muxmaster): the documentation website you are reading enables `PoolRequestBundle` and `PoolFastParams` and registers its middleware with `Pre`.

## Common questions

<section data-conversation="max-performance-example-faq">

### What does the `/bench` endpoint measure?

The `/bench` endpoint measures the time and allocations per request of the same one-parameter route on a default router and on a router with `PoolRequestBundle = true`, running in the same process on your hardware.

It reports `ns_per_op` and `allocs_per_op` for each router and their ratio as `speedup_ratio`. It is a single in-process measurement, not a statistical benchmark.

### Why does `/bench` differ from the numbers on the Benchmarks page?

The `/bench` endpoint differs because it takes one wall-clock sample through `httptest` on your machine, while the Benchmarks page reports medians of 10 `go test -bench` samples on an AMD Ryzen 9 5900HX with significance tests.

In that campaign, a one-parameter route took 115.9 ns with 1 allocation by default and 45.84 ns with 0 allocations with `PoolRequestBundle` (MuxMaster v1.3.0, go1.27.1, 2026-09-26, `-count=10`; see [Benchmarks](/benchmarks)).

### Should I leave `pprof` exposed in production?

No: expose `pprof` only behind authentication or on a private port, because the profiler endpoints reveal Go runtime state and a profile request consumes CPU.

Serve it from a separate `http.Server` bound to a loopback or private interface, or place the route in a group protected by `mw.BasicAuth` or `mw.APIKey`.

### Can I enable `PoolRequestBundle` for some routes only?

No: `PoolRequestBundle` is a field of `*Mux` and applies to every parameterised `Handle` route of that router.

To mix pooled and unpooled handlers, run them on two `*Mux` instances, or fix the unsafe handlers with the copy-before-spawn pattern from the [audit checklist](/docs/max-performance#auditing-your-handlers) and enable the pool globally.

</section>

## See also

- [Maximum performance guide](/docs/max-performance) — the lifetime contract, the failure modes, the audit checklist, and the recipes this example puts into practice.
- [Benchmarks](/benchmarks) — the measured v1.3.0 results, v1.1.0 compared with v1.3.0, and the router comparison.
- [Built with MuxMaster](/built-with-muxmaster) — how this website uses the same pool opt-ins in production.
- [Upload-file example](/examples/upload-file) — the body-drain-before-spawn pattern on multipart uploads.
- [REST API example](/examples/rest-api) — the canonical CRUD service, without the pool opt-ins.

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/max-performance/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/examples/max-performance/main.go) at the v1.3.0 tag. The upstream file also contains the remaining handlers (`getUser`, `getUserOrder`, `getRepoIssue`, `listStaticFile`, `getUserProfile`, `createUser`, `healthFast`, `metricsFast`), the `runMeasured` helper, the `/config` endpoint, and the graceful-shutdown wiring.

Source: <https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/examples/max-performance>
