---
datePublished: 2026-09-26
---

# Built with MuxMaster

This website is a Go program that uses **MuxMaster as its only HTTP router**. Every page, Markdown companion, text file, and static asset you request here is matched by a `*muxmaster.Mux` and served by a MuxMaster handler. This page shows the configuration the site uses, explains why each setting is safe here, and reports what the configuration costs per request, measured with the benchmarks in this website's repository.

The source code is public at [github.com/FlavioCFOliveira/MuxMasterWebsite](https://github.com/FlavioCFOliveira/MuxMasterWebsite). Every code excerpt below is copied verbatim from it.

## How the site serves a request

The site is **pre-rendered**: at startup, the server renders every public route to bytes once and keeps them in memory. At request time, no template runs and no file is read from disk. The router finds the route, the handler selects the stored bytes, and the server writes them.

Three MuxMaster features carry this design:

* **Both pooling opt-ins are on.** `PoolRequestBundle` and `PoolFastParams` recycle the per-request objects that the router would otherwise allocate.
* **All cross-cutting middleware runs through `Pre`.** `Pre` middleware runs once per request, before route matching, for every route type — including `HandleFast` routes.
* **Static assets use a `FastHandler`.** The `/static/*filepath` route receives its parameter as a function argument instead of through the request context.

## Step 1 — Turn on both pools

The router enables both of MuxMaster's pooling opt-ins:

```go
	m.PoolRequestBundle = true
	m.PoolFastParams = true
```

Pooling is safe only when no handler keeps the request, its context, its body, or the `Params` slice after it returns. The site meets that **lifetime contract**: its handlers write stored bytes and return, and none starts a goroutine. The audit uses the `grep` checks from [Maximum performance](/docs/max-performance#auditing-your-handlers); they find no handler goroutine, no `ReverseProxy`, and no captured request.

`PoolRequestBundle` recycles the request copy that **parameterised** `Handle` routes receive. Every `Handle` route on this site is static, and static routes never allocate that copy, so this opt-in has no measurable effect here today. It stays on so that any parameterised route added later starts pool-backed.

## Step 2 — Run every policy in `Pre`

The server registers all of its middleware with `Pre`:

```go
	m.Pre(mwm.RecovererWithLogger(logger))
	m.Pre(mwm.RequestID())
	if len(cfg.TrustedProxyCIDRs) > 0 {
		// Pass each prefix by address — RealIP's variadic signature
		// expects *netip.Prefix. Empty list means no proxy is trusted
		// and we skip RealIP entirely so r.RemoteAddr is the raw peer.
		prefixes := make([]*netip.Prefix, len(cfg.TrustedProxyCIDRs))
		for i := range cfg.TrustedProxyCIDRs {
			prefixes[i] = &cfg.TrustedProxyCIDRs[i]
		}
		m.Pre(mwm.RealIP(prefixes...))
	}
	m.Pre(slogAccessLog(logger))
	m.Pre(securityHeaders)
	// URL normalisation redirects (specification/url-and-versioning.md).
	// Runs after security headers so a 301 still carries the same
	// hardening as a 200, and before route matching so the canonical path
	// is what the router sees.
	m.Pre(normalisationRedirects)
```

`Recoverer`, `RequestID`, and `RealIP` come from MuxMaster's `middleware` package. The site never calls `Use`: MuxMaster panics when a `HandleFast` route is registered after a `Use` middleware, because `Use` middleware does not run on the fast path and a route would silently skip its policy. With `Pre` only, the same chain protects every route.

There is **no compression middleware**. Every response body is compressed once, at startup, so compressing it again per request would only repeat work.

## Step 3 — Serve pre-computed responses

At startup, the server stores each page as a `render.Response`: the identity body, a gzip body (kept only when it is smaller), one strong `ETag` per body, and every header value as a ready-made `[]string`. At request time, `Serve` assigns those values straight into the header map:

```go
func (p *Response) Serve(w http.ResponseWriter, r *http.Request) {
	rep := &p.identity
	if p.gzip != nil && AcceptsGzip(r.Header["Accept-Encoding"]) {
		rep = p.gzip
	}

	h := w.Header()
	h["Etag"] = rep.etagHdr
	h["Cache-Control"] = p.cacheControl
	if p.lastModified != nil {
		h["Last-Modified"] = p.lastModified
	}
```

Assigning a stored slice under the canonical header key avoids the allocation that `Header().Set` makes on every call. `Serve` allocates nothing; a unit test in the repository checks this.

## Step 4 — Serve static assets with `GETFast`

The static directory is loaded into memory at startup, with the same pre-computed responses. The route is registered for `GET` and `HEAD` with MuxMaster's fast-path API:

```go
	const staticPattern = "/static/*filepath"
	staticHandler := withRouteFast(staticPattern, s.static.handler(notFound))
	m.GETFast(staticPattern, staticHandler)
	m.HEADFast(staticPattern, staticHandler)
```

The handler reads the `filepath` parameter from its third argument and looks it up in a map that holds only the files loaded at startup. A directory, an unknown name, or a path such as `/static/../go.mod` is simply absent from the map and receives the 404 page:

```go
func (a *staticAssets) handler(notFound http.HandlerFunc) muxm.FastHandler {
	return func(w http.ResponseWriter, r *http.Request, ps muxm.Params) {
		resp, ok := a.files[ps.Get("filepath")]
		if !ok {
			notFound(w, r)
			return
		}
		resp.Serve(w, r)
	}
}
```

With `PoolFastParams` on, MuxMaster returns the `Params` slice to its pool when the handler returns. The handler reads `ps` only during the call, so the slice is never used after it is recycled.

## What a request costs

The table compares the site **before** and **after** this configuration. Both columns use the same benchmark: `make bench` drives a request through the site's complete handler (the `Pre` chain, the router, and the handler) inside one process, with the production logger configuration (JSON at level `Info`, written to `io.Discard`). Values are the median of six runs.

| Request | Time before | Time after | Memory before | Memory after | Allocations before | Allocations after |
|---|---:|---:|---:|---:|---:|---:|
| `/healthz` | 10.0 µs | 9.6 µs | 798 B | 626 B | 15 | 4 |
| Documentation page, no compression | 10.5 µs | 9.7 µs | 855 B | 627 B | 17 | 5 |
| Documentation page, gzip | 82.2 µs | 9.9 µs | 1,054 B | 626 B | 19 | 4 |
| Documentation page, `304 Not Modified` | 10.9 µs | 9.8 µs | 892 B | 627 B | 18 | 4 |
| Home page, gzip | 188.1 µs | 9.8 µs | 1,090 B | 626 B | 19 | 5 |
| Markdown companion, gzip | 11.0 µs | 9.8 µs | 893 B | 626 B | 18 | 5 |
| CSS bundle, gzip | 214.4 µs | 9.9 µs | 39,442 B | 627 B | 40 | 5 |
| 404 page, gzip | 48.9 µs | 9.9 µs | 1,034 B | 627 B | 17 | 5 |

Measured on 26 September 2026 on an AMD Ryzen 9 5900HX, Linux 6.8.0-139-generic, Go 1.27.1.

**Allocations** are the most reliable column: they do not depend on the machine. After the change, every request costs **4 or 5 allocations**, and none of them comes from the site's own code: `middleware.RequestID` makes two (its context value and the request copy that carries it), and the `log/slog` access-log line makes the rest.

**Times** on this machine are dominated by its clock. The kernel uses the `hpet` clock source, on which one `time.Now()` call costs about 2.8 µs, and each request reads the clock three times (the access log's start time, its duration, and the log record's timestamp). About 8.5 µs of the roughly 10 µs per request is therefore clock cost.

The largest gains come from computing the gzip bodies at startup. Before the change, the home page was compressed on every request (188.1 µs); the CSS bundle was read from disk and compressed on every request (214.4 µs, 39,442 B, 40 allocations).

## Reproduce the numbers

Clone the website's repository and run:

```sh
make bench
```

The target runs `go test ./internal/server -run '^$' -bench . -benchmem -count=6`. To compare two versions of the code, save the output of each run and compare them with `benchstat`.

## Common questions

<section data-conversation="built-with-muxmaster">

### Does this website run on MuxMaster?

Yes. Every route of this website is registered on a `*muxmaster.Mux`, which is the server's only `http.Handler`.

### Which MuxMaster performance features does the site use?

It uses `PoolRequestBundle`, `PoolFastParams`, `Pre` middleware, and a `HandleFast` route (`GETFast` and `HEADFast` on `/static/*filepath`). It does not use `Use` middleware, so `HandleFast` routes can be registered.

### How many allocations does a request to this website cost?

Four or five, with production logging on, for every type of request the benchmarks cover. Two come from `middleware.RequestID` and the rest from the `log/slog` access log; the router, the handlers, and the site's own middleware allocate nothing per request.

### Is pooling safe for this website?

Yes, because no handler keeps the request, its context, its body, or the `Params` slice after it returns, and no handler starts a goroutine. The audit that checks this is described in [Maximum performance](/docs/max-performance#auditing-your-handlers).

</section>

## Related pages

* [Maximum performance](/docs/max-performance): the pooling opt-ins, the lifetime contract, and the handler audit.
* [Maximum-performance example](/examples/max-performance): a runnable program that enables every opt-in.
* [Benchmarks](/benchmarks): MuxMaster's own benchmarks and the comparison with other routers.
