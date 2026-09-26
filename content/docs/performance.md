---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# Performance

MuxMaster is a Go HTTP router built to add as little as possible to the standard `net/http` stack: zero allocations on static routes, one fused allocation on parameterised routes by default, and zero allocations with the opt-in pools. This document explains how the router achieves that, what the measurements show, and how to reproduce them.

Every figure on this page comes from this website's benchmark campaign of 2026-09-26, which measured MuxMaster v1.3.0 against v1.1.0 and against other Go routers with `benchstat` significance tests, unless the figure is explicitly labelled as an upstream measurement. The complete tables, the method, and the caveats are on the [Benchmarks](/benchmarks) page.

For configuring the zero-allocation mode and its handler lifetime contract, see the **[Maximum Performance Guide](/docs/max-performance)**.

## Table of Contents

- [Design Goals](#design-goals)
- [How Allocations Are Minimised](#how-allocations-are-minimised)
- [Measured Results (v1.3.0)](#measured-results-v130)
- [Changes Since v1.1.0](#changes-since-v110)
- [Historical Results](#historical-results)
- [Running Benchmarks Locally](#running-benchmarks-locally)
- [What Affects Performance](#what-affects-performance)
- [Comparison Notes](#comparison-notes)

---

## Design Goals

1. **Zero allocations on static routes; one fused, size-class-aligned allocation on parameterised `Handle` routes** — 384, 416 or 480 B for 1, 2 or 3+ parameters. `HandleFast` routes allocate only the parameter slice (32, 64 or 96 B).
2. **Zero allocations on parameterised routes when the application accepts a stricter lifetime contract** — the opt-in `PoolRequestBundle` and `PoolFastParams` pools.
3. **Lock-free request dispatch** — no mutex on the steady-state request path.
4. **Strict `net/http` compatibility** — `*Mux` is an `http.Handler`; handlers keep the standard signature, and the default mode lets handlers retain `*http.Request` indefinitely.

---

## How Allocations Are Minimised

### Radix tree

Routes are stored in a radix (compressed prefix) tree, one per HTTP method. Lookup cost is O(k) in the path length, not O(n) in the number of routes. The trees are published through `treesPtr atomic.Pointer[methodTrees]`: requests load the pointer without a lock, and registration builds a copy-on-write replacement under a mutex. Registration copies only the nodes on the path being modified, so its cost grows with the depth of the new route, not the size of the tree.

When a static branch fails further down the path, the lookup backtracks to a parameter sibling. Backtracking is bounded: its cost grows linearly with path depth (see [What Affects Performance](#route-tree-shape-and-depth)).

### Stack-allocated parameter buffer

During lookup, path parameters are written into a fixed-size `paramsBuf` on the stack. A static route passes the original `*http.Request` straight to the handler, so it allocates nothing.

### Tiered request bundle

For a `Handle` route with parameters, MuxMaster copies `*http.Request` and fuses the copy with the parameter context (`requestCtx`) into one struct — the tiered `reqBundle`:

| Parameters | Bundle type  | Struct size | GC size class |
|------------|--------------|-------------|---------------|
| 1          | `reqBundle1` | 368 B       | 384 B         |
| 2          | `reqBundle2` | 400 B       | 416 B         |
| 3+         | `reqBundle`  | 456 B       | 480 B         |

More than three parameters add a separate overflow slice. Opt O12 (v1.1.0) removed a redundant `params Params` field from `requestCtx1` and `requestCtx2`; the slice is now derived from `small[:N]`, which moved `reqBundle1` from the 416 B to the 384 B size class and `reqBundle2` from 448 B to 416 B.

The copy's context is set with `setReqCtxUnsafe`, an `unsafe.Add` write at the reflected offset of the private `ctx` field of `http.Request`. This is safe because the bundle is not visible to any other goroutine until after the write, and the original `r` is never modified. If a future Go release renames or removes that field, the router detects it at start-up (`hasReqCtxField`) and falls back to `r.WithContext`, which costs a second allocation.

A request whose internal context is `nil` — for example a struct literal passed to `ServeHTTP` in a test — is dispatched with `context.Background()` as the parent context.

### FastHandler routes

`HandleFast` passes the parameters as a third argument instead of through the request context, so it does not copy the request. By default it allocates only the `Params` slice (32–96 B for 1–3 parameters).

### Opt-in pools (Opt O13 / O9)

| Field | Recycles | Lifetime contract |
|---|---|---|
| `Mux.PoolRequestBundle` | The `reqBundle` of `Handle` routes (three tiers) | Handlers must not retain `*http.Request` after returning |
| `Mux.PoolFastParams` | The `Params` slice of `HandleFast` routes with 1–3 parameters | Handlers must not retain `ps` after returning |

Both default to `false`. A pooled bundle is zeroed (`*b = reqBundle1{}`) before it returns to the pool, so a later request never sees stale fields. Zeroing does not make a retained reference safe: a goroutine that keeps `r` sees a zeroed or reissued bundle. The pooled path is used only when `hasReqCtxField` is true. See the [Maximum Performance Guide](/docs/max-performance).

### Middleware applied at registration time

`Use` middleware is applied when a route is registered (`wrapMiddleware`), and the tree stores the wrapped handler. At request time the router makes one call; there is no chain to iterate. Consequently `Use` must be called before the routes it should wrap. `Pre` middleware is wrapped once around the dispatcher and runs on every request.

### Method dispatch via array index

`methodIdx` maps the method string to an array index with a `switch` over the ten supported methods (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, CONNECT, TRACE, QUERY) plus the internal `"*"` token used by `Mount`; the tree root is then an array access. Unsupported methods are rejected at registration.

### Frozen configuration snapshot

On the first `ServeHTTP` call, the option fields (`RedirectTrailingSlash`, `RedirectFixedPath`, `HandleMethodNotAllowed`, `HandleOPTIONS`, `CaseInsensitive`, `UseRawPath`, `UnescapePathValues`, `RedirectCode`, `PoolFastParams`, `PoolRequestBundle`) and the handler fields (`NotFound`, `MethodNotAllowed`, `GlobalOPTIONS`, `ErrorHandler`, `PanicHandler`) are copied into a `muxConfig` snapshot. Requests read the snapshot through one atomic pointer load. `Rebuild()` discards it; the next request takes a new one.

### Cached error, OPTIONS and redirect handlers

The middleware-wrapped 405 and automatic-OPTIONS handlers are built once per `Allow` value and cached; the `Allow` string itself comes from a precomputed table indexed by a method bitmask. Redirects use a cached middleware-wrapped handler and read the `Use` middleware from an atomic snapshot, so no redirect takes a lock.

---

## Measured Results (v1.3.0)

**Host and method:** AMD Ryzen 9 5900HX (8 cores, 16 threads, governor `performance`), Ubuntu 24.04.5 LTS, Linux 6.8.0-139, go1.27.1 with `GOTOOLCHAIN=local`, 2026-09-26. Each benchmark has 10 samples (`-count=10` equivalent: 10 rounds of `-count=1`, each in a separate process). Values are medians. Router comparisons use `benchstat` with the Mann-Whitney U test at alpha = 0.05. Source: the [Benchmarks](/benchmarks) page and its campaign archive.

### Routing versus other routers

Upstream `competitor/` suite run against MuxMaster v1.3.0; ns/op, with allocs/op in parentheses. "Pooled" is `PoolRequestBundle = true`; "Fast" is `HandleFast` without `PoolFastParams`. Routers: httprouter v1.3.0, bunrouter v1.0.23 (through its `http.Handler` adapter), chi v5.3.2, gorilla/mux v1.8.1. Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

| Route type        | MuxMaster default | MuxMaster Pooled | MuxMaster Fast | httprouter | bunrouter¹ | chi v5    | gorilla/mux |
|-------------------|-------------------|------------------|----------------|------------|------------|-----------|-------------|
| Static            | 28.04 (0)         | 29.11 (0)        | 29.10 (0)      | 34.67 (0)  | 162.8 (3)  | 225.2 (2) | 576.6 (7)   |
| 1 parameter       | 115.0 (1)         | 46.70 (0)        | 45.55 (1)      | 49.67 (1)  | 160.5 (3)  | 368.9 (4) | 954.0 (8)   |
| 2 parameters      | 132.2 (1)         | 59.41 (0)        | 63.42 (1)      | 59.53 (1)  | 179.5 (3)  | 413.7 (4) | 1 492 (8)   |
| 3 parameters      | 143.6 (1)         | 65.17 (0)        | 83.31 (1)      | 75.37 (1)  | 180.2 (3)  | 428.0 (4) | 1 703 (8)   |
| Catch-all         | 114.4 (1)         | 45.71 (0)        | 45.61 (1)      | 42.92 (1)  | 155.4 (3)  | 345.0 (4) | 1 589 (8)   |
| Not found         | 257.3 (3)         | —²               | —²             | 383.2 (3)  | 285.6 (4)  | 354.4 (5) | 1 030 (4)   |
| Parallel static   | 4.121 (0)         | —²               | 4.183 (0)      | 4.902 (0)  | 129.0 (3)  | 134.7 (2) | 353.1 (7)   |
| Parallel 1 param  | 104.9 (1)         | 6.854 (0)        | 16.24 (1)      | 22.07 (1)  | 129.3 (3)  | 238.5 (4) | 469.2 (8)   |

¹ bunrouter measured through its `http.Handler` adapter only. Its native `bunrouter.HandlerFunc` API is not `net/http`-compatible and was not measured.
² Not measured: the suite has no benchmark for that mode and category.

Bytes per operation for the parameterised cases: MuxMaster default 384 / 416 / 480 B, MuxMaster Fast 32 / 64 / 96 B, MuxMaster Pooled 0 B, httprouter 64 / 64 / 96 B (32 B for the catch-all).

### Root package (`bench_test.go`)

MuxMaster v1.3.0, root package benchmarks; median ns/op of 10 samples. Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| StaticRoute | 27.20 | 0 | 0 |
| ParamRoute1 / 2 / 3 | 115.9 / 126.6 / 143.4 | 384 / 416 / 480 | 1 |
| WildcardRoute (catch-all) | 115.2 | 384 | 1 |
| NotFound | 218.4 | 99 | 3 |
| ParallelStaticRoute | 3.928 | 0 | 0 |
| ParallelParamRoute | 105.0 | 384 | 1 |
| FastStaticRoute | 27.62 | 0 | 0 |
| FastParamRoute1 / 2 / 3 | 44.34 / 62.30 / 76.63 | 32 / 64 / 96 | 1 |
| FastParallelParamRoute | 15.40 | 32 | 1 |
| PooledParamRoute1 / 2 / 3 | 45.84 / 55.58 / 59.58 | 0 | 0 |
| PooledWildcardRoute | 45.48 | 0 | 0 |
| PooledParallelParamRoute | 6.585 | 0 | 0 |
| Mount_Static / Mount_Param / Mount_TSRRedirect | 346.6 / 476.1 / 559.0 | 864 / 1 248 / 920 | 2 / 3 / 6 |
| ServeFiles / GroupServeFiles | 791.9 / 791.2 | 708 | 8 |
| RegisterRoutes N=100 / 1 000 / 5 000 (per route) | 546.4 / 680.0 / 918.6 ns | — | — |

`bench_test.go` has no benchmark for `PoolFastParams`.

### Middleware (`middleware/bench_test.go`)

MuxMaster v1.3.0, `middleware` package benchmarks; median of 10 samples. Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| ThrottlePerIP, one client / many clients | 115.4 / 121.3 | 0 | 0 |
| Logger | 5 664 | 0 | 0 |
| Recoverer / RecovererWithLogger, no panic | 20.82 / 20.58 | 0 | 0 |
| Compress, 600 B / chunked 12 KiB | 317.2 / 7 926 | 32 / 58 | 2 / 3 |
| RealIP, 1 hop / 3 hops | 165.3 / 201.2 | 16 | 1 |
| APIKey, hit | 450.2 | 416 | 6 |
| BasicAuth hit, 1 / 10 / 100 users | 314.4 / 547.5 / 2 866 | 32 | 2 |
| BasicAuth miss, 1 / 10 / 100 users | 700.5 / 929.1 / 3 235 | 168 | 8 |
| JWTAuth, HS256 | 4 384 | 738 | 7 |
| OAuth2Introspect cache insert at saturation | 3 208 | 288 | 2 |

---

## Changes Since v1.1.0

The campaign ran the same benchmarks against the v1.1.0 and v1.3.0 tags, interleaved in 10 rounds on the same host with the same go1.27.1 compiler. A delta is reported only when `benchstat` finds it significant at alpha = 0.05; an A/A control run of v1.3.0 against itself found no significant difference in any of 40 rows.

### Request hot path

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

- **Allocations are unchanged.** B/op and allocs/op are identical in 17 of the 18 root-package benchmarks shared by both versions. The exception is `NotFound`, whose B/op fell from 101 to 99; its allocs/op are identical.
- **7 of the 18 benchmarks show no significant difference in time**, including `ParamRoute2`, `ParamRoute3`, `ParallelParamRoute`, and `PooledParamRoute2`.
- **3 are faster in v1.3.0:** `NotFound` −2.19%, `FastParamRoute2` −9.06%, and `FastParamRoute3` −11.55%.
- **8 are slower in v1.3.0, by 1.42% to 6.25%:** `StaticRoute` (+4.62%, 26.00 → 27.20 ns), `ParamRoute1` (+4.65%), `WildcardRoute` (+4.54%), `ParallelStaticRoute` (+3.67%), `FastStaticRoute` (+6.25%), `FastParallelParamRoute` (+1.42%), `PooledParamRoute1` (+2.83%, 44.58 → 45.84 ns), and `PooledParallelParamRoute` (+2.12%). The campaign did not establish the cause of these regressions and did not separate a code change from a change in code layout.

### Gains measured since v1.1.0

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

| Change | v1.1.0 | v1.3.0 | Change |
|---|---|---|---|
| `ThrottlePerIP`, one client | 4.465 µs, 5 allocs | 118.1 ns, 0 allocs | −97.35% |
| `RequestID`, generate | 1.161 µs, 9 allocs | 293.9 ns, 4 allocs | −74.67% |
| `ThrottleBacklog`, no wait | 31.72 ns | 10.70 ns | −66.26% |
| `Compress`, large body | 830.6 ns, 2 allocs | 299.7 ns, 0 allocs | −63.92% |
| `JWTAuth` HS256, invalid token | 1.240 µs, 10 allocs | 617.6 ns, 7 allocs | −50.18% |
| Automatic `OPTIONS` | 120.5 ns, 3 allocs | 68.72 ns, 1 alloc | −42.95% |
| `StripSlashes`, dirty path | 192.1 ns, 3 allocs | 125.5 ns, 2 allocs | −34.67% |
| `CleanPath`, dirty path | 278.2 ns, 5 allocs | 196.1 ns, 4 allocs | −29.53% |
| `Compress`, small body | 371.4 ns, 2 allocs | 275.5 ns, 0 allocs | −25.81% |
| Trailing-slash redirect | 865.2 ns, 13 allocs | 660.9 ns, 10 allocs | −23.61% |
| `Logger` | 7.261 µs, 4 allocs | 5.665 µs, 0 allocs | −21.98% |
| 405 Method Not Allowed | 153.2 ns, 2 allocs | 124.2 ns, 1 alloc | −18.96% |
| `JWTAuth` HS256, valid token | 4.662 µs, 10 allocs | 4.122 µs, 7 allocs | −11.59% |

Every change in this table is significant with p < 0.0005 (perf-audit harness under `reports/perf-audit-2026-05-12/`, identical benchmark bodies at both tags). Route registration and `Mount` have no benchmark at v1.1.0, so their change since v1.1.0 could not be measured; their v1.3.0 values are in the root-package table above.

### Costs added by behaviour changes

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

| Change of behaviour | v1.1.0 | v1.3.0 | Change |
|---|---|---|---|
| `CORS` always adds `Vary: Origin`; request without `Origin` | 20.24 ns, 0 allocs | 81.79 ns, 1 alloc (112 B) | +304.08% |
| `Recoverer` tracks whether the response has started; no panic | 6.502 ns | 17.46 ns | +168.59% |
| `BasicAuth` constant-time scan over all users; valid credentials | 199.9 ns | 327.6 ns | +63.92% |
| `BasicAuth`; invalid credentials | 520.0 ns | 658.7 ns | +26.66% |

Each change is significant with p < 0.0005. `BasicAuth` now costs more as users are added: the v1.3.0 middleware table above shows a successful check at 314.4 ns with 1 user, 547.5 ns with 10, and 2 866 ns with 100. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

### Upstream measurements not repeated by the campaign

The following figures are upstream measurements from the sprint 18 contention hunt, taken on 2026-09-24 on an AMD Ryzen 9 5900HX with Go 1.27.0, `b.RunParallel`, and `-cpu 1,4,16`, before the v1.2.0 tag. Every row used `-count=6`, except the `OAuth2Introspect` eviction row, whose `-count` is not recorded upstream. Source: [`reports/perf-lab-2026-09-24/contention-hunt.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/reports/perf-lab-2026-09-24/contention-hunt.md) (upstream MuxMaster, 2026-09-24). The "before" values were measured against pre-change commits, not against v1.1.0.

| Change | Before | After |
|---|---|---|
| `ThrottlePerIP` table sharded 64 ways, many clients, 16 CPUs | 2 114 ns | 451 ns (4.68×); no longer slows down above 4 CPUs |
| `ThrottleBacklog` lock-free CAS fast path, 1 CPU / 16 CPUs | 39.12 ns / 76.15 ns | 22.76 ns (−42%) / 66.23 ns (−13%) |
| `RequestID` batched `crypto/rand` and fused allocation | 7 allocs; 1 092 / 291.4 / 253.1 ns at 1 / 4 / 16 CPUs | 2 allocs; 231.5 / 102.0 / 131.6 ns (~4.7× / 2.8× / 1.95×) |
| `OAuth2Introspect` eviction at a full cache (min-heap; `-count` not recorded upstream) | 247.7 / 257.2 / 257.4 µs at 1 / 4 / 16 CPUs | 3.27 / 1.53 / 2.15 µs |
| Redirects read the `Use` middleware snapshot lock-free | `RWMutex` read lock per redirect | no lock; no measurable ns/op change |

### High-concurrency scaling (upstream, 2026-09-24)

Upstream measurement: `b.RunParallel` with `-cpu 1,4,16`, `-count=6`, Go 1.27.0, AMD Ryzen 9 5900HX, 2026-09-24. Source: [contention hunt](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/reports/perf-lab-2026-09-24/contention-hunt.md) (upstream MuxMaster, 2026-09-24).

| Benchmark | cpu=1 | cpu=4 | cpu=16 |
|---|---:|---:|---:|
| 1 parameter, `Handle` default (1 alloc) | 143.2 ns | 94.7 ns | 104.4 ns |
| 1 parameter, `Handle` + `PoolRequestBundle` (0 allocs) | 41.5 ns | 11.0 ns | 7.3 ns |
| 1 parameter, `HandleFast` default (1 alloc) | 49.0 ns | 13.2 ns | 16.0 ns |
| 1 parameter, `HandleFast` + `PoolFastParams` (0 allocs) | 39.3 ns | 10.3 ns | 5.0 ns |

The allocating variants stop improving, and slightly regress, beyond 4 CPUs because of allocator and GC contention; the pooled variants keep scaling. This is the most recent measurement of `PoolFastParams`; neither the campaign nor the upstream 2026-09-26 run has a pooled fast-route benchmark.

---

## Historical Results

Figures measured on v1.1.0-era code on other hardware (Apple M4 and Raspberry Pi 5, 2026-05-12) are published only in the [Historical (v1.1.0-era code)](/benchmarks#historical-v110-era-code) section of the Benchmarks page. They were not re-measured on v1.3.0 and describe hardware differences, not the current release.

---

## Running Benchmarks Locally

```bash
# Root and middleware packages (excludes reports/ and competitor/)
make bench

# The same, three samples, for benchstat
go test -run='^$' -bench=. -benchmem -count=3 . ./middleware/ | tee results.txt
benchstat results.txt

# Competitor suite (separate module with vendored dependencies)
cd competitor && go test -mod=mod -run='^$' -bench=. -benchmem -count=3 .
```

`go test -bench=. ./...` from the repository root also runs the test harnesses under `reports/`, which belong to the same module.

To compare before and after a change, use at least `-count=6` so `benchstat` can report confidence intervals, and pin the CPU governor to `performance` if you can:

```bash
go test -run='^$' -bench=. -benchmem -count=10 . > before.txt
# make your change
go test -run='^$' -bench=. -benchmem -count=10 . > after.txt
benchstat before.txt after.txt
```

---

## What Affects Performance

### Number of path parameters

In the default mode each extra parameter moves the request into a larger bundle tier and adds lookup work: 1 → 3 parameters measured 115.9 → 143.4 ns (384 → 480 B) in the campaign. With `PoolRequestBundle` the same range is 45.84 → 59.58 ns with no allocation. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

### Regex-constrained parameters

A regex parameter is compiled at registration and evaluated against the candidate segment during lookup. Its cost depends on the expression; the benchmark suites contain no regex-route benchmark, so measure your own patterns.

### Middleware

`Use` middleware adds no routing overhead because it is applied at registration; each request pays only for what the middleware itself does. The [middleware table](#middleware-middlewarebench-testgo) shows those costs, from about 21 ns (`Recoverer`, no panic) to several microseconds (`Logger`, `JWTAuth`). Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

### Route tree shape and depth

Lookup cost depends on the length of the path and on how much backtracking the tree forces, not on the total number of routes. In the adversarial benchmarks, cost and allocations grow linearly with depth: `AdversarialBacktracking` goes from 160.3 ns at depth 1 to 15.13 µs at depth 128, and `QuadraticBacktracking` from 192.2 ns at depth 2 to 9.232 µs at depth 64. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

### Registration

Registration copies only the nodes along the new route's path. Measured per route: 546.4 ns with 100 routes, 680.0 ns with 1 000, 918.6 ns with 5 000. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

---

## Comparison Notes

All figures in this section are from the [routing table above](#routing-versus-other-routers). A comparison is stated as faster or slower only when `benchstat` finds it significant at alpha = 0.05, and it applies only to the routers, modes, and route categories measured.

### Where MuxMaster was the fastest measured router

MuxMaster was the fastest of the five measured routers in six of the eight categories: static routes (default mode, 28.04 ns), 1 parameter (Fast mode, 45.55 ns), 3 parameters (Pooled mode, 65.17 ns), not found (default mode, 257.3 ns), parallel static (default mode, 4.121 ns), and parallel 1 parameter (Pooled mode, 6.854 ns). In each of these categories the fastest non-MuxMaster router was slower with p < 0.001. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

### vs httprouter

httprouter is the usual performance reference, and it is the router that beats MuxMaster in some categories.

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: the campaign archive published on [Benchmarks](/benchmarks).

- **Catch-all:** httprouter was fastest (42.92 ns). Every MuxMaster mode was slower: Fast 45.61 ns, Pooled 45.71 ns, default 114.4 ns.
- **2 parameters:** Pooled MuxMaster (59.41 ns) and httprouter (59.53 ns) showed no significant difference (p = 0.668).
- **Default mode on parameterised routes:** MuxMaster's default mode is slower than httprouter on every parameterised category: 115.0 vs 49.67 ns on 1 parameter, 132.2 vs 59.53 ns on 2, 143.6 vs 75.37 ns on 3, 114.4 vs 42.92 ns on catch-all, and 104.9 vs 22.07 ns on the parallel parameter benchmark. The default mode copies `*http.Request` into a 384–480 B bundle so handlers keep the standard signature and may retain `r`; httprouter allocates only its `Params` slice (32–96 B) and uses a three-argument handler.
- **Fast mode:** `HandleFast` without pooling is faster than httprouter on static, 1 parameter, parallel static, and parallel 1 parameter, and slower on 2 parameters (63.42 vs 59.53 ns), 3 parameters (83.31 vs 75.37 ns), and catch-all.
- **Pooled mode:** with `PoolRequestBundle`, MuxMaster allocates nothing on parameterised routes and is faster than httprouter on static (29.11 vs 34.67 ns), 1 parameter (46.70 vs 49.67 ns), 3 parameters (65.17 vs 75.37 ns), and parallel 1 parameter (6.854 vs 22.07 ns), level on 2 parameters, and slower on catch-all (45.71 vs 42.92 ns).

MuxMaster is faster than httprouter on not-found in the default mode (257.3 vs 383.2 ns). Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

### vs bunrouter, chi, and gorilla/mux

Each MuxMaster mode was faster than bunrouter (through its `http.Handler` adapter), chi v5, and gorilla/mux in every category where that mode was measured, with p < 0.001 for each comparison. For example, on 1 parameter MuxMaster's default mode took 115.0 ns against 160.5 ns for bunrouter, 368.9 ns for chi, and 954.0 ns for gorilla/mux (measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks)). bunrouter's native API, which allocates nothing and does not use the `net/http` handler signature, was not measured; no claim on this page applies to it. See the [Migration Guide](/docs/migration) for moving from these routers.

---

## See Also

- [Benchmarks](/benchmarks) — the full campaign tables, method, caveats, and historical data
- [Maximum Performance Guide](/docs/max-performance) — the zero-allocation configuration and its lifetime contract
- [Migration Guide](/docs/migration) — replacing httprouter, chi, or gorilla/mux
- [Routing](/docs/routing) — how the radix tree resolves patterns
- [Security](/security) — the concurrency analysis behind the lifetime contracts

## Common questions

<section data-conversation="performance-allocations">

### How many allocations does MuxMaster make per request?

MuxMaster makes zero allocations on static routes and one allocation on parameterised `Handle` routes in its default configuration.

The single allocation is a request bundle of 384, 416, or 480 B for 1, 2, or 3+ parameters. `HandleFast` routes allocate only the `Params` slice (32, 64, or 96 B).

### Can MuxMaster dispatch parameterised routes without allocating?

Yes: setting `Mux.PoolRequestBundle = true` makes parameterised `Handle` routes allocate nothing, and `Mux.PoolFastParams = true` does the same for `HandleFast` routes.

Both options require that handlers never retain the request (or the `Params` slice) after returning; see the [Maximum Performance Guide](/docs/max-performance).

### Is MuxMaster v1.3.0 faster than v1.1.0?

MuxMaster v1.3.0 is not uniformly faster than v1.1.0: its request hot path allocates the same, several router and middleware paths are much faster, and some hot-path benchmarks are slightly slower.

In the campaign, 3 of 18 shared root-package benchmarks were faster in v1.3.0, 7 showed no significant difference, and 8 were slower by 1.42% to 6.25%. Middleware such as `ThrottlePerIP`, `Logger`, `Compress`, and `RequestID` became much faster, while behaviour changes made `CORS` without an `Origin` header, `Recoverer`, and `BasicAuth` slower. The [Changes Since v1.1.0](#changes-since-v110) section lists every figure. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive published on [Benchmarks](/benchmarks).

</section>

## Upstream source

This page is based on [`docs/performance.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/docs/performance.md) at the v1.3.0 tag. The design sections are mirrored from it; the measured figures were replaced with this website's benchmark campaign, as described at the top of the page. The mechanisms are implemented in [`mux.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/mux.go), [`tree.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/tree.go), and [`params.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/params.go).
