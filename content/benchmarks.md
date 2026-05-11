---
datePublished: 2026-05-08
---

# Benchmarks

> This page reflects the upstream `README.md` `## Benchmarks` section as of **2026-05-08** at upstream commit **`7827183`** (release **v1.0.1**). Numbers are reproduced verbatim from the upstream source. To re-run the suite, follow the reproduce instructions below.

Benchmarks run on AMD Ryzen 9 5900HX, Go 1.26.2. All measurements use the same route set (`/api/v1/...`).

| Route type          | MuxMaster               | httprouter              | chi v5                  |
|---------------------|-------------------------|-------------------------|-------------------------|
| Static              | **25 ns, 0 allocs**     | 33.8 ns, 0 allocs       | 213.5 ns, 2 allocs      |
| 1 parameter         | 112 ns, 1 alloc         | **56.4 ns, 1 alloc**    | 354.1 ns, 4 allocs      |
| 2 parameters        | 130 ns, 1 alloc         | **66.5 ns, 1 alloc**    | 402.2 ns, 4 allocs      |
| 3 parameters        | 141 ns, 1 alloc         | **78.4 ns, 1 alloc**    | 410.2 ns, 4 allocs      |
| Catch-all           | 109 ns, 1 alloc         | **51.3 ns, 1 alloc**    | 330.2 ns, 4 allocs      |
| Parallel static     | **3.7 ns, 0 allocs**    | 4.92 ns, 0 allocs       | 128.2 ns, 2 allocs      |
| Parallel 1 param    | 108 ns, 1 alloc         | **22.2 ns, 1 alloc**    | 223.9 ns, 4 allocs      |

## Reproduce

```bash
go test -bench=. -benchmem ./...
```

## Notes

- MuxMaster allocates **zero bytes** for static routes and **one tiered allocation** (416–480 B) for parameterized routes. That single allocation fuses the copied `*http.Request` and its context — meaning the router, `net/http`, and your handler all share one GC object.
- httprouter's 1 alloc for parameterized routes is only a 64 B `Params` slice; it passes parameters via a third argument outside the `http.Handler` interface, requiring a different handler signature.
- MuxMaster is **100% `net/http` compatible** — it accepts `http.Handler` directly, works with all existing middleware ecosystems, and requires no handler signature changes.

## Source

[`README.md` `## Benchmarks` at v1.0.1](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/README.md#benchmarks)
