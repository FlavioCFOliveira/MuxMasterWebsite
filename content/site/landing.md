---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# MuxMaster

MuxMaster is an HTTP router for Go built on a radix tree: route lookup costs O(k) in the length of the URL path, static routes allocate nothing, and every handler and middleware keeps the standard `net/http` signatures. It has zero external dependencies, supports the HTTP QUERY method (RFC 10008), and requires Go 1.27.1 or later. The current release is v1.3.0.

## Highlights

- **HTTP QUERY method (RFC 10008).** QUERY is a safe, idempotent method, like GET, that carries its query in the request body, like POST. MuxMaster has supported it since v1.2.0, with `MethodQuery`, `Mux.QUERY`, `Mux.QUERYE`, `Mux.QUERYFast`, `Group.QUERY`, and `Group.QUERYE`; `ANY` includes QUERY, the `Allow` header lists it, and automatic redirects use `307` so the method and body are preserved. Read the [HTTP QUERY method (RFC 10008)](/docs/http-query-method) guide.
- **`net/http` compatible, zero external dependencies.** A `*muxmaster.Mux` is an `http.Handler`, handlers are `http.HandlerFunc`, and middleware is `func(http.Handler) http.Handler`, so existing handlers and middleware work unchanged. The router and the `middleware` package use only the Go standard library.
- **Allocation-aware routing.** Static routes allocate nothing. By default a request to a route with path parameters makes one allocation; with the opt-in `PoolRequestBundle`, parameterised routes allocate nothing, under a stricter handler lifetime contract. See the [Maximum performance guide](/docs/max-performance).
- **Rich path patterns.** Named (`:id`), regex-constrained (`{id:[0-9]+}`), optional (`{/:id}`), and catch-all (`*filepath`) parameters, with typed accessors for `int`, `int64`, `uint64`, `float64`, and `bool`.
- **Middleware and error-returning handlers.** `Pre` runs before routing, for every request; `Use` wraps standard routes; `UseFast` wraps `FastHandler` routes. The `middleware` package provides 21 middleware constructors, including logging, panic recovery, CORS, Basic Auth, API keys, JWT, OAuth 2.0 introspection, compression, throttling, timeouts, and request IDs. `HandlerFuncE` handlers return an `error`, and one `ErrorHandler` turns errors into responses.

## Performance, measured

This website's benchmark campaign of 2026-09-26 measured MuxMaster v1.3.0 against httprouter v1.3.0, bunrouter v1.0.23 (through its `http.Handler` adapter), chi v5.3.2, and gorilla/mux v1.8.1 on an AMD Ryzen 9 5900HX with go1.27.1 (`-count=10`, medians, differences tested with `benchstat` at alpha = 0.05; source: [Benchmarks](/benchmarks)). Among these five routers:

- MuxMaster was the fastest in six categories, each in one of its three modes:
  - Default mode: static, not-found, and parallel static routes (for example 28.04 ns on a static route).
  - `HandleFast`: 1-parameter routes.
  - `PoolRequestBundle`: 3-parameter and parallel 1-parameter routes (for example 6.854 ns on the parallel 1-parameter benchmark).
- httprouter was the fastest on catch-all routes (42.92 ns, against 45.61 ns for MuxMaster's `HandleFast` mode).
- On 2-parameter routes, MuxMaster with `PoolRequestBundle` and httprouter showed no significant difference.
- MuxMaster's default mode, which allocates one request bundle per parameterised request, was slower than httprouter on every parameterised route.

The same campaign compared v1.1.0 with v1.3.0 on the same AMD Ryzen 9 5900HX host with go1.27.1, on 2026-09-26, with `-count=10` (source: [Benchmarks](/benchmarks)). Of the 18 root-package benchmarks present in both versions, every one makes the same number of allocations; 3 became faster, 7 showed no significant difference, and 8 became 1.42% to 6.25% slower. Automatic `OPTIONS`, redirects, and several middleware became much faster, and some behaviour changes added cost. Every table, the method, and the caveats are on the [Benchmarks](/benchmarks) page.

This website runs on MuxMaster v1.3.0 with `PoolRequestBundle` and `PoolFastParams` enabled; [Built with MuxMaster](/built-with-muxmaster) shows its configuration and what each request costs.

## What's new

- **v1.3.0 (2026-09-26):** Go 1.27.1 is now the minimum version; the default `OAuth2Introspect` client no longer exhausts ephemeral ports under load; `Group.ServeFiles` applies the same raw-path security guard as `Mux.ServeFiles`. The exported API is unchanged. [Release notes v1.3.0](/releases/v1.3.0).
- **v1.2.0 (2026-09-26):** HTTP QUERY method (RFC 10008); `Mount` fixes for nested routers; routing fixes (no empty-segment parameter matches, correct group prefix joins); behaviour changes to redirects and to `BasicAuth`, `CORS`, `OAuth2Introspect`, and `Recoverer`; route registration that copies O(depth) instead of O(tree size). [Release notes v1.2.0](/releases/v1.2.0).

## Quick links

- [Getting started](/docs/getting-started)
- [HTTP QUERY method (RFC 10008)](/docs/http-query-method)
- [API reference](/api)
- [Benchmarks](/benchmarks)
- [Maximum performance guide](/docs/max-performance)
- [Examples](/examples/)
- [Built with MuxMaster](/built-with-muxmaster)
- [Source on GitHub](https://github.com/FlavioCFOliveira/MuxMaster)

MuxMaster v1.3.0 is MIT-licensed and requires Go 1.27.1 or later.

## Frequently asked questions

<section data-conversation="landing-faq">

### What is MuxMaster?

MuxMaster is a zero-dependency HTTP router for Go that uses a radix tree for O(k) route lookups and keeps full compatibility with the `net/http` `Handler` interface.

Static routes allocate nothing, parameterised routes make one allocation by default or none with the opt-in `PoolRequestBundle`, and the `middleware` package provides 21 middleware constructors.

### What Go version does MuxMaster require?

MuxMaster v1.3.0 requires Go 1.27.1 or later, as declared by the `go` directive in its `go.mod`.

With the default `GOTOOLCHAIN=auto`, an older Go toolchain switches to Go 1.27.1 or newer automatically. See [Compatibility](/compatibility) for the version policy.

### Does MuxMaster support the HTTP QUERY method?

Yes: MuxMaster has supported the HTTP QUERY method defined in RFC 10008 since v1.2.0, through `MethodQuery` and the `QUERY`, `QUERYE`, and `QUERYFast` registration methods.

The [HTTP QUERY method (RFC 10008)](/docs/http-query-method) guide shows how to register a QUERY route, validate its request body, and call it with `curl`.

### Is MuxMaster compatible with `net/http`?

Yes: a `*muxmaster.Mux` implements `http.Handler`, handlers use the `http.HandlerFunc` signature, and middleware uses `func(http.Handler) http.Handler`.

Any code that accepts an `http.Handler`, such as `http.Server` or `httptest.NewServer`, accepts the router, so adoption can be incremental.

### How fast is MuxMaster compared with other routers?

In this website's 2026-09-26 benchmark campaign, MuxMaster v1.3.0 was the fastest of five measured Go routers (MuxMaster, httprouter, bunrouter, chi, and gorilla/mux) in six of the eight route categories of the upstream competitor suite, and httprouter was the fastest on catch-all routes.

MuxMaster led on static, not-found, and parallel static routes in its default mode, on 1-parameter routes with `HandleFast`, and on 3-parameter and parallel 1-parameter routes with `PoolRequestBundle`; on 2-parameter routes, MuxMaster with `PoolRequestBundle` and httprouter showed no significant difference. MuxMaster's default mode was slower than httprouter on every parameterised route. The full data, host, and method are on the [Benchmarks](/benchmarks) page.

### What is the catch with `PoolRequestBundle`?

`PoolRequestBundle` recycles the per-request bundle through `sync.Pool`, so handlers must not retain `*http.Request` after they return, for example in a goroutine that outlives the handler.

Reverse proxies built on `net/http.Transport` and handlers that hijack the connection must keep the pool off. The [Maximum performance guide](/docs/max-performance) lists the rules and an audit checklist.

### What is MuxMaster's license?

MuxMaster is released under the MIT License.

The full text is in the upstream repository at [LICENSE](https://github.com/FlavioCFOliveira/MuxMaster/blob/main/LICENSE).

</section>
