---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# Benchmarks

This page publishes the results of this website's benchmark campaign of 2026-09-26, which measured MuxMaster v1.3.0 against MuxMaster v1.1.0 and against four other Go HTTP routers (httprouter, bunrouter, chi, and gorilla/mux) on one AMD Ryzen 9 5900HX host with go1.27.1. Every figure is a median of 10 samples, every comparison was tested for statistical significance with `benchstat`, and every raw output, command, and caveat is archived in the campaign archive [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26) of this website's repository.

## Summary

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

- **Router comparison.** Among the five routers measured, MuxMaster was the fastest in six of the eight route categories of the upstream competitor suite: static, 1 parameter, 3 parameters, not found, parallel static, and parallel 1 parameter. The winning MuxMaster mode differs by category (default `Handle`, `Handle` with `PoolRequestBundle`, or `HandleFast`).
- **Where MuxMaster was not the fastest.** httprouter was the fastest router on catch-all routes, ahead of every MuxMaster mode. On 2 parameters, pooled MuxMaster and httprouter showed no significant difference. MuxMaster's default mode, which allocates one request bundle per parameterised request, was slower than httprouter on every parameterised category.
- **v1.1.0 compared with v1.3.0.** The request hot path allocates the same in both versions. Of the 18 shared root-package benchmarks, 3 are faster in v1.3.0, 7 show no significant difference, and 8 are 1.42% to 6.25% slower. Several router and middleware paths became much faster (for example `ThrottlePerIP` −97.35% and automatic `OPTIONS` −42.95%), and security fixes made `CORS` without an `Origin` header, `Recoverer`, and `BasicAuth` slower.

This website itself runs on MuxMaster with `PoolRequestBundle` and `PoolFastParams` enabled; [Built with MuxMaster](/built-with-muxmaster) reports what that configuration costs per request on the site's own request path.

## Host and method

The campaign ran on one host, in one session, with one pinned toolchain. The same facts apply to every table on this page except the historical section.

| Fact | Value |
|---|---|
| CPU | AMD Ryzen 9 5900HX with Radeon Graphics; 8 cores, 16 threads (SMT on) |
| Frequency governor | `amd-pstate-epp`, governor `performance` on all 16 CPUs |
| Operating system | Ubuntu 24.04.5 LTS, Linux 6.8.0-139-generic, x86-64 |
| Go toolchain | go1.27.1 linux/amd64, `GOTOOLCHAIN=local`, `GOAMD64=v1`, `GOMAXPROCS=16` |
| Date | 2026-09-26 |
| Code measured | `v1.1.0` (commit `8af68ecd7b6f35b0f425133d94e8dd6f584248e3`) and `v1.3.0` (commit `119f2895bf95a29fb2566d875facb1044c6fa9c2`), each from its own detached checkout of the tag |
| Samples | 10 per benchmark (`-count=10` equivalent): 10 rounds of `-count=1 -benchtime=1s`, each sample from a separate process |
| Order | v1.1.0 and v1.3.0 interleaved in each round, in ABBA order (odd rounds v1.1.0 first, even rounds v1.3.0 first) |
| Statistics | `benchstat` (`golang.org/x/perf` `v0.0.0-20260908200009-22c9c6c9d4da`): median with 95% confidence interval; Mann-Whitney U test; a delta with p ≥ 0.05 is reported as "no significant difference". A p printed as 0.000 means p < 0.0005. |
| Competitor routers | httprouter v1.3.0, bunrouter v1.0.23 (through its `http.Handler` adapter), chi v5.3.2, gorilla/mux v1.8.1 |

MuxMaster modes in the router comparison:

- **default** — `Handle` with default settings.
- **Pooled** — `Handle` with `Mux.PoolRequestBundle = true`.
- **Fast** — `HandleFast` without `PoolFastParams`.

## MuxMaster compared with other routers

The upstream `competitor/` suite registers the same route set on every router and measures eight categories. Caption for both tables: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 rounds), median ± 95% confidence interval, then B/op and allocs/op; MuxMaster v1.3.0. "Not measured" means the suite has no benchmark for that mode and category. Source: `benchstat/competitor-v130.txt` and `benchstat/competitor-v130-vs-*.txt` in the [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

### Fastest router per category

"Runner-up vs fastest" is the runner-up's time relative to the fastest router, from `benchstat` with the fastest as the baseline.

| Category | Fastest (median) | Runner-up (median) | Runner-up vs fastest | p | Fastest non-MuxMaster router |
|---|---|---|---|---|---|
| Static (`StaticRoute`) | MuxMaster default 28.04 ns | MuxMaster Fast 29.10 ns | +3.80% | 0.000 | httprouter 34.67 ns (+23.65% vs fastest, p=0.000) |
| 1 parameter (`ParamRoute1`) | MuxMaster Fast 45.55 ns | MuxMaster Pooled 46.70 ns | +2.51% | 0.001 | httprouter 49.67 ns (+9.03% vs fastest, p=0.000) |
| 2 parameters (`ParamRoute2`) | MuxMaster Pooled 59.41 ns | httprouter 59.53 ns | no significant difference | 0.668 | httprouter 59.53 ns (no significant difference vs fastest, p=0.668) |
| 3 parameters (`ParamRoute3`) | MuxMaster Pooled 65.17 ns | httprouter 75.37 ns | +15.66% | 0.000 | httprouter 75.37 ns (+15.66% vs fastest, p=0.000) |
| Catch-all (`WildcardRoute`) | httprouter 42.92 ns | MuxMaster Fast 45.61 ns | +6.24% | 0.000 | httprouter 42.92 ns (fastest) |
| Not found (`NotFound`) | MuxMaster default 257.3 ns | bunrouter (http.Handler) 285.6 ns | +11.00% | 0.000 | bunrouter (http.Handler) 285.6 ns (+11.00% vs fastest, p=0.000) |
| Parallel static (`ParallelStaticRoute`) | MuxMaster default 4.121 ns | MuxMaster Fast 4.183 ns | +1.50% | 0.001 | httprouter 4.902 ns (+18.95% vs fastest, p=0.000) |
| Parallel 1 parameter (`ParallelParamRoute`) | MuxMaster Pooled 6.854 ns | MuxMaster Fast 16.24 ns | +136.94% | 0.000 | httprouter 22.07 ns (+222.07% vs fastest, p=0.000) |

MuxMaster was the fastest measured router in six of the eight categories: static (default mode, 28.04 ns), 1 parameter (Fast mode, 45.55 ns), 3 parameters (Pooled mode, 65.17 ns), not found (default mode, 257.3 ns), parallel static (default mode, 4.121 ns), and parallel 1 parameter (Pooled mode, 6.854 ns). In each of these categories, the fastest non-MuxMaster router was slower with p < 0.001. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

### All measurements

| Category | MuxMaster default | MuxMaster Pooled | MuxMaster Fast | httprouter | bunrouter (http.Handler) | chi v5 | gorilla/mux |
|---|---|---|---|---|---|---|---|
| Static (`StaticRoute`) | 28.04 ns ± 2%<br>0 B, 0 allocs | 29.11 ns ± 1%<br>0 B, 0 allocs | 29.10 ns ± 1%<br>0 B, 0 allocs | 34.67 ns ± 1%<br>0 B, 0 allocs | 162.8 ns ± 1%<br>416 B, 3 allocs | 225.2 ns ± 2%<br>368 B, 2 allocs | 576.6 ns ± 1%<br>848 B, 7 allocs |
| 1 parameter (`ParamRoute1`) | 115.0 ns ± 1%<br>384 B, 1 alloc | 46.70 ns ± 3%<br>0 B, 0 allocs | 45.55 ns ± 1%<br>32 B, 1 alloc | 49.67 ns ± 1%<br>64 B, 1 alloc | 160.5 ns ± 1%<br>416 B, 3 allocs | 368.9 ns ± 5%<br>704 B, 4 allocs | 954.0 ns ± 1%<br>1152 B, 8 allocs |
| 2 parameters (`ParamRoute2`) | 132.2 ns ± 1%<br>416 B, 1 alloc | 59.41 ns ± 2%<br>0 B, 0 allocs | 63.42 ns ± 1%<br>64 B, 1 alloc | 59.53 ns ± 1%<br>64 B, 1 alloc | 179.5 ns ± 3%<br>416 B, 3 allocs | 413.7 ns ± 3%<br>704 B, 4 allocs | 1.492 µs ± 1%<br>1168 B, 8 allocs |
| 3 parameters (`ParamRoute3`) | 143.6 ns ± 1%<br>480 B, 1 alloc | 65.17 ns ± 1%<br>0 B, 0 allocs | 83.31 ns ± 2%<br>96 B, 1 alloc | 75.37 ns ± 1%<br>96 B, 1 alloc | 180.2 ns ± 1%<br>416 B, 3 allocs | 428.0 ns ± 4%<br>704 B, 4 allocs | 1.703 µs ± 0%<br>1184 B, 8 allocs |
| Catch-all (`WildcardRoute`) | 114.4 ns ± 1%<br>384 B, 1 alloc | 45.71 ns ± 2%<br>0 B, 0 allocs | 45.61 ns ± 1%<br>32 B, 1 alloc | 42.92 ns ± 1%<br>32 B, 1 alloc | 155.4 ns ± 2%<br>416 B, 3 allocs | 345.0 ns ± 3%<br>704 B, 4 allocs | 1.589 µs ± 0%<br>1152 B, 8 allocs |
| Not found (`NotFound`) | 257.3 ns ± 1%<br>107 B, 3 allocs | not measured | not measured | 383.2 ns ± 1%<br>92 B, 3 allocs | 285.6 ns ± 1%<br>145 B, 4 allocs | 354.4 ns ± 1%<br>459 B, 5 allocs | 1.030 µs ± 1%<br>163 B, 4 allocs |
| Parallel static (`ParallelStaticRoute`) | 4.121 ns ± 1%<br>0 B, 0 allocs | not measured | 4.183 ns ± 1%<br>0 B, 0 allocs | 4.902 ns ± 1%<br>0 B, 0 allocs | 129.0 ns ± 3%<br>416 B, 3 allocs | 134.7 ns ± 2%<br>368 B, 2 allocs | 353.1 ns ± 2%<br>849 B, 7 allocs |
| Parallel 1 parameter (`ParallelParamRoute`) | 104.9 ns ± 1%<br>384 B, 1 alloc | 6.854 ns ± 1%<br>0 B, 0 allocs | 16.24 ns ± 2%<br>32 B, 1 alloc | 22.07 ns ± 2%<br>64 B, 1 alloc | 129.3 ns ± 3%<br>416 B, 3 allocs | 238.5 ns ± 2%<br>704 B, 4 allocs | 469.2 ns ± 2%<br>1153 B, 8 allocs |

### Where MuxMaster was not the fastest

These results carry the same weight as the favourable ones above.

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

- **Catch-all: httprouter was fastest (42.92 ns).** Every MuxMaster mode was slower: Fast by 6.24% (45.61 ns), Pooled by 6.49% (45.71 ns), and default by 166.51% (114.4 ns); p < 0.001 for each (`benchstat/competitor-v130-vs-HTTProuter.txt`).
- **2 parameters: no significant difference.** Pooled MuxMaster (59.41 ns) and httprouter (59.53 ns) could not be separated (p = 0.668).
- **Default mode against httprouter: slower on every parameterised route.** 1 parameter: 115.0 ns against 49.67 ns. 2 parameters: 132.2 ns against 59.53 ns. 3 parameters: 143.6 ns against 75.37 ns. Catch-all: 114.4 ns against 42.92 ns. Parallel 1 parameter: 104.9 ns against 22.07 ns. In these categories the default mode allocates 384–480 B per request (1 allocation), because it copies the request into a bundle so that handlers keep the standard `http.Handler` signature and may retain `*http.Request`; httprouter allocates 32–96 B (1 allocation) and uses a three-argument handler.
- **Fast mode against httprouter: slower on 2 parameters, 3 parameters, and catch-all.** 63.42 ns against 59.53 ns, 83.31 ns against 75.37 ns, and 45.61 ns against 42.92 ns. Fast mode is faster on static, 1 parameter, parallel static, and parallel 1 parameter.

### Every MuxMaster mode compared with httprouter

In this table the baseline is each MuxMaster mode: a positive value means httprouter takes longer than that MuxMaster mode; a negative value means httprouter is faster. Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 rounds), medians. Source: `benchstat/competitor-v130-vs-MuxMaster.txt`, `benchstat/competitor-v130-vs-MuxMasterPooled.txt`, and `benchstat/competitor-v130-vs-MuxMasterFast.txt` in the [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

| Category | MuxMaster mode | MuxMaster (median) | httprouter (median) | httprouter vs MuxMaster mode | p |
|---|---|---|---|---|---|
| Static (`StaticRoute`) | default | 28.04 ns | 34.67 ns | +23.65% | 0.000 |
| Static (`StaticRoute`) | Pooled | 29.11 ns | 34.67 ns | +19.08% | 0.000 |
| Static (`StaticRoute`) | Fast | 29.10 ns | 34.67 ns | +19.12% | 0.000 |
| 1 parameter (`ParamRoute1`) | default | 115.0 ns | 49.67 ns | -56.79% | 0.000 |
| 1 parameter (`ParamRoute1`) | Pooled | 46.70 ns | 49.67 ns | +6.36% | 0.001 |
| 1 parameter (`ParamRoute1`) | Fast | 45.55 ns | 49.67 ns | +9.03% | 0.000 |
| 2 parameters (`ParamRoute2`) | default | 132.2 ns | 59.53 ns | -54.97% | 0.000 |
| 2 parameters (`ParamRoute2`) | Pooled | 59.41 ns | 59.53 ns | no significant difference | 0.668 |
| 2 parameters (`ParamRoute2`) | Fast | 63.42 ns | 59.53 ns | -6.14% | 0.000 |
| 3 parameters (`ParamRoute3`) | default | 143.6 ns | 75.37 ns | -47.51% | 0.000 |
| 3 parameters (`ParamRoute3`) | Pooled | 65.17 ns | 75.37 ns | +15.66% | 0.000 |
| 3 parameters (`ParamRoute3`) | Fast | 83.31 ns | 75.37 ns | -9.54% | 0.000 |
| Catch-all (`WildcardRoute`) | default | 114.4 ns | 42.92 ns | -62.48% | 0.000 |
| Catch-all (`WildcardRoute`) | Pooled | 45.71 ns | 42.92 ns | -6.09% | 0.000 |
| Catch-all (`WildcardRoute`) | Fast | 45.61 ns | 42.92 ns | -5.88% | 0.000 |
| Not found (`NotFound`) | default | 257.3 ns | 383.2 ns | +48.95% | 0.000 |
| Parallel static (`ParallelStaticRoute`) | default | 4.121 ns | 4.902 ns | +18.95% | 0.000 |
| Parallel static (`ParallelStaticRoute`) | Fast | 4.183 ns | 4.902 ns | +17.19% | 0.000 |
| Parallel 1 parameter (`ParallelParamRoute`) | default | 104.9 ns | 22.07 ns | -78.96% | 0.000 |
| Parallel 1 parameter (`ParallelParamRoute`) | Pooled | 6.854 ns | 22.07 ns | +222.07% | 0.000 |
| Parallel 1 parameter (`ParallelParamRoute`) | Fast | 16.24 ns | 22.07 ns | +35.93% | 0.000 |

Against bunrouter (through its `http.Handler` adapter), chi v5, and gorilla/mux, each MuxMaster mode was faster in every category where that mode was measured, with p < 0.001 for each comparison. bunrouter's native `bunrouter.HandlerFunc` API was not measured, and no statement on this page applies to it.

## MuxMaster v1.1.0 compared with v1.3.0

### Root package (`bench_test.go`)

The 18 benchmarks below exist at both tags with identical benchmark bodies; each side runs its own version of the router. Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 interleaved rounds), median ± 95% CI. Source: `benchstat/root-v110-vs-v130.txt` in the [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

| Benchmark | v1.1.0 time/op | v1.3.0 time/op | Change | p | v1.1.0 B/op → v1.3.0 | v1.1.0 allocs/op → v1.3.0 |
|---|---|---|---|---|---|---|
| StaticRoute | 26.00 ns ± 1% | 27.20 ns ± 3% | +4.62% | 0.000 | identical (0) | identical (0) |
| ParamRoute1 | 110.8 ns ± 1% | 115.9 ns ± 1% | +4.65% | 0.000 | identical (384) | identical (1) |
| ParamRoute2 | 126.7 ns ± 1% | 126.6 ns ± 1% | no significant difference | 0.669 | identical (416) | identical (1) |
| ParamRoute3 | 143.4 ns ± 2% | 143.4 ns ± 1% | no significant difference | 0.725 | identical (480) | identical (1) |
| WildcardRoute | 110.2 ns ± 1% | 115.2 ns ± 1% | +4.54% | 0.000 | identical (384) | identical (1) |
| NotFound | 223.3 ns ± 2% | 218.4 ns ± 1% | -2.19% | 0.004 | 101 → 99 (-1.98%, p=0.000) | identical (3) |
| ParallelStaticRoute | 3.789 ns ± 1% | 3.928 ns ± 1% | +3.67% | 0.000 | identical (0) | identical (0) |
| ParallelParamRoute | 105.4 ns ± 1% | 105.0 ns ± 2% | no significant difference | 0.753 | identical (384) | identical (1) |
| FastStaticRoute | 26.00 ns ± 1% | 27.62 ns ± 2% | +6.25% | 0.000 | identical (0) | identical (0) |
| FastParamRoute1 | 44.09 ns ± 0% | 44.34 ns ± 1% | no significant difference | 0.190 | identical (32) | identical (1) |
| FastParamRoute2 | 68.50 ns ± 1% | 62.30 ns ± 2% | -9.06% | 0.000 | identical (64) | identical (1) |
| FastParamRoute3 | 86.64 ns ± 1% | 76.63 ns ± 2% | -11.55% | 0.000 | identical (96) | identical (1) |
| FastParallelParamRoute | 15.19 ns ± 1% | 15.40 ns ± 1% | +1.42% | 0.007 | identical (32) | identical (1) |
| PooledParamRoute1 | 44.58 ns ± 1% | 45.84 ns ± 1% | +2.83% | 0.001 | identical (0) | identical (0) |
| PooledParamRoute2 | 55.90 ns ± 1% | 55.58 ns ± 3% | no significant difference | 0.436 | identical (0) | identical (0) |
| PooledParamRoute3 | 59.76 ns ± 1% | 59.58 ns ± 2% | no significant difference | 0.469 | identical (0) | identical (0) |
| PooledWildcardRoute | 45.21 ns ± 1% | 45.48 ns ± 2% | no significant difference | 0.247 | identical (0) | identical (0) |
| PooledParallelParamRoute | 6.449 ns ± 1% | 6.585 ns ± 1% | +2.12% | 0.004 | identical (0) | identical (0) |

Figures: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

- **Allocations.** B/op and allocs/op are identical in 17 of the 18 benchmarks. The exception is `NotFound`: 101 → 99 B/op, with allocs/op identical.
- **Faster in v1.3.0 (3):** `NotFound` −2.19%, `FastParamRoute2` −9.06%, `FastParamRoute3` −11.55%.
- **No significant difference (7):** `ParamRoute2`, `ParamRoute3`, `ParallelParamRoute`, `FastParamRoute1`, `PooledParamRoute2`, `PooledParamRoute3`, `PooledWildcardRoute`.
- **Slower in v1.3.0 (8), by 1.42% to 6.25%:** `StaticRoute`, `ParamRoute1`, `WildcardRoute`, `ParallelStaticRoute`, `FastStaticRoute`, `FastParallelParamRoute`, `PooledParamRoute1`, `PooledParallelParamRoute`. The campaign did not establish the cause of these regressions and did not separate a code change from a change in code layout or alignment.

**Noise floor.** Each round also ran the v1.3.0 root binary a second time. Comparing the two v1.3.0 series (an A/A test) produced 0 significant differences in 40 rows at alpha = 0.05, with a geometric-mean difference of +0.09%, so the deltas above are larger than the noise of the method. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: `benchstat/root-v130-aa-noise-floor.txt` in the [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

### Changes measured since v1.1.0: gains and costs

This table maps each item of the upstream README's "Measured changes since v1.1.0" list to the benchmark that measures it. Where the benchmark exists at both tags, the v1.1.0 column is a real v1.1.0 measurement from this campaign. The upstream list's "before" values were measured on 2026-09-24 against pre-change commits, not against v1.1.0, and are not used here. Where the benchmark exists only at v1.3.0, the change since v1.1.0 could not be measured and only the v1.3.0 value is given. Costs added by security fixes are listed with the gains. The waste-hunt harness is the upstream benchmark set in `reports/perf-lab-2026-09-24/waste-hunt/` (see [Source](#source)). Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians. Sources: `benchstat/perfaudit-v110-vs-v130.txt`, `benchstat/root-v110-vs-v130.txt`, `benchstat/wastehunt-v130.txt`.

| Item | Benchmark | v1.1.0 | v1.3.0 | Change | p |
|---|---|---|---|---|---|
| Route registration, 100 routes | `RegisterRoutes/N=100` | not measurable (no benchmark at v1.1.0) | 54.64 µs, 88384 B, 1135 allocs | v1.3.0 only | — |
| Route registration, 1 000 routes | `RegisterRoutes/N=1000` | not measurable (no benchmark at v1.1.0) | 680.0 µs, 1091114 B, 13405 allocs | v1.3.0 only | — |
| Route registration, 5 000 routes | `RegisterRoutes/N=5000` | not measurable (no benchmark at v1.1.0) | 4.593 ms, 6379718 B, 75938 allocs | v1.3.0 only | — |
| `Mount`, static prefix (root suite) | `Mount_Static` | not measurable (no benchmark at v1.1.0) | 346.6 ns, 864 B, 2 allocs | v1.3.0 only | — |
| `Mount`, parameterised prefix (root suite) | `Mount_Param` | not measurable (no benchmark at v1.1.0) | 476.1 ns, 1248 B, 3 allocs | v1.3.0 only | — |
| `Mount`, static prefix (waste-hunt harness) | `Mount/current` | not measurable (no benchmark at v1.1.0) | 236.8 ns, 864 B, 2 allocs | v1.3.0 only | — |
| `CleanPath` middleware, dirty path | `Middleware_CleanPath_Dirty` | 278.2 ns, 552 B, 5 allocs | 196.1 ns, 504 B, 4 allocs | -29.53% | 0.000 |
| `CleanPath` middleware, dirty path (waste-hunt harness) | `CleanPathChanged/current` | not measurable (no benchmark at v1.1.0) | 137.5 ns, 464 B, 2 allocs | v1.3.0 only | — |
| `StripSlashes` middleware, dirty path | `Middleware_StripSlashes_Dirty` | 192.1 ns, 512 B, 3 allocs | 125.5 ns, 464 B, 2 allocs | -34.67% | 0.000 |
| Trailing-slash redirect, no middleware | `RedirectTSL` | 865.2 ns, 1248 B, 13 allocs | 660.9 ns, 1088 B, 10 allocs | -23.61% | 0.000 |
| Trailing-slash redirect, 5-middleware chain | `RedirectTSLWithMiddleware` | not measurable (no benchmark at v1.1.0) | 805.5 ns, 1440 B, 11 allocs | v1.3.0 only | — |
| 405 Method Not Allowed | `MethodNotAllowed` | 153.2 ns, 88 B, 2 allocs | 124.2 ns, 102 B, 1 alloc | -18.96% | 0.000 |
| Automatic `OPTIONS` | `OPTIONSAuto` | 120.5 ns, 40 B, 3 allocs | 68.72 ns, 16 B, 1 alloc | -42.95% | 0.000 |
| `Text` response helper | `Text/current` | not measurable (no benchmark at v1.1.0) | 60.27 ns, 16 B, 1 alloc | v1.3.0 only | — |
| `ThrottlePerIP`, one client | `Middleware_ThrottlePerIP_Hit` | 4.465 µs, 376 B, 5 allocs | 118.1 ns, 0 B, 0 allocs | -97.35% | 0.000 |
| `ThrottleBacklog`, no wait | `Middleware_ThrottleBacklog_NoWait` | 31.72 ns, 0 B, 0 allocs | 10.70 ns, 0 B, 0 allocs | -66.26% | 0.000 |
| `RequestID`, generate | `Middleware_RequestID_Generate` | 1.161 µs, 864 B, 9 allocs | 293.9 ns, 801 B, 4 allocs | -74.67% | 0.000 |
| `RequestID`, propagate | `Middleware_RequestID_Propagate` | 432.9 ns, 832 B, 8 allocs | 251.9 ns, 800 B, 4 allocs | -41.80% | 0.000 |
| `Logger` | `Middleware_Logger` | 7.261 µs, 40 B, 4 allocs | 5.665 µs, 0 B, 0 allocs | -21.98% | 0.000 |
| `Compress`, small body | `Middleware_Compress_SmallBody` | 371.4 ns, 677 B, 2 allocs | 275.5 ns, 98 B, 0 allocs | -25.81% | 0.000 |
| `Compress`, large body | `Middleware_Compress_LargeBody` | 830.6 ns, 2862 B, 2 allocs | 299.7 ns, 85.5 B, 0 allocs | -63.92% | 0.000 |
| `JWTAuth` HS256, valid token | `Middleware_JWTAuth_HS256_Hit` | 4.662 µs, 866 B, 10 allocs | 4.122 µs, 706 B, 7 allocs | -11.59% | 0.000 |
| `JWTAuth` HS256, invalid token | `Middleware_JWTAuth_HS256_Miss` | 1.240 µs, 305 B, 10 allocs | 617.6 ns, 144 B, 7 allocs | -50.18% | 0.000 |
| `RealIP`, `X-Forwarded-For` | `Middleware_RealIP_XFF` | 48.87 ns, 80 B, 2 allocs | 49.93 ns, 80 B, 2 allocs | no significant difference | 0.172 |
| `RealIP`, no header | `Middleware_RealIP_NoHeader` | 120.8 ns, 16 B, 1 alloc | 128.0 ns, 16 B, 1 alloc | +5.88% | 0.000 |
| `APIKey`, valid key | `Middleware_APIKey_Hit` | 462.8 ns, 448 B, 7 allocs | 438.8 ns, 416 B, 6 allocs | -5.20% | 0.000 |
| `APIKey`, invalid key | `Middleware_APIKey_Miss` | 409.5 ns, 96 B, 6 allocs | 415.6 ns, 96 B, 6 allocs | +1.48% | 0.003 |
| `BasicAuth`, valid credentials (security fix TSC-2026-0002) | `Middleware_BasicAuth_Hit` | 199.9 ns, 48 B, 2 allocs | 327.6 ns, 48 B, 2 allocs | +63.92% | 0.000 |
| `BasicAuth`, invalid credentials (security fix TSC-2026-0002) | `Middleware_BasicAuth_Miss` | 520.0 ns, 152 B, 8 allocs | 658.7 ns, 152 B, 8 allocs | +26.66% | 0.000 |
| `CORS`, no `Origin` header (security fix TM-2026-033) | `Middleware_CORS_NoOrigin` | 20.24 ns, 0 B, 0 allocs | 81.79 ns, 112 B, 1 alloc | +304.08% | 0.000 |
| `CORS`, allowed origin | `Middleware_CORS_AllowedOrigin` | 207.9 ns, 416 B, 3 allocs | 218.1 ns, 512 B, 3 allocs | +4.93% | 0.029 |
| `CORS`, preflight | `Middleware_CORS_Preflight` | 226.2 ns, 416 B, 3 allocs | 237.0 ns, 512 B, 3 allocs | +4.77% | 0.037 |
| `Recoverer`, no panic (security fix O-14) | `Middleware_Recoverer_NoPanic` | 6.502 ns, 0 B, 0 allocs | 17.46 ns, 0 B, 0 allocs | +168.59% | 0.000 |

Not re-measured by this campaign: the `-cpu` scaling results of the upstream contention hunt (`ThrottlePerIP` with many clients at 16 CPUs, `ThrottleBacklog` at 1 and 16 CPUs, `RequestID` at 1, 4, and 16 CPUs, `OAuth2Introspect` eviction at 16 CPUs). The campaign ran every benchmark at `GOMAXPROCS=16` only. Those upstream figures are quoted, with their attribution, on the [Performance](/docs/performance#upstream-measurements-not-repeated-by-the-campaign) page.

### Perf-audit harness: every benchmark

The upstream perf-audit harness (`reports/perf-audit-2026-05-12/`) has identical benchmark bodies at both tags, so it measures the router, middleware, and middleware chains of each version directly. The full table is published here, including the rows that became slower. Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 interleaved rounds), median ± 95% CI. Source: `benchstat/perfaudit-v110-vs-v130.txt`.

| Benchmark | v1.1.0 time/op | v1.3.0 time/op | Change | p | v1.1.0 B/op → v1.3.0 | v1.1.0 allocs/op → v1.3.0 |
|---|---|---|---|---|---|---|
| NotFoundClean | 209.7 ns ± 3% | 206.4 ns ± 1% | -1.53% | 0.006 | 98 → 97 (-1.02%, p=0.001) | identical (3) |
| NotFoundCustomHandler | 16.62 ns ± 2% | 16.41 ns ± 1% | -1.26% | 0.027 | identical (0) | identical (0) |
| NotFoundWithMethodAllowedLookup | 243.8 ns ± 4% | 221.8 ns ± 1% | -9.03% | 0.000 | 103 → 99 (-3.88%, p=0.000) | identical (3) |
| MethodNotAllowed | 153.2 ns ± 3% | 124.2 ns ± 1% | -18.96% | 0.000 | 88 → 102 (+15.91%, p=0.000) | 2 → 1 |
| OPTIONSAuto | 120.5 ns ± 2% | 68.72 ns ± 1% | -42.95% | 0.000 | 40 → 16 (-60.00%, p=0.000) | 3 → 1 |
| GroupDispatch | 96.23 ns ± 12% | 97.64 ns ± 2% | no significant difference | 0.255 | identical (384) | identical (1) |
| NestedGroupDispatch | 96.30 ns ± 10% | 97.99 ns ± 2% | no significant difference | 0.210 | identical (384) | identical (1) |
| RedirectTSL | 865.2 ns ± 2% | 660.9 ns ± 0% | -23.61% | 0.000 | 1248 → 1088 (-12.82%, p=0.000) | 13 → 10 |
| PathParamLookup | 113.1 ns ± 8% | 111.4 ns ± 2% | -1.55% | 0.041 | identical (384) | identical (1) |
| PathParamFast | 34.54 ns ± 3% | 36.37 ns ± 1% | +5.28% | 0.000 | identical (32) | identical (1) |
| ParamsFromContext | 111.4 ns ± 10% | 117.5 ns ± 2% | no significant difference | 0.101 | identical (416) | identical (1) |
| Middleware_APIKey_Hit | 462.8 ns ± 2% | 438.8 ns ± 0% | -5.20% | 0.000 | 448 → 416 (-7.14%, p=0.000) | 7 → 6 |
| Middleware_APIKey_Miss | 409.5 ns ± 1% | 415.6 ns ± 1% | +1.48% | 0.003 | identical (96) | identical (6) |
| Middleware_BasicAuth_Hit | 199.9 ns ± 1% | 327.6 ns ± 0% | +63.92% | 0.000 | identical (48) | identical (2) |
| Middleware_BasicAuth_Miss | 520.0 ns ± 1% | 658.7 ns ± 1% | +26.66% | 0.000 | identical (152) | identical (8) |
| Middleware_CleanPath_Clean | 20.59 ns ± 1% | 20.83 ns ± 1% | +1.14% | 0.007 | identical (0) | identical (0) |
| Middleware_CleanPath_Dirty | 278.2 ns ± 3% | 196.1 ns ± 5% | -29.53% | 0.000 | 552 → 504 (-8.70%, p=0.000) | 5 → 4 |
| Middleware_Compress_NoGzip | 28.66 ns ± 1% | 28.94 ns ± 1% | +0.96% | 0.011 | identical (0) | identical (0) |
| Middleware_Compress_SmallBody | 371.4 ns ± 3% | 275.5 ns ± 4% | -25.81% | 0.000 | 677 → 98 (-85.52%, p=0.000) | 2 → 0 |
| Middleware_Compress_LargeBody | 830.6 ns ± 3% | 299.7 ns ± 4% | -63.92% | 0.000 | 2862 → 85.5 (-97.01%, p=0.000) | 2 → 0 |
| Middleware_CORS_NoOrigin | 20.24 ns ± 2% | 81.79 ns ± 6% | +304.08% | 0.000 | 0 → 112 (?, p=0.000) | 0 → 1 |
| Middleware_CORS_AllowedOrigin | 207.9 ns ± 7% | 218.1 ns ± 22% | +4.93% | 0.029 | 416 → 512 (+23.08%, p=0.000) | identical (3) |
| Middleware_CORS_Preflight | 226.2 ns ± 9% | 237.0 ns ± 9% | +4.77% | 0.037 | 416 → 512 (+23.08%, p=0.000) | identical (3) |
| Middleware_JWTAuth_HS256_Hit | 4.662 µs ± 3% | 4.122 µs ± 4% | -11.59% | 0.000 | 866 → 706 (-18.48%, p=0.000) | 10 → 7 |
| Middleware_JWTAuth_HS256_Miss | 1.240 µs ± 2% | 617.6 ns ± 5% | -50.18% | 0.000 | 305 → 144 (-52.79%, p=0.000) | 10 → 7 |
| Middleware_JWTAuth_ES256_Hit | 64.27 µs ± 0% | 63.90 µs ± 0% | -0.57% | 0.004 | 2052 → 1892 (-7.77%, p=0.000) | 29 → 26 |
| Middleware_Logger | 7.261 µs ± 0% | 5.665 µs ± 0% | -21.98% | 0.000 | 40 → 0 (-100.00%, p=0.000) | 4 → 0 |
| Middleware_NoCache | 171.4 ns ± 16% | 193.2 ns ± 27% | +12.75% | 0.029 | 400 → 480 (+20.00%, p=0.000) | 2 → 3 |
| Middleware_RealIP_NoHeader | 120.8 ns ± 2% | 128.0 ns ± 3% | +5.88% | 0.000 | identical (16) | identical (1) |
| Middleware_RealIP_XFF | 48.87 ns ± 5% | 49.93 ns ± 9% | no significant difference | 0.172 | identical (80) | identical (2) |
| Middleware_Recoverer_NoPanic | 6.502 ns ± 1% | 17.46 ns ± 1% | +168.59% | 0.000 | identical (0) | identical (0) |
| Middleware_RequestID_Generate | 1.161 µs ± 5% | 293.9 ns ± 27% | -74.67% | 0.000 | 864 → 801 (-7.29%, p=0.000) | 9 → 4 |
| Middleware_RequestID_Propagate | 432.9 ns ± 9% | 251.9 ns ± 36% | -41.80% | 0.000 | 832 → 800 (-3.85%, p=0.000) | 8 → 4 |
| Middleware_SetHeader | 160.7 ns ± 19% | 134.1 ns ± 35% | -16.56% | 0.009 | identical (416) | identical (3) |
| Middleware_StripSlashes_Clean | 3.989 ns ± 1% | 4.187 ns ± 1% | +4.96% | 0.000 | identical (0) | identical (0) |
| Middleware_StripSlashes_Dirty | 192.1 ns ± 20% | 125.5 ns ± 34% | -34.67% | 0.000 | 512 → 464 (-9.38%, p=0.000) | 3 → 2 |
| Middleware_ThrottleBacklog_NoWait | 31.72 ns ± 2% | 10.70 ns ± 2% | -66.26% | 0.000 | identical (0) | identical (0) |
| Middleware_ThrottlePerIP_Hit | 4.465 µs ± 1% | 118.1 ns ± 1% | -97.35% | 0.000 | 376 → 0 (-100.00%, p=0.000) | 5 → 0 |
| Middleware_Timeout | 6.049 µs ± 1% | 6.026 µs ± 2% | no significant difference | 0.324 | identical (592) | identical (5) |
| Middleware_WithValue | 107.1 ns ± 34% | 101.6 ns ± 45% | no significant difference | 0.912 | identical (368) | identical (2) |
| Chain_Production | 9.069 µs ± 1% | 6.537 µs ± 3% | -27.92% | 0.000 | 1001 → 996 (-0.50%, p=0.000) | 16 → 7 |
| Chain_AuthBasic | 202.3 ns ± 3% | 334.6 ns ± 2% | +65.37% | 0.000 | identical (32) | identical (2) |
| Chain_AuthJWT | 6.174 µs ± 3% | 4.636 µs ± 7% | -24.90% | 0.000 | 1733 → 1513 (-12.69%, p=0.000) | 19 → 11 |
| Chain_Security | 1.235 µs ± 6% | 1.274 µs ± 7% | no significant difference | 0.118 | 1624 → 1769 (+8.93%, p=0.000) | identical (14) |
| Chain_Heavy | 9.950 µs ± 2% | 7.617 µs ± 5% | -23.45% | 0.000 | 1786 → 1872 (+4.82%, p=0.000) | 22 → 14 |
| Chain_Minimal | 6.594 ns ± 2% | 17.29 ns ± 1% | +162.13% | 0.000 | identical (0) | identical (0) |

Some rows have confidence intervals of ±20% to ±45% on one or both sides (`Middleware_SetHeader`, `Middleware_WithValue`, `Middleware_RequestID_*`, `Middleware_StripSlashes_Dirty`, `Middleware_NoCache`, `Middleware_CORS_AllowedOrigin`, and `GroupDispatch` and `NestedGroupDispatch` at v1.1.0). Where the rank test reports a significant difference for these rows, the direction is supported but the size is uncertain. The chain benchmarks follow their middleware: `Chain_Minimal` (`Recoverer` only) +162.13% and `Chain_AuthBasic` (`BasicAuth` and `Recoverer`) +65.37% reflect the security-fix costs above. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

## Benchmarks new in v1.3.0

The benchmarks in this section do not exist at v1.1.0, so only their absolute v1.3.0 values are given; none of them is a change since v1.1.0.

### New root-package benchmarks

Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, median ± 95% CI. Source: `benchstat/root-v110-vs-v130.txt`.

| Benchmark | time/op | B/op | allocs/op |
|---|---|---|---|
| RegisterRoutes/N=100 | 54.64 µs ± 1% | 88384 | 1135 |
| RegisterRoutes/N=1000 | 680.0 µs ± 1% | 1091114 | 13405 |
| RegisterRoutes/N=5000 | 4.593 ms ± 7% | 6379718 | 75938 |
| ServeFiles | 791.9 ns ± 1% | 708 | 8 |
| GroupServeFiles | 791.2 ns ± 1% | 708 | 8 |
| AdversarialBacktracking/1 | 160.3 ns ± 1% | 384 | 1 |
| AdversarialBacktracking/2 | 190.7 ns ± 4% | 416 | 1 |
| AdversarialBacktracking/4 | 519.6 ns ± 1% | 896 | 6 |
| AdversarialBacktracking/8 | 1.155 µs ± 1% | 2080 | 11 |
| AdversarialBacktracking/16 | 2.084 µs ± 2% | 3808 | 13 |
| AdversarialBacktracking/32 | 4.007 µs ± 2% | 7648 | 15 |
| AdversarialBacktracking/64 | 7.606 µs ± 1% | 15328 | 17 |
| AdversarialBacktracking/128 | 15.13 µs ± 3% | 31456 | 19 |
| QuadraticBacktracking/2 | 192.2 ns ± 5% | 416 | 1 |
| QuadraticBacktracking/4 | 532.4 ns ± 3% | 896 | 6 |
| QuadraticBacktracking/8 | 1.172 µs ± 4% | 2080 | 11 |
| QuadraticBacktracking/16 | 2.146 µs ± 3% | 3808 | 13 |
| QuadraticBacktracking/32 | 4.242 µs ± 4% | 7648 | 15 |
| QuadraticBacktracking/64 | 9.232 µs ± 3% | 15328 | 17 |
| Mount_Static | 346.6 ns ± 3% | 864 | 2 |
| Mount_Param | 476.1 ns ± 3% | 1248 | 3 |
| Mount_TSRRedirect | 559.0 ns ± 3% | 920 | 6 |

`AdversarialBacktracking` and `QuadraticBacktracking` measure patterns built to force the router's bounded backtracking; their cost grows linearly with depth.

### New perf-audit benchmarks

Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, median ± 95% CI. Source: `benchstat/perfaudit-v110-vs-v130.txt` in the [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

| Benchmark | time/op | B/op | allocs/op |
|---|---|---|---|
| RedirectTSLWithMiddleware | 805.5 ns ± 1% | 1440 | 11 |
| ParallelRedirectTrailingSlash | 122.2 ns ± 1% | 368 | 2 |

### New middleware benchmarks (`middleware/bench_test.go`)

Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, median ± 95% CI. Source: `benchstat/middleware-v130.txt`.

| Benchmark | time/op | B/op | allocs/op |
|---|---|---|---|
| OAuth2CacheSetAtSaturation | 3.208 µs ± 0% | 288 | 2 |
| ThrottlePerIP/sequential-one-client | 115.4 ns ± 1% | 0 | 0 |
| ThrottlePerIP/many-clients-no-overlap | 121.3 ns ± 1% | 0 | 0 |
| Logger | 5.664 µs ± 0% | 0 | 0 |
| Recoverer_NoPanic | 20.82 ns ± 2% | 0 | 0 |
| RecovererWithLogger_NoPanic | 20.58 ns ± 1% | 0 | 0 |
| Compress/small-600B | 317.2 ns ± 0% | 32 | 2 |
| Compress/chunked-12KiB | 7.926 µs ± 1% | 58 | 3 |
| RealIP/1-hop | 165.3 ns ± 0% | 16 | 1 |
| RealIP/3-hops | 201.2 ns ± 0% | 16 | 1 |
| APIKeyHit | 450.2 ns ± 1% | 416 | 6 |
| BasicAuth/1-users/hit | 314.4 ns ± 0% | 32 | 2 |
| BasicAuth/10-users/hit | 547.5 ns ± 0% | 32 | 2 |
| BasicAuth/100-users/hit | 2.866 µs ± 0% | 32 | 2 |
| BasicAuth/1-users/miss | 700.5 ns ± 0% | 168 | 8 |
| BasicAuth/10-users/miss | 929.1 ns ± 0% | 168 | 8 |
| BasicAuth/100-users/miss | 3.235 µs ± 0% | 168 | 8 |
| JWTAuthHS256 | 4.384 µs ± 1% | 738 | 7 |

`BasicAuth` scans every registered user in constant time (security fix TSC-2026-0002), so its cost grows with the number of users.

## Caveats

- **Competitor suite provenance.** The upstream `v1.3.0` tag does not contain the competitor suite source: upstream `.gitignore` excludes `competitor/`, and the tag tracks only `competitor/vendor/`. The campaign copied `competitor/go.mod`, `go.sum`, `bench_test.go`, and `route_existence_timing_test.go` from the upstream working tree into the `v1.3.0` checkout and archived them, with their SHA-256 sums, in `competitor-suite/`. The suite was built with `-mod=mod`, so the MuxMaster code it measured is the `v1.3.0` checkout (confirmed with `go version -m`), but the benchmark source itself cannot be pinned to a tag.
- **bunrouter** was measured through its `http.Handler` adapter only. Its native API was not measured.
- **Different harnesses give different absolute values for similar operations**, because they build requests and routes differently. For example, `Mount` with a static prefix measures 346.6 ns in the root suite and 236.8 ns in the upstream waste-hunt harness (AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians; source: `benchstat/root-v110-vs-v130.txt` and `benchstat/wastehunt-v130.txt` in the [campaign archive](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26)). Compare values only within one suite.
- **Module `go` directive.** `v1.1.0` declares `go 1.26` and `v1.3.0` declares `go 1.27.1`. Both were built by the same go1.27.1 compiler. The `v1.1.0` binaries carry `DefaultGODEBUG=tracebacklabels=0,x509sslcertoverrideplatform=0` because of the older `go` directive; the campaign did not test whether these two settings affect the measured paths.
- **Host.** The host is a workstation, not a dedicated benchmark machine. SMT was on and CPU boost was not disabled. Idle database and container services were running; `raw/host-load.log` records the load before every run.
- **Attribution.** The campaign measures differences between versions. It does not attribute any difference to a specific commit.
- **Scope.** One x86-64 CPU model, one operating system, one session. The results do not predict other CPUs, and in particular not Arm hosts.

## Reproduce

The campaign's `run.sh` builds isolated checkouts of both tags, compiles every suite once, runs the 10 interleaved rounds, and produces the `benchstat` outputs. It requires go1.27.1 or later as the local toolchain, the competitor modules in the local module cache (`GOPROXY=off`), and `benchstat` on `PATH`.

```bash
UPSTREAM=/path/to/MuxMaster SCRATCH=/path/to/empty/dir ./run.sh setup
UPSTREAM=/path/to/MuxMaster SCRATCH=/path/to/empty/dir ./run.sh measure
./run.sh stats
python3 tables.py            # regenerates the tables in the archive README
UPSTREAM=/path/to/MuxMaster SCRATCH=/path/to/empty/dir ./run.sh cleanup
```

To compare your own change, run the root benchmarks before and after with at least `-count=10` and compare them with `benchstat`:

```bash
go test -run='^$' -bench=. -benchmem -count=10 . > before.txt
# make your change
go test -run='^$' -bench=. -benchmem -count=10 . > after.txt
benchstat before.txt after.txt
```

## Common questions

<section data-conversation="benchmarks-fastest">

### Which router was fastest in the MuxMaster benchmark campaign?

MuxMaster was the fastest of the five routers measured in six of the eight categories of the upstream competitor suite, not in all of them.

It was fastest on static, 1-parameter, 3-parameter, not-found, parallel static, and parallel 1-parameter routes (with the mode that suits each category). httprouter was fastest on catch-all routes, and on 2-parameter routes pooled MuxMaster and httprouter showed no significant difference. The measured routers were MuxMaster v1.3.0, httprouter v1.3.0, bunrouter v1.0.23 (`http.Handler` adapter), chi v5.3.2, and gorilla/mux v1.8.1, on one AMD Ryzen 9 5900HX host.

### Which MuxMaster mode should I use for the lowest latency?

For parameterised routes, `Handle` with `PoolRequestBundle = true` or `HandleFast` gave the lowest latencies: in the root suite, the default mode took 2.3 to 2.5 times as long as the pooled mode on serial parameterised routes (for example 115.9 ns against 45.84 ns on 1 parameter). Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

The pooled and fast modes require handlers that do not retain the request (or the `Params` slice) after returning; the [Maximum performance guide](/docs/max-performance) explains the contract. Static routes allocate nothing in every mode.

### Did MuxMaster get faster between v1.1.0 and v1.3.0?

Several paths got much faster between v1.1.0 and v1.3.0, but the request hot path did not: 8 of 18 hot-path benchmarks are 1.42% to 6.25% slower in v1.3.0, 3 are faster, and 7 show no significant difference. Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

The large gains are in router features and middleware: automatic `OPTIONS` −42.95%, trailing-slash redirects −23.61%, `ThrottlePerIP` −97.35%, `Logger` −21.98% with zero allocations. Security fixes added cost to `CORS` without an `Origin` header (+304.08%), `Recoverer` (+168.59%), and `BasicAuth` (+63.92% on a valid login). Measured on 2026-09-26 on an AMD Ryzen 9 5900HX with go1.27.1, `-count=10`; source: the benchmark campaign archive [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26).

</section>

## Historical (v1.1.0-era code)

The figures in this section were measured on v1.1.0-era code on other hardware and were not re-measured on the current release. They show how the same benchmarks behave on Arm hosts; they are not comparable with the campaign figures above and do not support any statement about MuxMaster v1.3.0. Values are copied from the upstream platform reports' internal-benchmark tables (`bench_test.go`): median ns/op, B/op, and allocs/op.

### Apple M4 (2026-05-12)

Platform: Apple M4, 10 cores (4 performance + 6 efficiency), 32 GB RAM; macOS Darwin 25.4.0 (arm64); Go version go1.26.2 darwin/arm64; `GOMAXPROCS=10`; `go test -bench=. -benchmem -count=5 -benchtime=3s`, medians of 5 runs; measured 2026-05-12 on v1.1.0-era code.

Serial (single goroutine):

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Static route | 14.1 | 0 | 0 |
| 1 parameter (`Handle`) | 56.7 | 384 | 1 |
| 2 parameters (`Handle`) | 64.2 | 416 | 1 |
| 3 parameters (`Handle`) | 69.7 | 480 | 1 |
| Catch-all (`Handle`) | 58.2 | 384 | 1 |
| Fast static (`HandleFast`) | 14.2 | 0 | 0 |
| Fast 1 param (`HandleFast`) | 28.4 | 32 | 1 |
| Fast 2 params (`HandleFast`) | 36.9 | 64 | 1 |
| Fast 3 params (`HandleFast`) | 45.9 | 96 | 1 |
| Pooled 1 param (`PoolRequestBundle=true`) | 28.4 | 0 | 0 |
| Pooled 2 params (`PoolRequestBundle=true`) | 36.4 | 0 | 0 |
| Pooled 3 params (`PoolRequestBundle=true`) | 38.6 | 0 | 0 |
| Pooled catch-all (`PoolRequestBundle=true`) | 28.5 | 0 | 0 |

Parallel (`GOMAXPROCS=10`):

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Parallel static | 2.4 | 0 | 0 |
| Parallel 1 param (`Handle`) | 82.2 | 384 | 1 |
| Fast parallel 1 param (`HandleFast`) | 13.1 | 32 | 1 |
| Pooled parallel 1 param (`PoolRequestBundle=true`) | 10.2 | 0 | 0 |

Source: [`reports/apple-m4-benchmarks-2026-05-12.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/reports/apple-m4-benchmarks-2026-05-12.md) (upstream MuxMaster, 2026-05-12)

### Raspberry Pi 5 (2026-05-12)

Platform: Raspberry Pi 5 Model B Rev 1.1 — BCM2712 Cortex-A76 @ 2.4 GHz, 4 cores, 16 GB RAM; Debian GNU/Linux 12 (bookworm), Linux 6.12.75+rpt-rpi-2712; Go version 1.26.3 linux/arm64; `go test -bench=. -benchmem -count=5 -benchtime=3s ./...`, medians of 5 consecutive runs; measured 2026-05-12 on v1.1.0-era code. The upstream report groups its internal benchmarks by handler mode rather than into serial and parallel tables; the parallel rows are the rows named "Parallel".

`Handle` (default):

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| Static route | 51.8 | 0 | 0 |
| 1 parameter | 287 | 384 | 1 |
| 2 parameters | 335 | 416 | 1 |
| 3 parameters | 351 | 480 | 1 |
| Catch-all | 282 | 384 | 1 |
| Not found | 594 | 93 | 3 |
| Parallel static | 14.1 | 0 | 0 |
| Parallel 1 param | 184 | 384 | 1 |

`Handle` with `PoolRequestBundle = true`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| 1 parameter | 98.6 | 0 | 0 |
| 2 parameters | 128 | 0 | 0 |
| 3 parameters | 138 | 0 | 0 |
| Catch-all | 100 | 0 | 0 |
| Parallel 1 param | 25.1 | 0 | 0 |

`HandleFast`:

| Benchmark | ns/op | B/op | allocs/op |
|---|---|---|---|
| Static route | 51.6 | 0 | 0 |
| 1 parameter | 149 | 32 | 1 |
| 2 parameters | 218 | 64 | 1 |
| 3 parameters | 242 | 96 | 1 |
| Parallel 1 param | 45.4 | 32 | 1 |

Source: [`reports/rpi5-benchmarks-2026-05-12.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/reports/rpi5-benchmarks-2026-05-12.md) (upstream MuxMaster, 2026-05-12)

## Source

- **Campaign archive (primary source):** [`reports/benchmarks-2026-09-26/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26) in the MuxMasterWebsite repository — raw `go test` output of every run, `benchstat` output of every comparison, exact commands (`run.sh`), host facts, and the [campaign README](https://github.com/FlavioCFOliveira/MuxMasterWebsite/blob/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26/README.md) with the method and caveats.
- **Root-package benchmark suite:** [`bench_test.go` at v1.1.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/bench_test.go) and [`bench_test.go` at v1.3.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/bench_test.go).
- **Perf-audit harness:** [`reports/perf-audit-2026-05-12/` at v1.1.0](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.1.0/reports/perf-audit-2026-05-12) and [at v1.3.0](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/reports/perf-audit-2026-05-12).
- **Middleware benchmarks:** [`middleware/bench_test.go` at v1.3.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/middleware/bench_test.go).
- **Waste-hunt harness:** [`reports/perf-lab-2026-09-24/waste-hunt/` at v1.3.0](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/reports/perf-lab-2026-09-24/waste-hunt).
- **Competitor suite:** `competitor/bench_test.go` from the upstream working tree, which is not part of the v1.3.0 tag (see [Caveats](#caveats)); the exact files used are archived in [`competitor-suite/`](https://github.com/FlavioCFOliveira/MuxMasterWebsite/tree/26abbe6c1cf2f4c9c16af45f4c02377a685f352d/reports/benchmarks-2026-09-26/competitor-suite) with their SHA-256 sums.
- **Historical data:** [`reports/apple-m4-benchmarks-2026-05-12.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/reports/apple-m4-benchmarks-2026-05-12.md) and [`reports/rpi5-benchmarks-2026-05-12.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/reports/rpi5-benchmarks-2026-05-12.md) at v1.3.0.
