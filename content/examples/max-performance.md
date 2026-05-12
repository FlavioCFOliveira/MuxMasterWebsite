---
datePublished: 2026-05-12
---

# Maximum-performance example

A complete, runnable program that stacks **every v1.1.0 opt-in** — `PoolRequestBundle`, `PoolFastParams`, `Pre`, `Use`, `HandleFast`, `UseFast`, and a `Mount`ed `net/http/pprof` tree — plus an in-process `/bench` endpoint that measures the live speed-up on the user's hardware and returns the result as JSON. Treat this page as the **operational companion** to the [Maximum performance guide](/docs/max-performance), which covers the contracts and the audit.

## Step 1 — Enable both pool opt-ins

Both pools eliminate the per-request allocation on the routing layer. The lifetime contract — handlers must not retain `*http.Request` (on `Handle`) or the `Params` slice (on `HandleFast`) past return — is documented in [Maximum performance](/docs/max-performance#lifetime-contract--what-you-must-not-do). Every handler in this example is written to satisfy that contract.

```go
mux := mm.New()

// PoolRequestBundle: recycles the fused reqBundle.
// Drops 1-param routes from 105 ns / 384 B / 1 alloc to ~45 ns / 0 B / 0 allocs.
mux.PoolRequestBundle = true

// PoolFastParams: recycles the Params slice handed to FastHandler routes.
// Drops FastParam routes from 50 ns / 32 B / 1 alloc to ~44 ns / 0 B / 0 allocs.
mux.PoolFastParams = true
```

## Step 2 — `Pre` for cross-cutting policy

`Pre` runs **once per request, before route lookup**, so any work it does (request-ID generation, panic recovery, IP rewriting) is paid uniformly across **both** `Handle` and `HandleFast` routes. Use `Pre` for policy that must wrap every request.

```go
mux.Pre(
    mw.RequestID(),                  // X-Request-Id propagation
    mw.RecovererWithLogger(log),     // recover from panics in handlers
)
```

## Step 3 — `Group` + `Use` for the JSON REST API

`Use` applies stdlib middleware at route registration time. It wraps **only `Handle` routes** — registering a `HandleFast` route on a `Mux` that has `Use` panics at registration. The panic is deliberate: silently mixing the two would let `HandleFast` routes bypass authentication, logging, or any other policy you intended to apply globally.

```go
v1 := mux.Group("/v1")
v1.Use(mw.Logger(os.Stdout))

v1.GET("/users/:id", getUser)                              // 1 param, 0 alloc
v1.GET("/users/:id/orders/:orderID", getUserOrder)         // 2 params, 0 alloc
v1.GET("/orgs/:org/repos/:repo/issues/:num", getRepoIssue) // 3 params, 0 alloc
v1.GET("/static/*filepath", listStaticFile)                // catch-all, 0 alloc
v1.POST("/users", createUser)

// Regex-constrained route on a different prefix (a regex param and a `:name`
// param cannot share the same parent in the radix tree).
v1.GET("/profiles/{id:[0-9]+}", getUserProfile)

// Background-work pattern.
v1.POST("/events", postEvent)
```

## Step 4 — `HandleFast` + `UseFast` for the latency-sensitive hot path

`HandleFast` bypasses the standard context allocation entirely. Parameters arrive as a third argument (`mm.Params`) instead of through `r.Context()`. Stdlib `Use` middleware is not applied — use `UseFast` for the `FastMiddleware` family, or rely on `Pre` for cross-cutting policy.

```go
mux.UseFast(fastTimer(log))
mux.GETFast("/v1/health", healthFast)
mux.GETFast("/v1/metrics/:metric", metricsFast)
```

`fastTimer` is a `FastMiddleware` — the fast-path equivalent of stdlib middleware:

```go
func fastTimer(log *slog.Logger) mm.FastMiddleware {
    return func(next mm.FastHandler) mm.FastHandler {
        return func(w http.ResponseWriter, r *http.Request, ps mm.Params) {
            start := time.Now()
            next(w, r, ps)
            log.Debug("fast", "path", r.URL.Path, "elapsed", time.Since(start))
        }
    }
}
```

Latency target: ~25 ns for static, ~44 ns for 1-parameter routes with `PoolFastParams` on.

## Step 5 — The background-work handler

`postEvent` is the canonical example of the **body-drain-before-spawn pattern**: every value the goroutine needs is captured by value (`body`, `requestID`, `remoteAddr`) before the `go` statement. The goroutine has no reference to `r`, so the bundle recycles cleanly the instant `postEvent` returns.

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

If `r` were captured directly, the goroutine would observe a recycled bundle once `postEvent` returns — exactly the use-after-free that the [audit checklist](/docs/max-performance#auditing-your-handlers) catches.

## Step 6 — The `/bench` endpoint: measure pool wins live

The `/bench` endpoint builds **two `Mux` instances** — one default, one with `PoolRequestBundle` enabled — registers the same `/users/:id` route on each, and times 200 000 dispatches against each through `httptest`. It returns a JSON payload with the per-op latency, the per-op allocations measured via `runtime.MemStats`, and the speed-up ratio.

```go
func benchHandler(w http.ResponseWriter, r *http.Request) {
    const iterations = 200_000

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

    startDefault := time.Now()
    allocsDefault := runMeasured(iterations, func() { mux.ServeHTTP(rec, req) })
    nsDefault := time.Since(startDefault).Nanoseconds() / int64(iterations)

    startPool := time.Now()
    allocsPool := runMeasured(iterations, func() { muxPool.ServeHTTP(rec, req) })
    nsPool := time.Since(startPool).Nanoseconds() / int64(iterations)

    _ = json.NewEncoder(w).Encode(map[string]any{
        "route":         "/users/:id",
        "iterations":    iterations,
        "default":       benchResult{NsPerOp: nsDefault, AllocsPerOp: allocsDefault},
        "pooled":        benchResult{NsPerOp: nsPool, AllocsPerOp: allocsPool},
        "speedup_ratio": fmt.Sprintf("%.2fx", float64(nsDefault)/float64(nsPool)),
        "go":            runtime.Version(),
    })
}
```

`runMeasured` snapshots `runtime.MemStats.Mallocs` around the loop and divides by the iteration count — a coarser but more portable measurement than `testing.B.AllocsPerOp`.

## Step 7 — `Mount` `net/http/pprof` for production profiling

`Mount` grafts another `http.Handler` onto the `Mux` with prefix stripping. Mounting `net/http/pprof`'s default `ServeMux` exposes the full profiler endpoint suite (`/debug/pprof/profile`, `/heap`, `/goroutine`, `/block`, etc.) without rewriting MuxMaster routes for each. The `_ "net/http/pprof"` blank import is what registers those endpoints onto `http.DefaultServeMux`.

```go
import _ "net/http/pprof" // attaches /debug/pprof/* to http.DefaultServeMux

mux.Mount("/debug/pprof", http.DefaultServeMux)
```

Capture a CPU profile under load:

```bash
curl http://localhost:8080/debug/pprof/profile?seconds=10 > cpu.prof
go tool pprof -top -cum cpu.prof
```

## Try it

```bash
go run .

# REST routes — zero allocations on the pool path
curl http://localhost:8080/v1/health
curl http://localhost:8080/v1/users/42
curl http://localhost:8080/v1/orgs/acme/repos/api/issues/123

# Live in-process benchmark — returns JSON with speedup_ratio
curl http://localhost:8080/bench

# Live configuration snapshot
curl http://localhost:8080/config
```

The `/bench` response on the reference hardware (AMD Ryzen 9 5900HX, Go 1.26.2) reports `speedup_ratio: "2.40x"` and `allocs_per_op: 0` on the pooled mux, matching the headline numbers in [Benchmarks](/benchmarks).

## Source

Every code excerpt above is lifted verbatim from [`examples/max-performance/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/examples/max-performance/main.go) at the v1.1.0 tag. The upstream file also contains the full handler set (`getUserOrder`, `getRepoIssue`, `createUser`, `getUserProfile`, `listStaticFile`, `metricsFast`), the `/config` endpoint, and the graceful-shutdown wiring — follow the link for the full program.

## See also

- [Maximum performance guide](/docs/max-performance) — the lifetime contract, the failure modes, the audit checklist, and the four recipe patterns that this example operationalises.
- [Benchmarks](/benchmarks) — the v1.1.0 per-route and competitor tables that the `/bench` endpoint reproduces in miniature.
- [Upload-file example](/examples/upload-file) — the body-drain-before-spawn pattern shown at full scale on multipart uploads.
- [REST API example](/examples/rest-api) — the canonical CRUD service, without the pool opt-ins.
