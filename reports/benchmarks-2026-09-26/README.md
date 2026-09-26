# Benchmark campaign 2026-09-26

This archive holds the benchmark campaign that the MuxMaster website uses as the primary source of its performance figures (specification rules CS-BENCH-1 to CS-BENCH-18 and INT-PERF-1 to INT-PERF-9). It contains:

- a comparison of MuxMaster `v1.1.0` with MuxMaster `v1.3.0`, run on the same benchmarks;
- the upstream `competitor/` router comparison, run against `v1.3.0`;
- the upstream "Measured changes since v1.1.0" items, re-measured. Where a benchmark exists at both tags, the item is measured at both tags. Where it exists only at `v1.3.0`, only the absolute `v1.3.0` value is given.

All measurements were taken on one host, in one session, on 2026-09-26, with one pinned Go toolchain.

## Contents

| Path | Content |
|---|---|
| `run.sh` | The exact commands: `setup` (isolated tag checkouts and test binaries), `measure`, `stats`, `cleanup`. |
| `raw/*.txt` | Raw `go test` output of every suite, 10 samples per benchmark (concatenation of `raw/rounds/`). |
| `raw/rounds/` | One file per process run (`<suite>-<version>-r<round>.txt`). |
| `raw/run-status.log` | Exit status and last output line of every run. |
| `raw/host-load.log` | Load average and the five largest CPU consumers before every run. |
| `raw/pa-*-clean.txt` | `pa-*.txt` without the log lines that the perf-audit harness writes to stderr (see `strip-log-lines.sh`). |
| `raw/competitor-v130-by-router.txt` | `competitor-v130.txt` with benchmarks renamed to `<Category>/router=<Router>` (see `competitor-rename.sh`), so that `benchstat` can compare routers. |
| `benchstat/*.txt`, `benchstat/csv/*.csv` | `benchstat` output of every comparison, as text and as CSV. |
| `tables.py` | Generates every results table in this file from `benchstat/csv/`. |
| `competitor-suite/` | The competitor suite source files used, with `SHA256SUMS` (see "Competitor suite source"). |

## Host and toolchain

| Fact | Value |
|---|---|
| CPU | AMD Ryzen 9 5900HX with Radeon Graphics |
| Cores / threads | 8 cores, 16 threads (1 socket, SMT active) |
| CPU frequency driver / governor | `amd-pstate-epp` (status `active`); governor `performance` on all 16 CPUs; energy-performance preference `performance` on all 16 CPUs. Verified before the first run and after the last run. |
| Memory | 30 GiB |
| Operating system | Ubuntu 24.04.5 LTS, x86-64 |
| Kernel | Linux 6.8.0-139-generic (`#139-Ubuntu SMP PREEMPT_DYNAMIC`) |
| Go toolchain | `go1.27.1 linux/amd64` (`/usr/local/go`), `GOAMD64=v1` |
| `GOTOOLCHAIN` | `local` (every build and run); `GOPROXY=off`; `GOFLAGS` empty |
| `GOMAXPROCS` | 16 (default) |
| `benchstat` | `golang.org/x/perf/cmd/benchstat` `v0.0.0-20260908200009-22c9c6c9d4da`, built with go1.27.1 |
| Date | 2026-09-26; first run started 18:55:51 +01:00, last run ended 20:13:28 +01:00 |

Every test binary was checked with `go version -m`: all seven were built by go1.27.1.

## Code measured

| Tag | Tag object SHA | Commit SHA |
|---|---|---|
| `v1.1.0` | `1c338013cfe4379d388cfbea9b58bb5a99e35737` | `8af68ecd7b6f35b0f425133d94e8dd6f584248e3` |
| `v1.3.0` | `a5d91649fdec3e783f2b9a611f667c57fbce0365` | `119f2895bf95a29fb2566d875facb1044c6fa9c2` |

Each tag was checked out into its own detached git worktree outside the upstream working tree (`git -C <upstream> worktree add --detach <scratch>/v110 v1.1.0`, and the same for `v1.3.0`). The upstream working tree was not modified. The worktrees were removed at the end with `git worktree remove`.

Suites run:

| Suite | Upstream path | Versions | Benchmarks |
|---|---|---|---|
| Root package | `bench_test.go` | `v1.1.0`, `v1.3.0` | 18 at `v1.1.0`; the same 18 plus 8 new at `v1.3.0`. The 18 shared benchmark bodies are identical at both tags; `v1.3.0` only adds functions. |
| Perf-audit harness | `reports/perf-audit-2026-05-12/` (package in the root module) | `v1.1.0`, `v1.3.0` | 46 at `v1.1.0`; the same 46 plus 2 new at `v1.3.0`. `middleware_bench_test.go` is identical at both tags; `extra_bench_test.go` only adds functions. |
| Middleware package | `middleware/bench_test.go` | `v1.3.0` | 10 functions. The file does not exist at `v1.1.0`. |
| Waste-hunt harness | `reports/perf-lab-2026-09-24/waste-hunt/bench/` (own module, `replace` to the checkout root) | `v1.3.0` | 27 functions. The directory does not exist at `v1.1.0`. |
| Competitor suite | `competitor/bench_test.go` (own module, `replace` to the checkout root) | `v1.3.0` | 53 functions. |

Competitor routers, as linked into the test binary: `github.com/julienschmidt/httprouter v1.3.0`, `github.com/uptrace/bunrouter v1.0.23`, `github.com/go-chi/chi/v5 v5.3.2`, `github.com/gorilla/mux v1.8.1`.

### Competitor suite source

The `v1.3.0` tag does not contain the competitor suite source. Upstream `.gitignore` excludes `competitor/`. The tag tracks only `competitor/vendor/`, which holds a stale copy of MuxMaster. `competitor/go.mod`, `go.sum`, `bench_test.go` and `route_existence_timing_test.go` exist only as untracked files in the upstream working tree. This campaign copied those four files into the `v1.3.0` checkout's `competitor/` directory. They are archived in `competitor-suite/` with their SHA-256 sums. The suite was built with `-mod=mod`, so that `replace github.com/FlavioCFOliveira/MuxMaster => ../` resolved to the `v1.3.0` checkout and not to the vendored copy. `go version -m` on the binary confirms the `=> ../` replacement. The MuxMaster code measured by the competitor suite is therefore `v1.3.0`. The benchmark source itself cannot be pinned to a tag.

## Method

- **Pre-built binaries.** Every suite was compiled once with `go test -c` (see `run.sh setup`). The binaries were then run with `-test.run='^$' -test.bench=. -test.benchmem -test.count=1 -test.benchtime=1s`, from the package directory.
- **Samples.** Each suite was run in 10 rounds of one sample each, so every benchmark has 10 samples (`-count=10` equivalent), and each sample comes from a separate process.
- **Interleaving (v1.1.0 versus v1.3.0).** Each round ran the root package and the perf-audit harness for both versions. Odd rounds ran `v1.1.0` first; even rounds ran `v1.3.0` first (ABBA order), so that slow drift of the host affects both versions equally.
- **Noise floor.** Each round also ran the `v1.3.0` root binary a second time (`root-v130aa`). Comparing the two `v1.3.0` series (an A/A test) measures how often the method reports a difference where none exists.
- **Phase 2.** After the ten comparison rounds, ten rounds ran the competitor suite, the middleware package and the waste-hunt harness, in that order, all at `v1.3.0`.
- **One at a time.** Runs were strictly sequential; nothing else was built or benchmarked during the session. All 80 runs exited with status 0 and printed `PASS` (`raw/run-status.log`).
- **Statistics.** `benchstat` reports the median of the 10 samples with a 95% confidence interval ("± x%"). It compares two series with the Mann-Whitney U test. A delta with p ≥ 0.05 is reported as "no significant difference", without a percentage. benchstat prints p with three decimals: `0.000` means p < 0.0005.
- **Competitor comparisons.** To compare routers with a significance test, `competitor-rename.sh` rewrites `Benchmark<Router><Category>` to `Benchmark<Category>/router=<Router>`, and `benchstat -col /router` compares each router with a chosen baseline. Measured values are not changed. Four baselines were used: MuxMaster default, MuxMaster Pooled, MuxMaster Fast and httprouter.
- **Log lines.** The perf-audit harness logs a `JWTAuth` warning on stderr each time a benchmark constructs the middleware. The warning is printed once per calibration pass, not per iteration, and is emitted at both tags. Because stderr was captured with stdout, some result lines were split. `strip-log-lines.sh` removes the log lines and rejoins the split lines into `raw/pa-*-clean.txt` (460 and 480 result lines, that is 46 × 10 and 48 × 10).

MuxMaster modes in the competitor suite:

- **default**: `Handle` with default settings.
- **Pooled**: `Mux.PoolRequestBundle = true`.
- **Fast**: `HandleFast` without `PoolFastParams`.

bunrouter is measured through its `http.Handler` adapter, as the upstream suite does; its native `bunrouter.HandlerFunc` API was not measured.

## Results

### Noise floor

The `v1.3.0` root binary run twice per round (A/A, 10 samples each) produced **0 significant differences in 40 rows** at alpha = 0.05; the geometric mean of the medians differs by +0.09%. Source: `benchstat/root-v130-aa-noise-floor.txt`.

### Root package: v1.1.0 versus v1.3.0

Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 interleaved rounds), median ± 95% CI. Source: `benchstat/root-v110-vs-v130.txt`.

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

### Root package: benchmarks new in v1.3.0

These benchmarks do not exist at `v1.1.0`; only absolute `v1.3.0` values are given. Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`. Source: `benchstat/root-v110-vs-v130.txt`.

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

### Perf-audit harness: v1.1.0 versus v1.3.0

The benchmark bodies are identical at both tags; each side calls its own version's router and middleware. Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 interleaved rounds), median ± 95% CI. Source: `benchstat/perfaudit-v110-vs-v130.txt`.

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

Benchmarks new in `v1.3.0` (absolute values only):

| Benchmark | time/op | B/op | allocs/op |
|---|---|---|---|
| RedirectTSLWithMiddleware | 805.5 ns ± 1% | 1440 | 11 |
| ParallelRedirectTrailingSlash | 122.2 ns ± 1% | 368 | 2 |

### Measured changes since v1.1.0

This table maps each item of the upstream README's "Measured changes since v1.1.0" list to the benchmark that measures it. Where the benchmark exists at both tags, the `v1.1.0` column holds a real `v1.1.0` measurement. That differs from the upstream list, whose "before" values were measured on 2026-09-24 against pre-change commits. Where the benchmark exists only at `v1.3.0`, the change since `v1.1.0` could not be measured; only the absolute `v1.3.0` value is given. Security-fix costs are listed with the gains (INT-PERF-6). Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, medians. Sources: `benchstat/perfaudit-v110-vs-v130.txt`, `benchstat/root-v110-vs-v130.txt`, `benchstat/wastehunt-v130.txt`.

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

Upstream items not re-measured in this campaign:

- The `-cpu` scaling results from the upstream contention hunt (`ThrottlePerIP` with many clients at 16 CPUs, `ThrottleBacklog` at 1 and 16 CPUs, `RequestID` at 1, 4 and 16 CPUs, `OAuth2Introspect` eviction at 16 CPUs). This campaign ran every benchmark at `GOMAXPROCS=16` only, and did not run the upstream `reports/perf-lab-2026-09-24/harness/` contention benchmarks.
- `BasicAuth` cost by registered-user count has no `v1.1.0` benchmark; the `v1.3.0` values are in the middleware table below.

### Middleware package (v1.3.0 only)

`middleware/bench_test.go` does not exist at `v1.1.0`. Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10`, median ± 95% CI. Source: `benchstat/middleware-v130.txt`.

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

The full waste-hunt harness results, including the upstream pre-change replicas (`*-replica`) and candidate alternatives (`*/alternative`, `*/alt-*`), are in `benchstat/wastehunt-v130.txt`. The replicas are hand-written copies of pre-change code, not `v1.1.0`; they are not used in any table here.

### Competitor suite (v1.3.0)

Caption: AMD Ryzen 9 5900HX, go1.27.1, 2026-09-26, `-count=10` (10 rounds), median ± 95% CI, then B/op and allocs/op. Routers: MuxMaster `v1.3.0` (three modes), httprouter v1.3.0, bunrouter v1.0.23 through its `http.Handler` adapter, chi v5.3.2, gorilla/mux v1.8.1. "Not measured" means the suite has no benchmark for that mode and category. Sources: `benchstat/competitor-v130.txt`, `benchstat/competitor-v130-vs-*.txt`.

| Category | MuxMaster default | MuxMaster Pooled | MuxMaster Fast | httprouter | bunrouter (http.Handler) | chi v5 | gorilla/mux |
|---|---|---|---|---|---|---|---|
| StaticRoute | 28.04 ns ± 2%<br>0 B, 0 allocs | 29.11 ns ± 1%<br>0 B, 0 allocs | 29.10 ns ± 1%<br>0 B, 0 allocs | 34.67 ns ± 1%<br>0 B, 0 allocs | 162.8 ns ± 1%<br>416 B, 3 allocs | 225.2 ns ± 2%<br>368 B, 2 allocs | 576.6 ns ± 1%<br>848 B, 7 allocs |
| ParamRoute1 | 115.0 ns ± 1%<br>384 B, 1 alloc | 46.70 ns ± 3%<br>0 B, 0 allocs | 45.55 ns ± 1%<br>32 B, 1 alloc | 49.67 ns ± 1%<br>64 B, 1 alloc | 160.5 ns ± 1%<br>416 B, 3 allocs | 368.9 ns ± 5%<br>704 B, 4 allocs | 954.0 ns ± 1%<br>1152 B, 8 allocs |
| ParamRoute2 | 132.2 ns ± 1%<br>416 B, 1 alloc | 59.41 ns ± 2%<br>0 B, 0 allocs | 63.42 ns ± 1%<br>64 B, 1 alloc | 59.53 ns ± 1%<br>64 B, 1 alloc | 179.5 ns ± 3%<br>416 B, 3 allocs | 413.7 ns ± 3%<br>704 B, 4 allocs | 1.492 µs ± 1%<br>1168 B, 8 allocs |
| ParamRoute3 | 143.6 ns ± 1%<br>480 B, 1 alloc | 65.17 ns ± 1%<br>0 B, 0 allocs | 83.31 ns ± 2%<br>96 B, 1 alloc | 75.37 ns ± 1%<br>96 B, 1 alloc | 180.2 ns ± 1%<br>416 B, 3 allocs | 428.0 ns ± 4%<br>704 B, 4 allocs | 1.703 µs ± 0%<br>1184 B, 8 allocs |
| WildcardRoute | 114.4 ns ± 1%<br>384 B, 1 alloc | 45.71 ns ± 2%<br>0 B, 0 allocs | 45.61 ns ± 1%<br>32 B, 1 alloc | 42.92 ns ± 1%<br>32 B, 1 alloc | 155.4 ns ± 2%<br>416 B, 3 allocs | 345.0 ns ± 3%<br>704 B, 4 allocs | 1.589 µs ± 0%<br>1152 B, 8 allocs |
| NotFound | 257.3 ns ± 1%<br>107 B, 3 allocs | not measured | not measured | 383.2 ns ± 1%<br>92 B, 3 allocs | 285.6 ns ± 1%<br>145 B, 4 allocs | 354.4 ns ± 1%<br>459 B, 5 allocs | 1.030 µs ± 1%<br>163 B, 4 allocs |
| ParallelStaticRoute | 4.121 ns ± 1%<br>0 B, 0 allocs | not measured | 4.183 ns ± 1%<br>0 B, 0 allocs | 4.902 ns ± 1%<br>0 B, 0 allocs | 129.0 ns ± 3%<br>416 B, 3 allocs | 134.7 ns ± 2%<br>368 B, 2 allocs | 353.1 ns ± 2%<br>849 B, 7 allocs |
| ParallelParamRoute | 104.9 ns ± 1%<br>384 B, 1 alloc | 6.854 ns ± 1%<br>0 B, 0 allocs | 16.24 ns ± 2%<br>32 B, 1 alloc | 22.07 ns ± 2%<br>64 B, 1 alloc | 129.3 ns ± 3%<br>416 B, 3 allocs | 238.5 ns ± 2%<br>704 B, 4 allocs | 469.2 ns ± 2%<br>1153 B, 8 allocs |

Fastest router per category. "Runner-up vs fastest" is the runner-up's time relative to the fastest, from `benchstat` with the fastest as the baseline:

| Category | Fastest (median) | Runner-up (median) | Runner-up vs fastest | p | Fastest non-MuxMaster router |
|---|---|---|---|---|---|
| StaticRoute | MuxMaster default 28.04 ns | MuxMaster Fast 29.10 ns | +3.80% | 0.000 | httprouter 34.67 ns (+23.65% vs fastest, p=0.000) |
| ParamRoute1 | MuxMaster Fast 45.55 ns | MuxMaster Pooled 46.70 ns | +2.51% | 0.001 | httprouter 49.67 ns (+9.03% vs fastest, p=0.000) |
| ParamRoute2 | MuxMaster Pooled 59.41 ns | httprouter 59.53 ns | no significant difference | 0.668 | httprouter 59.53 ns (no significant difference vs fastest, p=0.668) |
| ParamRoute3 | MuxMaster Pooled 65.17 ns | httprouter 75.37 ns | +15.66% | 0.000 | httprouter 75.37 ns (+15.66% vs fastest, p=0.000) |
| WildcardRoute | httprouter 42.92 ns | MuxMaster Fast 45.61 ns | +6.24% | 0.000 | httprouter 42.92 ns (fastest) |
| NotFound | MuxMaster default 257.3 ns | bunrouter (http.Handler) 285.6 ns | +11.00% | 0.000 | bunrouter (http.Handler) 285.6 ns (+11.00% vs fastest, p=0.000) |
| ParallelStaticRoute | MuxMaster default 4.121 ns | MuxMaster Fast 4.183 ns | +1.50% | 0.001 | httprouter 4.902 ns (+18.95% vs fastest, p=0.000) |
| ParallelParamRoute | MuxMaster Pooled 6.854 ns | MuxMaster Fast 16.24 ns | +136.94% | 0.000 | httprouter 22.07 ns (+222.07% vs fastest, p=0.000) |

Every MuxMaster mode compared with httprouter. A positive value means httprouter takes longer than that MuxMaster mode; a negative value means httprouter is faster:

| Category | MuxMaster mode | MuxMaster (median) | httprouter (median) | httprouter vs MuxMaster mode | p |
|---|---|---|---|---|---|
| StaticRoute | default | 28.04 ns | 34.67 ns | +23.65% | 0.000 |
| StaticRoute | Pooled | 29.11 ns | 34.67 ns | +19.08% | 0.000 |
| StaticRoute | Fast | 29.10 ns | 34.67 ns | +19.12% | 0.000 |
| ParamRoute1 | default | 115.0 ns | 49.67 ns | -56.79% | 0.000 |
| ParamRoute1 | Pooled | 46.70 ns | 49.67 ns | +6.36% | 0.001 |
| ParamRoute1 | Fast | 45.55 ns | 49.67 ns | +9.03% | 0.000 |
| ParamRoute2 | default | 132.2 ns | 59.53 ns | -54.97% | 0.000 |
| ParamRoute2 | Pooled | 59.41 ns | 59.53 ns | no significant difference | 0.668 |
| ParamRoute2 | Fast | 63.42 ns | 59.53 ns | -6.14% | 0.000 |
| ParamRoute3 | default | 143.6 ns | 75.37 ns | -47.51% | 0.000 |
| ParamRoute3 | Pooled | 65.17 ns | 75.37 ns | +15.66% | 0.000 |
| ParamRoute3 | Fast | 83.31 ns | 75.37 ns | -9.54% | 0.000 |
| WildcardRoute | default | 114.4 ns | 42.92 ns | -62.48% | 0.000 |
| WildcardRoute | Pooled | 45.71 ns | 42.92 ns | -6.09% | 0.000 |
| WildcardRoute | Fast | 45.61 ns | 42.92 ns | -5.88% | 0.000 |
| NotFound | default | 257.3 ns | 383.2 ns | +48.95% | 0.000 |
| ParallelStaticRoute | default | 4.121 ns | 4.902 ns | +18.95% | 0.000 |
| ParallelStaticRoute | Fast | 4.183 ns | 4.902 ns | +17.19% | 0.000 |
| ParallelParamRoute | default | 104.9 ns | 22.07 ns | -78.96% | 0.000 |
| ParallelParamRoute | Pooled | 6.854 ns | 22.07 ns | +222.07% | 0.000 |
| ParallelParamRoute | Fast | 16.24 ns | 22.07 ns | +35.93% | 0.000 |

## Reading the results

Every statement below is limited to this host, this toolchain, the routers listed above, and the benchmark categories of the upstream competitor suite.

**Competitor suite at v1.3.0**

- **Fastest router per category.** MuxMaster was the fastest measured router in six of the eight categories:
  - Static: default mode, 28.04 ns.
  - 1 parameter: Fast mode, 45.55 ns.
  - 3 parameters: Pooled mode, 65.17 ns.
  - Not found: default mode, 257.3 ns.
  - Parallel static: default mode, 4.121 ns.
  - Parallel 1 parameter: Pooled mode, 6.854 ns.

  In each of these categories, the fastest non-MuxMaster router was slower with p < 0.001.
- **Where MuxMaster was not fastest.** In the catch-all (wildcard) category, httprouter was fastest (42.92 ns). Every MuxMaster mode was slower: Fast by 5.88% (45.61 ns), Pooled by 6.09% (45.71 ns), default 114.4 ns; p < 0.001 for each. In the 2-parameter category, Pooled MuxMaster (59.41 ns) and httprouter (59.53 ns) showed no significant difference (p = 0.668).
- **MuxMaster default mode against httprouter.** The default mode is slower than httprouter on every parameterised route:
  - 1 parameter: 115.0 ns against 49.67 ns.
  - 2 parameters: 132.2 ns against 59.53 ns.
  - 3 parameters: 143.6 ns against 75.37 ns.
  - Catch-all: 114.4 ns against 42.92 ns.
  - Parallel 1 parameter: 104.9 ns against 22.07 ns.

  In these categories the default mode allocates 384–480 B per request (1 allocation); httprouter allocates 32–96 B (1 allocation).
- **MuxMaster Fast mode against httprouter.** Fast mode is slower than httprouter on 2 parameters (63.42 ns against 59.53 ns), 3 parameters (83.31 ns against 75.37 ns) and catch-all (45.61 ns against 42.92 ns). It is faster on static, 1 parameter, parallel static and parallel 1 parameter.
- **MuxMaster Pooled mode against httprouter.** Pooled mode allocates nothing in every parameterised category measured. Compared with httprouter, it is:
  - faster on static, 1 parameter, 3 parameters and parallel 1 parameter;
  - level on 2 parameters (no significant difference);
  - slower on catch-all.
- **Against bunrouter, chi and gorilla/mux.** Each MuxMaster mode was faster than bunrouter (through its `http.Handler` adapter), chi v5 and gorilla/mux in every category where that mode was measured. Each of these comparisons is significant with p < 0.001.

**v1.1.0 against v1.3.0**

- **Root package, allocations.** In the 18 shared root-package benchmarks, B/op and allocs/op are identical between `v1.1.0` and `v1.3.0` in 17. The exception is `NotFound`: 101 → 99 B/op, with allocs/op identical.
- **Root package, time.** Of the 18 shared benchmarks:
  - 7 show no significant difference.
  - 3 are faster at `v1.3.0`: `NotFound` −2.19%, `FastParamRoute2` −9.06%, `FastParamRoute3` −11.55%.
  - 8 are slower at `v1.3.0`, by 1.42% to 6.25%: `StaticRoute`, `ParamRoute1`, `WildcardRoute`, `ParallelStaticRoute`, `FastStaticRoute`, `FastParallelParamRoute`, `PooledParamRoute1`, `PooledParallelParamRoute`.

  The A/A test found no significant difference in any of 40 rows, so these deltas are larger than the noise of this method. This campaign did not establish their cause. It did not separate a code change from a change in code layout or alignment.
- **Perf-audit harness, large gains at v1.3.0:**
  - `ThrottlePerIP`: 4.465 µs → 118.1 ns, 5 → 0 allocations.
  - Automatic `OPTIONS`: −42.95%.
  - Trailing-slash redirect: −23.61%, 13 → 10 allocations.
  - 405: −18.96%, allocations 2 → 1.
  - `Logger`: −21.98%, 4 → 0 allocations.
  - `Compress`: −25.81% on a small body and −63.92% on a large body, 2 → 0 allocations.
  - `JWTAuth` HS256: −11.59% for a valid token, −50.18% for an invalid one.
  - `RequestID` generation: −74.67%.
  - `ThrottleBacklog`: −66.26%.
  - `CleanPath` with a dirty path: −29.53%.
- **Perf-audit harness, costs added since v1.1.0:**
  - `CORS` without an `Origin` header: 20.24 ns → 81.79 ns, 0 → 1 allocation (112 B).
  - `Recoverer` without a panic: 6.502 ns → 17.46 ns.
  - `BasicAuth` with valid credentials: 199.9 ns → 327.6 ns (+63.92%).
  - `BasicAuth` with invalid credentials: +26.66%.

  The chain benchmarks built from these middlewares show the same direction: `Chain_Minimal` (`Recoverer` only) +162.13%, `Chain_AuthBasic` (`BasicAuth` and `Recoverer`) +65.37%.

## Caveats

- **Competitor suite provenance.** The suite source is not part of the `v1.3.0` tag (see "Competitor suite source"). The files used are archived here with their hashes.
- **bunrouter** was measured through its `http.Handler` adapter only. Its native API was not measured, and no claim here applies to it.
- **Different harnesses give different absolute values for similar operations**, because they build requests and routes differently. For example, `Mount` with a static prefix measures 346.6 ns in the root suite and 236.8 ns in the waste-hunt harness. Route registration of 100 routes measures 54.64 µs in the root suite and 62.49 µs in the waste-hunt harness. Compare values only within one suite.
- **Waste-hunt redirect benchmarks.** The waste-hunt harness's `RedirectTrailingSlashWithMiddleware` (7.756 µs) uses a 7-middleware chain that includes `Logger` and `Compress`. It is not the 5-middleware redirect figure that the upstream README quotes; that figure comes from the perf-audit harness's `RedirectTSLWithMiddleware` (805.5 ns here). Upstream's own 2026-09-26 raw output for the waste-hunt benchmark (7 709–7 779 ns) agrees with this campaign's value.
- **Wide confidence intervals.** Some perf-audit rows have confidence intervals of ±20% to ±45% on one or both sides: `Middleware_SetHeader`, `Middleware_WithValue`, `Middleware_RequestID_*`, `Middleware_StripSlashes_Dirty`, `Middleware_NoCache`, `Middleware_CORS_AllowedOrigin`, and `GroupDispatch` and `NestedGroupDispatch` at `v1.1.0`. Where the rank test reports a significant difference for these rows, the direction is supported but the size is uncertain.
- **Module `go` directive.** `v1.1.0` declares `go 1.26` and `v1.3.0` declares `go 1.27.1`. Both were built by the same go1.27.1 compiler. The `v1.1.0` binaries carry `DefaultGODEBUG=tracebacklabels=0,x509sslcertoverrideplatform=0` because of the older `go` directive. This campaign did not test whether those two settings affect the measured paths.
- **Host.** The host is a workstation, not a dedicated benchmark machine. Database and container services (mysqld, mongod, a JVM, redis, dockerd) were running idle. `raw/host-load.log` records load before every run. The per-process CPU figures in that log are lifetime averages from `ps`, so they show that no process was consistently busy, not that the host was idle at every instant. Apart from `ps` itself, the only process at or above 3% in that log was the interactive agent process that drove the session (3.0–3.7%, lifetime average). SMT was on and CPU boost was not disabled.
- **Attribution.** This campaign measures differences between versions. It does not attribute any difference to a specific commit.
- **Scope.** One x86-64 CPU model, one operating system, one session. The results do not predict other CPUs, and in particular not Arm hosts.
- **Upstream figures.** The upstream README's "Measured changes since v1.1.0" before values (2026-09-24, pre-change commits, not `v1.1.0`) are not comparable with the `v1.1.0` column here and are not used.

## Reproduction

```bash
UPSTREAM=/path/to/MuxMaster SCRATCH=/path/to/empty/dir ./run.sh setup
UPSTREAM=/path/to/MuxMaster SCRATCH=/path/to/empty/dir ./run.sh measure
./run.sh stats
python3 tables.py            # regenerates the tables in this README
UPSTREAM=/path/to/MuxMaster SCRATCH=/path/to/empty/dir ./run.sh cleanup
```

`setup` requires go1.27.1 or later as the local toolchain, the competitor modules in the local module cache (`GOPROXY=off`), and `benchstat` on `PATH`.
