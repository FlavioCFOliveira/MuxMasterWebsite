---
datePublished: 2026-05-08
---

# MuxMaster

A radix-tree HTTP router for Go. Zero dependencies, O(k) lookups.

MuxMaster routes 25 ns on static paths and 112 ns on a single parameter (AMD Ryzen 9 5900HX, Go 1.26.2), allocates zero bytes on static routes, and is 100% compatible with the `net/http` handler interface. Build production services on the Go standard library.

## Highlights

- **Zero dependencies.** The router and the 17 bundled middlewares are implemented on the Go standard library alone. `go.mod` declares no `require` beyond the test fixtures.
- **Radix-tree, allocation-free hot path.** Static-route lookups are 25 ns / 0 allocs; one-parameter routes are 112 ns / 1 alloc for a single 416-byte tiered request bundle.
- **100% `net/http` compatible.** Handlers stay `http.Handler`; middleware stays `func(http.Handler) http.Handler`. Adopt incrementally — your existing handlers compile unchanged.
- **Typed errors and parameters.** Optional `HandlerFuncE` threads errors through middleware. `Params.Int`, `Params.Bool`, `Params.UUID` parse and validate path parameters in one call.
- **Production-grade middleware.** `RequestID`, `Recoverer`, `Logger`, `Compress`, `RealIP`, `Timeout`, `Throttle`, `BasicAuth`, `JWTAuth`, `OAuth2Introspect`, `APIKey`, `CORS` — all hardened and audited.
- **Dogfooded.** This documentation site is itself served by a Go binary using MuxMaster as its router. The router is the documentation and the proof.

## Quick links

- [Getting started](/docs/getting-started)
- [API reference](/api)
- [Benchmarks](/benchmarks)
- [Examples](/examples/)
- [Source on GitHub](https://github.com/FlavioCFOliveira/MuxMaster)

Released as v1.0.1. MIT-licensed. Requires Go 1.26+.

## Frequently asked questions

<section data-conversation="landing-faq">

### What is MuxMaster?

MuxMaster is a high-performance, zero-dependency HTTP router for Go. It implements a radix tree with O(k) lookups, allocates zero bytes on the static-route hot path, and is fully compatible with the `net/http` `Handler` interface so existing handlers compile unchanged.

### What Go version does MuxMaster require?

Go 1.26 or later. The minimum is set by the `go` directive in the upstream `go.mod`; see [Compatibility](/compatibility) for the full version policy.

### Is MuxMaster compatible with `net/http`?

100%. Handlers remain `http.Handler` and middleware remains `func(http.Handler) http.Handler`. A `*muxmaster.Mux` is a drop-in replacement for `http.ServeMux` everywhere the standard library accepts an `http.Handler`, so adoption is incremental: a single route, a single sub-tree, or a whole service.

### How fast is MuxMaster?

Static-route lookups complete in 25 ns with zero allocations; one-parameter routes complete in 112 ns with a single 416-byte allocation for the tiered request bundle. Numbers measured on AMD Ryzen 9 5900HX, Go 1.26.2; see [Benchmarks](/benchmarks) for the full table and the reproduce instructions.

### What is MuxMaster's license?

MIT. The full text is in the upstream repository at [LICENSE](https://github.com/FlavioCFOliveira/MuxMaster/blob/main/LICENSE). MIT permits commercial use, modification, distribution, and private use; the only requirement is preserving the copyright notice.

### How does MuxMaster compare to chi, gin, gorilla/mux, and httprouter?

MuxMaster keeps the `net/http` handler signature (unlike `gin`, which introduces `gin.Context`) and ships zero external dependencies (unlike `chi`, which depends on nothing but `gorilla/mux`, which depends on `gorilla/context`). It matches `httprouter`'s static-route latency while exposing a higher-level API (groups, typed parameters, error-returning handlers). See the [migration guide](/docs/migration) for side-by-side equivalents.

</section>
