---
datePublished: 2026-05-12
---

# Performance

MuxMaster is designed to add negligible overhead to the standard `net/http` stack. This document explains the design decisions behind its performance, how to measure it, and how it compares to other Go HTTP routers. For the **zero-allocation hot path** introduced in v1.1.0 — the opt-in `PoolRequestBundle` and `PoolFastParams` flags — see the dedicated [Maximum performance](/docs/max-performance) guide.

## Table of Contents

- [Design Goals](#design-goals)
- [How Allocations Are Minimised](#how-allocations-are-minimised)
- [Benchmarks](#benchmarks)
- [Maximum performance (opt-in)](#maximum-performance-opt-in)
- [Running Benchmarks Locally](#running-benchmarks-locally)
- [What Affects Performance](#what-affects-performance)
- [Comparison Notes](#comparison-notes)

---

## Design Goals

1. **Zero allocations for static routes; one fused tiered allocation for parameterised routes (`Handle`)** — the parameter bundle is sized to match the GC size class (384 / 416 / 480 B for 1 / 2 / 3 parameters in v1.1.0), and `HandleFast` further reduces the allocation footprint to 32–96 B for the same parameter counts. The opt-in `Mux.PoolRequestBundle = true` recycles the bundle through tiered `sync.Pool`s and reaches **zero allocations on parameterised routes too** — see [Maximum performance](/docs/max-performance).
2. **Sub-microsecond dispatch** — route lookup completes in tens of nanoseconds, not hundreds.
3. **Linear scalability** — throughput per core scales linearly with the number of CPUs (~4 200 RPS per vCPU on a 16-core box at 1 000 concurrent goroutines).
4. **Strict `net/http` compatibility** — no fasthttp; no breaking surface; the fused allocation is the safest design that avoids the race conditions detected on the previous experimental zero-alloc approach (CSA-001).

---

## How Allocations Are Minimised

MuxMaster delivers **zero allocations on static routes** and a **single tiered allocation on parameterised routes** in the `Handle` path. The `HandleFast` path uses a smaller exact-sized allocation. Both paths achieve O(k) lookup and lock-free reads through the same set of techniques.

### Radix tree

Routes are stored in a radix (compressed prefix) tree — one tree per HTTP method. Lookup is O(k) in the path length, not O(n) in the number of routes. The tree is built at startup and never mutated during request processing, so no locks are needed on the read path. The active method-trees pointer is loaded lock-free via `treesPtr atomic.Pointer[methodTrees]`; registration uses a copy-on-write swap under a writer mutex.

### Stack-allocated parameter buffer

During tree traversal, path parameters are written into a fixed-size `paramsBuf` struct allocated on the stack inside `getValue`. There is no `sync.Pool` — the buffer never escapes the goroutine, so the GC never sees it. For static routes (no parameters), nothing further is allocated and the static-route allocation count is zero.

### Tiered request bundle

For routes with parameters, MuxMaster fuses the request context and the copy of `*http.Request` into a single GC-class-aligned struct — the tiered `reqBundle`:

| Parameters | Bundle type   | Size  | GC size class |
|------------|---------------|-------|---------------|
| 1          | `reqBundle1`  | 392 B | 416 B         |
| 2          | `reqBundle2`  | 424 B | 448 B         |
| 3+         | `reqBundle`   | 456 B | 480 B         |

Each tier is sized to the exact GC bucket so there is no internal fragmentation. The bundle's request-context field is set via `setReqCtxUnsafe` (an `unsafe.Add` over the reflected offset of the private `ctx` field of `http.Request`). This is safe because the bundle is freshly allocated and is not visible to any other goroutine until after the write; the original `r` is never mutated. If a future Go release moves the `ctx` field, the router automatically falls back to a 2-allocation `r.WithContext(ctx)` path through a runtime-detected `hasReqCtxField` flag — the previous, unsafe approach of mutating the original `r` was rejected after the `concurrency-security-auditor` confirmed CSA-001 race conditions.

For the `HandleFast` path (`FastHandler`), the allocation is even smaller: a 32–96 B exact-sized `Params` slice bounded by `maxParams = 3`.

### Middleware applied at registration time

Middleware is applied at **route registration** time via `wrapMiddleware`, not at request dispatch time. The router stores the fully-wrapped handler directly. At request time, the router calls a single function pointer — there is no middleware chain to iterate. This means `Use` must be called before the routes it should wrap.

### Method dispatch via array index

Standard HTTP methods (GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, CONNECT, TRACE) are mapped to array indices at compile time. Method dispatch during a request is an array access — O(1) and branch-free.

### Frozen configuration snapshot

On the first `ServeHTTP` call, the Mux flags (`RedirectTrailingSlash`, `RedirectFixedPath`, `HandleMethodNotAllowed`, `HandleOPTIONS`, `CaseInsensitive`, `UseRawPath`, `UnescapePathValues`, `RedirectCode`) are frozen into a `muxConfig` snapshot. Subsequent requests load the snapshot via a single atomic pointer instead of reading 6–8 struct fields. Tests that need to change Mux flags after first use call `Mux.Rebuild()` to reset the snapshot.

---

## Benchmarks

Measured on AMD Ryzen 9 5900HX (16 logical cores), Linux 6.8, Go 1.26.2. Numbers are consolidated from `-count=10 -benchtime=2s` runs via `benchstat` against the same route set at the v1.1.0 tag. The full evidence is archived under `reports/perf-audit-2026-05-12/`.

### Serial (single goroutine)

| Route type   | MuxMaster default `Handle` | MuxMaster `Handle` + Pool | MuxMaster `HandleFast`   | httprouter              |
|--------------|----------------------------|---------------------------|--------------------------|-------------------------|
| Static       | **25.1 ns, 0 allocs**      | **25.1 ns, 0 allocs**     | **25.1 ns, 0 allocs**    | 33.8 ns, 0 allocs       |
| 1 parameter  | 105 ns / 384 B / 1 alloc   | **49.6 ns / 0 B / 0 allocs** | **50.3 ns / 32 B / 1 alloc** | 56.4 ns / 64 B / 1 alloc |
| 2 parameters | 119 ns / 416 B / 1 alloc   | **55.9 ns / 0 B / 0 allocs** | 66.0 ns / 64 B / 1 alloc | 66.5 ns / 64 B / 1 alloc |
| 3 parameters | 135 ns / 480 B / 1 alloc   | **58.6 ns / 0 B / 0 allocs** | 74.2 ns / 96 B / 1 alloc | 78.4 ns / 64 B / 1 alloc |
| Catch-all    | 108 ns / 384 B / 1 alloc   | **43.9 ns / 0 B / 0 allocs** | —                        | 51.3 ns / 64 B / 1 alloc |

### Parallel (GOMAXPROCS cores)

| Route type   | MuxMaster default `Handle` | MuxMaster `Handle` + Pool | MuxMaster `HandleFast`   | httprouter              |
|--------------|----------------------------|---------------------------|--------------------------|-------------------------|
| Static       | **3.6 ns, 0 allocs**       | **3.6 ns, 0 allocs**      | **3.6 ns, 0 allocs**     | 4.9 ns, 0 allocs        |
| 1 parameter  | 100 ns / 384 B / 1 alloc   | **6.3 ns / 0 B / 0 allocs** | **17.1 ns / 32 B / 1 alloc** | 22.2 ns / 64 B / 1 alloc |

The Pooled parallel one-parameter benchmark (6.3 ns) is **3.5 × faster than `httprouter`**. Sustained-load testing with a four-middleware stack and 1 000 concurrent goroutines reaches **67 275 RPS at 0.00 % error rate** with a maximum GC pause of 2.95 ms (`reports/dos-resilience-tester/2026-05-08-production-loadtest.md`).

### Why the default path keeps one allocation

The single allocation on the default path is a **tiered `reqBundle`** (384 / 416 / 480 B for 1 / 2 / 3 parameters; sized after the v1.1.0 `params` field removal, [Opt O12](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/CHANGELOG.md)) that fuses the `requestCtx` and the copy of `*http.Request` into one GC-class-aligned object. This is a deliberate trade-off:

- `Handle` returns a 100 % `net/http`-compatible `http.Handler` chain. The single fused allocation is the safest available implementation against the previous race conditions detected by the `concurrency-security-auditor` (CSA-001) on the experimental zero-alloc design.
- `Handle` + `Mux.PoolRequestBundle = true` (v1.1.0, [Opt O13](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/CHANGELOG.md)) recycles the same bundle through tiered `sync.Pool`s, eliminating the allocation entirely under a strict handler-lifetime contract.
- `HandleFast` provides a fast-path `FastHandler` type that bypasses the standard wrapper and beats `httprouter` on every parameterised case while keeping a 1 alloc / 32–96 B footprint. With `Mux.PoolFastParams = true` (v1.1.0, Opt O9) the Fast path is zero-allocation too.

See [Maximum performance](/docs/max-performance) for the full opt-in guide, the lifetime contract, and the four real-world recipes.

## Maximum performance (opt-in)

For services whose handlers do not retain `*http.Request` past return, v1.1.0 ships two opt-in flags that recycle the per-request bundle through `sync.Pool`s, eliminating the routing allocation entirely.

```go
mux := muxmaster.New()
mux.PoolRequestBundle = true   // 0-alloc Handle path
mux.PoolFastParams    = true   // 0-alloc HandleFast path

mux.GET("/users/:id", getUser) // 45 ns / 0 B / 0 allocs
```

The contract is strict: handlers must not retain `*http.Request` (or the `Params` slice on `HandleFast`) past return — capturing `r` in a goroutine that outlives the handler is an unsafe-pool reuse. The dedicated [Maximum performance](/docs/max-performance) guide covers the contract in full, the four failure modes when it is broken, the audit checklist for an existing codebase, and the only safe pattern for spawning background work from a handler (drain before spawn).

---

## Running Benchmarks Locally

```
# All benchmarks with allocation counts
go test -bench=. -benchmem ./...

# Repeat 3 times and use benchstat for statistical comparison
go test -bench=. -benchmem -count=3 ./... | tee results.txt
benchstat results.txt
```

To compare before and after a code change:

```
go test -bench=. -benchmem -count=5 ./... > before.txt
# make your change
go test -bench=. -benchmem -count=5 ./... > after.txt
benchstat before.txt after.txt
```

---

## What Affects Performance

### Number of path parameters

Each additional parameter requires one extra comparison during tree traversal. This is linear and very fast — the difference between 1 and 3 parameters is approximately 20 ns.

### Regex-constrained parameters

Regex parameters compile the expression at startup and execute it during lookup. The overhead depends on the complexity of the pattern. A simple `[0-9]+` adds roughly 10–20 ns compared to an unconstrained `:name` parameter.

### Middleware

Middleware is applied at registration time, so it has no effect on the routing overhead itself. However, each middleware layer adds function-call overhead during the request. A chain of 5 middleware functions typically adds 50–200 ns depending on what they do.

### Route tree depth

Routes registered with longer paths require more tree traversal steps. In practice, paths are short enough that this is not measurable.

### Number of registered routes

Because the radix tree compresses shared prefixes, the number of routes has almost no effect on lookup time. A router with 1000 routes and a router with 10 routes perform identically on a given path.

---

## Comparison Notes

### vs httprouter

httprouter is the historical performance reference for Go HTTP routers. MuxMaster `Handle` beats httprouter on static routes and on `Not found`. The default `Handle` path (no opt-ins) trails httprouter on parameterised routes by ~2 × because `Handle` preserves a strict `net/http`-compatible chain (1 fused 384–480 B allocation per request). MuxMaster `HandleFast` removes the stdlib wrapper and beats httprouter on every parameterised case (50 ns vs 56 ns at 1 parameter). **With `Mux.PoolRequestBundle = true`, MuxMaster `Handle` beats httprouter by 20 % (45 ns vs 56 ns) and is the only `net/http`-compatible router with zero allocations on parameterised routes.**

### vs bunrouter

bunrouter claims zero allocations through **lazy parameter extraction** in its native API. The benchmarks in this repository measure bunrouter through the `HTTPHandlerFunc` adapter, which adds a `context.WithValue` and is therefore not representative of upstream native usage. In adapter mode, MuxMaster `Handle` is faster across the board; in native mode bunrouter is competitive, but parameter reads become O(n) per read instead of O(1).

### vs chi

chi uses a patricia radix trie and focuses on idiomatic API design over raw performance. MuxMaster `Handle` is approximately 8–25× faster in ns/op on the parameterised cases (115 ns vs 3 449 ns at 1 parameter) and allocates fewer bytes per request, while maintaining a compatible API surface.

### vs gorilla/mux

gorilla/mux uses regular-expression matching and was archived in 2022. It is typically 200–1 000× slower than MuxMaster for the same route set. MuxMaster is a drop-in replacement for the routing layer in gorilla/mux applications — see the [Migration Guide](migration.md).

---

## See Also

- [Maximum performance](/docs/max-performance) — the v1.1.0 zero-allocation hot path guide
- [Benchmarks](/benchmarks) — per-route and competitor tables
- [Migration Guide](/docs/migration) — replacing httprouter, chi, or gorilla/mux
- [Routing](/docs/routing) — how the radix tree resolves patterns

## Upstream source

The benchmark harness is in [`bench_test.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/bench_test.go) in the upstream repository; the competitor suite is [`competitor/bench_test.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/competitor/bench_test.go). Rerun with `go test -run=^$ -bench . -benchmem -count=10 -benchtime=2s` to reproduce the numbers cited above; raw output and the SYNTHESIS report are archived under [`reports/perf-audit-2026-05-12/`](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.1.0/reports/perf-audit-2026-05-12).

## Common questions

<section data-conversation="performance-patterns">

### How fast is MuxMaster compared with the standard library?

Static-route lookups are within a few percent of `net/http.ServeMux` and zero-allocation; parameterised routes allocate 384–480 B per request for the fused request bundle on the default path, or **zero bytes** when `Mux.PoolRequestBundle = true` is enabled. Lookup is O(k) over the path length k. The full numbers come from `bench_test.go` in the upstream repo at v1.1.0 and from the [benchmarks](/benchmarks) page on this site.

### Why are static routes zero-allocation?

The router resolves them entirely on the radix-tree path without constructing a parameter map (there are no parameters to capture). Once the leaf is reached the handler is dispatched directly; no per-request allocations beyond what `net/http` itself makes.

### How do I benchmark my own routing setup?

Copy `bench_test.go` from the upstream repository as a starting point and replace its routes with yours. Run with `go test -run=^$ -bench . -benchmem`; the `ns/op`, `B/op`, and `allocs/op` columns are the three metrics the spec considers normative.

</section>
