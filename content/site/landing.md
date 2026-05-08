# MuxMaster

A radix-tree HTTP router for Go. Zero dependencies, O(k) lookups.

MuxMaster routes 25 ns on static paths and 112 ns on a single parameter (AMD Ryzen 9 5900HX, Go 1.26.2), allocates zero bytes on static routes, and is 100% compatible with the `net/http` handler interface. Build production services on the Go standard library.

## Highlights

- **Zero dependencies.** The router and the 19 bundled middlewares are implemented on the Go standard library alone. `go.mod` declares no `require` beyond the test fixtures.
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
