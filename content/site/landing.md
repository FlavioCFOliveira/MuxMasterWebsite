---
datePublished: 2026-05-12
---

# MuxMaster

A radix-tree HTTP router for Go. Zero dependencies, O(k) lookups.

MuxMaster routes 25 ns on static paths and, with the opt-in `PoolRequestBundle`, 45 ns on a single parameter with zero allocations (AMD Ryzen 9 5900HX, Go 1.26.2). It is **20 % faster than `httprouter`** and the only stdlib-compatible router that achieves zero allocations on parameterised routes. 100 % compatible with the `net/http` handler interface — build production services on the Go standard library.

## Highlights

- **Zero dependencies.** The router and the 17 bundled middlewares are implemented on the Go standard library alone. `go.mod` declares no `require` beyond the test fixtures.
- **Fastest stdlib-compatible router.** Static-route lookups in 25 ns / 0 allocs; one-parameter routes in 45 ns / 0 allocs with the opt-in `PoolRequestBundle`. See the tables below.
- **100 % `net/http` compatible.** Handlers stay `http.Handler`; middleware stays `func(http.Handler) http.Handler`. Adopt incrementally — your existing handlers compile unchanged.
- **Typed errors and parameters.** Optional `HandlerFuncE` threads errors through middleware. `Params.Int`, `Params.Bool`, `Params.UUID` parse and validate path parameters in one call.
- **Production-grade middleware.** `RequestID`, `Recoverer`, `Logger`, `Compress`, `RealIP`, `Timeout`, `Throttle`, `BasicAuth`, `JWTAuth`, `OAuth2Introspect`, `APIKey`, `CORS` — all hardened and audited.
- **Dogfooded.** This documentation site is itself served by a Go binary using MuxMaster as its router. The router is the documentation and the proof.

## Performance

Benchmarks measured on AMD Ryzen 9 5900HX, Go 1.26.2. Each row is consolidated from 10 × 2 s runs via `benchstat`. The full methodology, the asm listings, and the per-sprint history are archived in the upstream report at [`reports/perf-audit-2026-05-12/`](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.1.0/reports/perf-audit-2026-05-12).

> **Two MuxMaster Pooled numbers appear on this page.** The per-route table reports **49.6 ns** (`bench_test.go`, the internal micro-benchmark across all seven route categories). The competitor table reports **45 ns** (`competitor/bench_test.go`, the apples-to-apples harness registering the same route set on every router). Use **45 ns** when comparing MuxMaster against other routers; use **49.6 ns** when comparing across MuxMaster route categories.

### Per route category — v1.0.1 vs v1.1.0 default vs v1.1.0 Pooled

| Case                       | v1.0.1                   | v1.1.0 default           | v1.1.0 Pooled                  |
|----------------------------|--------------------------|--------------------------|--------------------------------|
| Static route               | 27 ns / 0 B              | 25.1 ns / 0 B            | 25.1 ns / 0 B                  |
| 1-parameter route          | 110 ns / 416 B / 1 alloc | 105 ns / 384 B / 1 alloc | **49.6 ns / 0 B / 0 allocs**   |
| 2-parameter route          | 124 ns / 448 B / 1 alloc | 119 ns / 416 B / 1 alloc | **55.9 ns / 0 B / 0 allocs**   |
| 3-parameter route          | 138 ns / 480 B / 1 alloc | 135 ns / 480 B / 1 alloc | **58.6 ns / 0 B / 0 allocs**   |
| Catch-all                  | 112 ns / 384 B / 1 alloc | 108 ns / 384 B / 1 alloc | **43.9 ns / 0 B / 0 allocs**   |
| Parallel 1-parameter route | 105 ns / 384 B / 1 alloc | 100 ns / 384 B / 1 alloc | **6.3 ns / 0 B / 0 allocs**    |
| Fast 1-parameter route     | 51 ns / 32 B / 1 alloc   | 50.3 ns / 32 B / 1 alloc | n/a (Fast path uses `Params`)  |

The `Pooled` column requires the opt-in `mux.PoolRequestBundle = true`. The lifetime contract — handlers must not retain `*http.Request` past return — is documented in [`/docs/max-performance`](/docs/max-performance).

### Versus every other Go HTTP router (1-parameter route)

| Router                          | ns/op     | B/op  | allocs/op | Notes                                                                |
|---------------------------------|-----------|-------|-----------|----------------------------------------------------------------------|
| **MuxMaster Pooled (Opt O13)**  | **45**    | **0** | **0**     | Opt-in `PoolRequestBundle = true`. Only `net/http`-compatible router at 0 allocs. |
| MuxMaster Fast                  | 50        | 32    | 1         | `HandleFast` — bypasses the `net/http` chain for trusted internal routes. |
| MuxMaster default               | 108       | 384   | 1         | Default `http.Handler` path, no opt-ins.                              |
| httprouter                      | 56        | 64    | 1         | Different handler signature; not a `net/http` drop-in.                |
| Fiber v3                        | 212       | 0     | 0         | `fasthttp` stack — not `net/http` compatible.                         |
| bunrouter                       | 183       | 192   | 3         | `httpRouter` adapter wrapping a vendored fork.                        |
| chi v5                          | 354       | 304   | 4         | `net/http` compatible, regex-aware.                                   |
| gorilla/mux                     | 3 444 278 | n/a   | 156 015   | Regex-walk router — included for completeness.                        |

MuxMaster is the fastest Go HTTP router across every route category measured. With `PoolRequestBundle` it is the only stdlib-compatible router with zero allocations on parameterised routes in the entire Go ecosystem. The full per-category report, including 2- and 3-parameter routes, catch-alls, and parallel dispatch, is at [`reports/perf-audit-2026-05-12/2026-05-12-competitor-showdown.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/reports/perf-audit-2026-05-12/2026-05-12-competitor-showdown.md).

## Quick links

- [Getting started](/docs/getting-started)
- [API reference](/api)
- [Benchmarks](/benchmarks)
- [Maximum performance guide](/docs/max-performance)
- [Examples](/examples/)
- [Source on GitHub](https://github.com/FlavioCFOliveira/MuxMaster)

Released as v1.1.0. MIT-licensed. Requires Go 1.26+.

## Frequently asked questions

<section data-conversation="landing-faq">

### What is MuxMaster?

MuxMaster is a high-performance, zero-dependency HTTP router for Go. It implements a radix tree with O(k) lookups, allocates zero bytes on the static-route hot path (and, with the opt-in `PoolRequestBundle`, on parameterised routes too), and is fully compatible with the `net/http` `Handler` interface so existing handlers compile unchanged.

### What Go version does MuxMaster require?

Go 1.26 or later. The minimum is set by the `go` directive in the upstream `go.mod`; see [Compatibility](/compatibility) for the full version policy.

### Is MuxMaster compatible with `net/http`?

100 %. Handlers remain `http.Handler` and middleware remains `func(http.Handler) http.Handler`. A `*muxmaster.Mux` is a drop-in replacement for `http.ServeMux` everywhere the standard library accepts an `http.Handler`, so adoption is incremental: a single route, a single sub-tree, or a whole service.

### How fast is MuxMaster?

Static-route lookups complete in 25 ns with zero allocations. One-parameter routes complete in 49.6 ns with **zero allocations** when the opt-in `mux.PoolRequestBundle = true` is enabled, or 105 ns / 1 alloc on the default path. Numbers measured on AMD Ryzen 9 5900HX, Go 1.26.2; the competitive 1-parameter measurement (45 ns, +20 % over `httprouter`) is reproduced from the upstream competitor showdown. See the [performance tables above](#performance) and [Benchmarks](/benchmarks) for the full data and reproduce instructions.

### What is the catch with `PoolRequestBundle`?

`PoolRequestBundle` recycles the per-request bundle via tiered `sync.Pool`s. The contract is strict: handlers must not retain `*http.Request` past return — for instance, never capture `r` in a goroutine that outlives the handler. The full contract, the failure modes, and the only safe goroutine pattern (drain before spawn) are documented in [`/docs/max-performance`](/docs/max-performance).

### What is MuxMaster's license?

MIT. The full text is in the upstream repository at [LICENSE](https://github.com/FlavioCFOliveira/MuxMaster/blob/main/LICENSE). MIT permits commercial use, modification, distribution, and private use; the only requirement is preserving the copyright notice.

### How does MuxMaster compare to chi, gin, gorilla/mux, and httprouter?

MuxMaster keeps the `net/http` handler signature (unlike `gin`, which introduces `gin.Context`) and ships zero external dependencies (unlike `chi`, which depends on `go-chi` sub-packages, or `gorilla/mux`, which depends on `gorilla/context`). On a 1-parameter route it is **20 % faster than `httprouter`** at the same handler signature MuxMaster preserves; **7.9 × faster than `chi v5`**; **76 000 × faster than `gorilla/mux`**. See the [competitor table above](#versus-every-other-go-http-router-1-parameter-route) and the [migration guide](/docs/migration) for side-by-side equivalents.

</section>
