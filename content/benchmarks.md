---
datePublished: 2026-05-12
---

# Benchmarks

> This page reflects the upstream `CHANGELOG.md` `## [1.1.0]` section and the `reports/perf-audit-2026-05-12/` archive as of **2026-05-12** at upstream tag **`v1.1.0`**. Every number on this page is reproduced verbatim from those upstream sources. To re-run the suite, follow the [reproduce instructions](#reproduce) below.

All benchmarks were measured on AMD Ryzen 9 5900HX (16 logical CPUs), Linux 6.8.0-111, Go 1.26.2. Each row consolidates **10 × 2 s runs** via `benchstat`. The route set is identical across every router (10 static, 8 parameterised, 2 catch-all).

> **Two MuxMaster Pooled numbers appear on this page.** The per-route table reports **49.6 ns** for a one-parameter Pooled route (`bench_test.go`, the upstream micro-benchmark across MuxMaster's seven route categories). The competitor table reports **45 ns** (`competitor/bench_test.go`, the apples-to-apples harness that registers an identical route set on every router). Both numbers come from the same machine, the same Go release, and the same v1.1.0 source; they differ because the harnesses are different (`competitor/bench_test.go` runs against a flatter route set with no static siblings to give every competitor its best shot). Use **45 ns** when comparing MuxMaster against other routers; use **49.6 ns** when comparing across MuxMaster route categories on the same harness.

## Per route category — v1.0.1 vs v1.1.0 default vs v1.1.0 Pooled

| Case                       | v1.0.1                   | v1.1.0 default           | v1.1.0 Pooled |
|----------------------------|--------------------------|--------------------------|---------------|
| Static route               | 27 ns / 0 B              | 25.1 ns / 0 B            | 25.1 ns / 0 B |
| 1-parameter route          | 110 ns / 416 B / 1 alloc | 105 ns / 384 B / 1 alloc | **49.6 ns / 0 B / 0 allocs** |
| 2-parameter route          | 124 ns / 448 B / 1 alloc | 119 ns / 416 B / 1 alloc | **55.9 ns / 0 B / 0 allocs** |
| 3-parameter route          | 138 ns / 480 B / 1 alloc | 135 ns / 480 B / 1 alloc | **58.6 ns / 0 B / 0 allocs** |
| Catch-all                  | 112 ns / 384 B / 1 alloc | 108 ns / 384 B / 1 alloc | **43.9 ns / 0 B / 0 allocs** |
| Parallel 1-parameter route | 105 ns / 384 B / 1 alloc | 100 ns / 384 B / 1 alloc | **6.3 ns / 0 B / 0 allocs** |
| Fast 1-parameter route     | 51 ns / 32 B / 1 alloc   | 50.3 ns / 32 B / 1 alloc | n/a (Fast path uses `Params`) |

The **Pooled** column requires the opt-in `mux.PoolRequestBundle = true`. Lifetime contract: handlers must not retain `*http.Request` past return. Full guide and the only safe goroutine pattern at [`/docs/max-performance`](/docs/max-performance).

## Competitive — every Go HTTP router on a 1-parameter route

| Router                          | ns/op     | B/op  | allocs/op | vs MuxMaster Pooled |
|---------------------------------|-----------|-------|-----------|---------------------|
| **MuxMaster Pooled (Opt O13)**  | **45**    | **0** | **0**     | — baseline (winner) |
| MuxMaster Fast (`HandleFast`)   | 50        | 32    | 1         | +11 %               |
| MuxMaster default (`Handle`)    | 108       | 384   | 1         | +140 %              |
| `httprouter`                    | 56        | 64    | 1         | +24 %               |
| `Fiber v3` (`fasthttp` stack)   | 212       | 0     | 0         | +371 %              |
| `bunrouter` (vendored fork)     | 183       | 192   | 3         | +307 %              |
| `chi v5`                        | 354       | 304   | 4         | +687 %              |
| `gorilla/mux`                   | 3 444 278 | n/a   | 156 015   | (regex-walk router) |

**MuxMaster is the fastest Go HTTP router across every route category measured.** With the opt-in `PoolRequestBundle` it is the only stdlib-compatible router with zero allocations on parameterised routes in the entire Go ecosystem.

The full per-category report — covering 2- and 3-parameter routes, catch-alls, parallel dispatch, scaling tests, and middleware overhead — is archived at [`reports/perf-audit-2026-05-12/2026-05-12-competitor-showdown.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/reports/perf-audit-2026-05-12/2026-05-12-competitor-showdown.md).

## Notes on the numbers

- **MuxMaster default** allocates one tiered bundle per parameterised request (384 / 416 / 480 B for 1 / 2 / 3+ parameters). The bundle fuses the per-request `*http.Request` copy with its context, so the router, `net/http`, and the handler share a single GC object.
- **MuxMaster Pooled** recycles that bundle through three tier-matched `sync.Pool`s, eliminating the allocation entirely. The handler observes the same `*http.Request` it has always observed — the only contract is that the pointer must not survive past handler return.
- **`httprouter`** allocates a 64 B `Params` slice and passes it as a third argument outside the `http.Handler` interface. It is not a `net/http` drop-in; existing middleware does not compose without an adapter.
- **`Fiber v3`** runs on `fasthttp`, a parallel HTTP stack with its own request and response types. Zero allocations on the router itself, but the entire request lifecycle is incompatible with `net/http` and the surrounding ecosystem.
- **`bunrouter`** wraps a vendored fork of `httprouter` and adds three allocations for the handler adapter.
- **`chi v5`** is a regex-aware `net/http`-compatible router; its overhead comes from per-request middleware-chain materialisation.
- **`gorilla/mux`** matches routes by walking a regex list per request; it is included for completeness and for adopters considering a migration.

## Reproduce

```bash
# Clone the upstream module at the v1.1.0 tag.
git clone https://github.com/FlavioCFOliveira/MuxMaster.git
cd MuxMaster
git checkout v1.1.0

# Internal benchmarks (per route category).
go test -bench=. -benchmem -benchtime=2s -count=10 ./... | tee bench.txt
benchstat bench.txt

# Competitive benchmarks (every router on the same route set).
go test -bench=. -benchmem -benchtime=2s -count=10 ./competitor/... | tee bench_competitor.txt
benchstat bench_competitor.txt
```

Raw output, asm listings, escape analysis, and the SYNTHESIS document are archived at [`reports/perf-audit-2026-05-12/`](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.1.0/reports/perf-audit-2026-05-12).

## Source

- Internal benchmark suite — [`bench_test.go` at v1.1.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/bench_test.go)
- Competitive benchmark suite — [`competitor/bench_test.go` at v1.1.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/competitor/bench_test.go)
- Changelog entry — [`CHANGELOG.md` `## [1.1.0]`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/CHANGELOG.md)
- Full deep-audit report — [`reports/perf-audit-2026-05-12/`](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.1.0/reports/perf-audit-2026-05-12)
