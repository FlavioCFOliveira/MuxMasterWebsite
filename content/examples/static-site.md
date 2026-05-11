---
datePublished: 2026-05-08
---

# Static site example

A documentation static-site server that exercises the full HTTP vocabulary for static delivery: conditional GETs (304 via `ETag` / `Last-Modified`), range requests (206 partial content), HEAD, OPTIONS / CORS, gzip via `Accept-Encoding`, security headers, rate-limited backlog, RealIP behind a reverse proxy, clean-path normalisation, custom HTML 404, redirect chains, sub-handler mount for versioned documentation, and route inspection. Reach for this example when serving any non-trivial static asset tree.

## Step 1 — Construct the router and configure dispatch flags

The four flags on `*Mux` declare how the dispatcher behaves on edge cases the static-site path encounters all the time: trailing-slash redirects, method-not-allowed responses, and OPTIONS handling for CORS preflight.

```go
r := mm.New()

r.RedirectTrailingSlash = true
r.RedirectFixedPath = false
r.HandleMethodNotAllowed = true
r.HandleOPTIONS = true
```

`RedirectTrailingSlash` matters because `http.FileServer` requires a trailing slash on directories — keeping both layers in agreement avoids a redirect loop.

## Step 2 — Wire the themed 404, method-not-allowed, and global OPTIONS handlers

`r.NotFound`, `r.MethodNotAllowed`, and `r.GlobalOPTIONS` are the override slots for the dispatcher's defaults. The static-site uses themed HTML 404, a plain-text method-not-allowed (browsers never POST to a static file), and an OPTIONS handler that answers CORS preflight for every registered route in one place.

```go
r.NotFound = http.HandlerFunc(serveNotFound(staticRoot))

r.MethodNotAllowed = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Header().Set("Allow", w.Header().Get("Allow")) // already set by the router
    http.Error(w, fmt.Sprintf("method %s not allowed", req.Method), http.StatusMethodNotAllowed)
})

r.GlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", w.Header().Get("Allow"))
    w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Encoding, Range")
    w.Header().Set("Access-Control-Max-Age", "86400")
    w.WriteHeader(http.StatusNoContent)
})
```

`Access-Control-Max-Age: 86400` tells browsers to cache the preflight result for a day, eliminating the per-request OPTIONS round-trip for repeated cross-origin reads.

## Step 3 — Normalise paths in `Pre`

`CleanPath` runs before the radix tree resolves the URL — it collapses `//`, resolves `..` segments, and drops any path-traversal attempt before the dispatcher sees it. Belongs in `Pre` because the cleaned path is the one that gets matched.

```go
r.Pre(mw.CleanPath())
```

This also closes a double-hit cache vector: `/foo` and `//foo` would otherwise be two distinct keys with the same body.

## Step 4 — Apply the global middleware stack

Five layers of cross-cutting concerns: `RealIP` (rewrite `RemoteAddr` from `X-Forwarded-For` when a trusted proxy is in front), `RequestID` (correlation across logs), `Logger` (structured access log), `Recoverer` (panic safety), `ThrottleBacklog` (concurrency cap with bounded queue), `Compress` (gzip on text/* and application/* responses ≥ 1 kB), and four security headers (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Permissions-Policy`).

```go
r.Use(
    mw.RealIP(&loopback, &private10, &private172, &private192),
    mw.RequestID(),
    mw.Logger(os.Stdout),
    mw.RecovererWithLogger(log),
    mw.ThrottleBacklog(500, 200, 8*time.Second),
    mw.Compress(gzip.BestSpeed),
    mw.SetHeader("X-Content-Type-Options", "nosniff"),
    mw.SetHeader("X-Frame-Options", "SAMEORIGIN"),
    mw.SetHeader("Referrer-Policy", "strict-origin-when-cross-origin"),
    mw.SetHeader("Permissions-Policy", "camera=(), microphone=(), geolocation=()"),
)
```

`ThrottleBacklog(500, 200, 8s)` is the canonical guard against accidental DDoS from scrapers or CI load tests: 500 in-flight requests with a 200-request backlog, 8-second queue timeout. Returns 503 when the backlog overflows.

## Step 5 — Register the FastHandler health probe (GET + HEAD)

Health probes are called thousands of times per second by load balancers and operators. `GETFast` registers the route on the `FastHandler` path so the request resolves with zero allocations, and `HEADFast` covers tools that probe with HEAD only.

```go
r.GETFast("/health", func(w http.ResponseWriter, _ *http.Request, _ mm.Params) {
    w.Header().Set("Content-Type", "application/json")
    _, _ = io.WriteString(w, `{"status":"ok"}`)
})

r.HEADFast("/health", func(w http.ResponseWriter, _ *http.Request, _ mm.Params) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
})
```

`io.WriteString` on a literal avoids both the `fmt` package's reflection cost and the byte-slice allocation `[]byte(s)` would imply.

## Step 6 — Wire a no-cache JSON API endpoint via `With()`

`With()` returns a group with extra middleware applied only to its routes. The example uses it for a small JSON config endpoint that must NEVER be cached: `NoCache` writes the headers (`Cache-Control: no-store, no-cache, must-revalidate`, `Pragma: no-cache`), `Timeout` bounds the handler.

```go
api := r.With(mw.NoCache(), mw.Timeout(10*time.Second))
api.GET("/api/config", serveConfig)
```

`With()` is the right choice when only a handful of routes need the extra layer; for a larger surface use `r.Group("/api").Use(...)` instead.

## Step 7 — Serve HTML pages with `no-cache` so updates land immediately

`With(SetHeader("Cache-Control", "no-cache, must-revalidate"))` is the canonical policy for HTML: the browser keeps the page but revalidates with the server on every navigation, picking up new content without a hard refresh.

```go
pages := r.With(
    mw.SetHeader("Cache-Control", "no-cache, must-revalidate"),
)

pages.GET("/", serveFile(staticRoot, "/index.html"))
pages.HEAD("/", serveFile(staticRoot, "/index.html"))
```

`http.FileServer` (under `serveFile`) handles `ETag`, `Last-Modified`, `If-None-Match`, `If-Modified-Since`, range requests, and HEAD transparently — there is no need to write any of that by hand.

## Step 8 — Redirect old URL shapes to the canonical docs path

`mm.Redirect` writes the redirect with the supplied status code. 301 is the right choice for "moved permanently" — search engines and intermediate caches transfer their signal from the legacy URL to the canonical one.

```go
r.GET("/doc", func(w http.ResponseWriter, req *http.Request) {
    mm.Redirect(w, req, http.StatusMovedPermanently, "/docs/v2/")
})
r.GET("/documentation", func(w http.ResponseWriter, req *http.Request) {
    mm.Redirect(w, req, http.StatusMovedPermanently, "/docs/v2/")
})
```

When a redirect needs to be temporary (A/B test, maintenance redirect), use 302 / 307 instead — search engines preserve signal on the original URL.

## Step 9 — Mount versioned documentation sub-routers

`r.Mount(prefix, handler)` attaches a sub-handler at a prefix and strips the prefix before forwarding. Each docs-version sub-mux runs its own middleware (cache policy, robots tag, throttle) and serves its own file tree.

```go
docsV1 := buildDocsMux(http.Dir("./static/docs/v1"))
docsV2 := buildDocsMux(http.Dir("./static/docs/v2"))
r.Mount("/docs/v1", docsV1)
r.Mount("/docs/v2", docsV2)
```

After mount, a request to `/docs/v1/index.html` arrives at `docsV1` as `/index.html`, which is what `http.FileServer` rooted at `./static/docs/v1` expects to see.

## Step 10 — Serve fingerprinted assets with immutable cache + CORS

Versioned assets (e.g. `/assets/style.abc123.css`) carry their content hash in the URL — when the body changes, the URL changes too. The `immutable` directive plus `max-age=31536000` (one year) tells browsers and CDNs to keep the asset until the URL stops being referenced, eliminating revalidation traffic for assets that cannot change behind their URL.

```go
assetsGroup := r.With(
    mw.SetHeader("Cache-Control", "public, max-age=31536000, immutable"),
    mw.CORS(mw.CORSOptions{
        AllowedOrigins: []string{"*"},
        AllowedMethods: []string{http.MethodGet, http.MethodHead},
        AllowedHeaders: []string{"Accept-Encoding", "Range"},
        ExposedHeaders: []string{"Content-Length", "Content-Range", "ETag"},
        MaxAge:         86400,
    }),
)

assetsGroup.ServeFiles("/assets/*filepath", staticRoot)
```

`ServeFiles` registers GET and HEAD for the catch-all pattern; `http.FileServer` handles ETag, range, conditional GET, and HEAD transparently. Per `SECURITY.md` CDX-S8-002, `ServeFiles` refuses to register when the mux is configured with both `UseRawPath=true` and `UnescapePathValues=true` simultaneously — both stay at default to let `net/http` canonicalise the path before dispatch.

## Step 11 — Expose route introspection for debugging

`r.Routes()` returns a slice of `RouteInfo` — every method/pattern pair the dispatcher knows about. The example exposes them at `/debug/routes` with the request id in the response header, useful for verifying the dispatch tree against an expected manifest.

```go
r.GET("/debug/routes", func(w http.ResponseWriter, req *http.Request) {
    traceID := mw.GetRequestID(req.Context())
    w.Header().Set("X-Trace-ID", traceID)
    routes := r.Routes()
    type entry struct {
        Method  string `json:"method"`
        Pattern string `json:"pattern"`
    }
    list := make([]entry, 0, len(routes))
    for _, ri := range routes {
        list = append(list, entry{Method: ri.Method, Pattern: ri.Pattern})
    }
    _ = mm.JSON(w, http.StatusOK, list)
})
```

In production, gate this endpoint behind an authentication middleware — exposing the full route table to an unauthenticated reader leaks API shape to attackers.

## Step 12 — Serve with hardened timeouts and graceful shutdown

The same shape as the graceful-shutdown example: a goroutine-driven start, signal-driven drain, bounded grace period with `Shutdown(ctx)`. Static content is unusual in needing a long write timeout because slow clients legitimately consume a multi-megabyte asset over a metered connection.

```go
srv := &http.Server{
    Addr:         ":8080",
    Handler:      r,
    ReadTimeout:  15 * time.Second,
    WriteTimeout: 60 * time.Second,
    IdleTimeout:  120 * time.Second,
}
```

`WriteTimeout` here is the upper bound on a single response — if a client is so slow that writing a single asset takes more than 60 seconds, the connection is forcibly closed.

## Common questions

<section data-conversation="static-site-patterns">

### How do I serve a directory tree of static files through MuxMaster?

Register a catch-all route that delegates to `mux.ServeFiles`: for example `assetsGroup.ServeFiles("/assets/*filepath", staticRoot)`. The helper resolves the requested path, sets `Content-Type` from the file extension, and delegates to `http.FileServer`, which handles ETag, conditional GET, range requests, and HEAD transparently.

### How do I use the immutable-cache directive correctly?

Apply `Cache-Control: public, max-age=31536000, immutable` ONLY to URLs whose body cannot change behind the URL — typically content-hash-fingerprinted assets like `/assets/style.abc123.css`. HTML pages and any URL that may serve different bytes over time MUST use `no-cache, must-revalidate` instead, or browsers will cache stale content for a year.

### How do I support partial-content (range) requests for large files?

`mux.ServeFiles` already handles `Range` headers and returns 206 with `Content-Range` when the request asks for a slice of the file. No extra handler code is needed; the helper delegates to `http.ServeContent`, which negotiates ranges, ETags, and conditional GETs in one pass.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/static-site/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/static-site/main.go) at the v1.0.1 tag. The upstream directory also contains the `static/` tree the example serves (the index, asset stylesheet, two versioned `docs/` subtrees, and the themed 404 page) and the `serveFile` / `serveNotFound` helpers.
