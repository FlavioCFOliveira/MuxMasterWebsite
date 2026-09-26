---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# REST API example

A bookstore REST API that exercises most MuxMaster features in one program: the HTTP method helpers including an HTTP QUERY (RFC 10008) search route, path / regex / catch-all parameters, group composition, scoped middleware, the error-returning handler family, FastHandler + FastMiddleware, route introspection (`Routes`, `Lookup`, `Walk`), `Mount` for sub-handlers, and the production-grade middleware stack (`RealIP`, `RequestID`, `Logger`, `Recoverer`, `Timeout`, throttles, gzip, CORS, BasicAuth, NoCache, security headers). Reach for this example as the canonical "how do I structure a CRUD service?" reference.

## Step 1 — Construct the router and pin the dispatch flags

The seven flags on `*Mux` are set explicitly even when the value matches the default — for documentation purposes, the pinned flag set is the contract. Notable choices: `RedirectFixedPath = false` is the security default, because a redirect to a canonicalised path can bypass middleware that inspects the raw path, and `UseRawPath`/`UnescapePathValues` both stay `false` to let `net/http` canonicalise the URL before matching.

```go
r := mm.New()

// Configuration flags (explicit for documentation purposes).
r.RedirectTrailingSlash = true  // /books/ → /books
r.RedirectFixedPath = false     // security default — keeps middleware auth intact
r.HandleMethodNotAllowed = true // 405 with Allow header
r.HandleOPTIONS = true          // automatic OPTIONS + Allow header
r.CaseInsensitive = false       // strict case matching
r.UseRawPath = false            // use decoded path for matching
r.UnescapePathValues = false    // raw param values; opt-in if you need %2F decoded
```

`UseRawPath = true` + `UnescapePathValues = true` is the configuration `SECURITY.md` CDX-S8-002 forbids in combination — `ServeFiles` refuses to register on a router with both flags set.

## Step 2 — Wire JSON-shaped error / fallback handlers

Five override slots are populated: `PanicHandler` (last-resort panic recovery), `ErrorHandler` (central handler for `HandlerFuncE` errors), `NotFound` (JSON 404 with the offending method/path echoed back), `MethodNotAllowed` (JSON 405; the `Allow` header is set by the dispatcher), and `GlobalOPTIONS` (permissive CORS preflight for paths without their own CORS middleware).

```go
r.PanicHandler = func(w http.ResponseWriter, req *http.Request, rcv any) {
	log.Error("panic recovered", "path", req.URL.Path, "panic", rcv)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: "internal server error", Code: 500})
}

// ErrorHandler: central handler for HandlerFuncE errors.
r.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
	traceID := mw.GetRequestID(req.Context())
	code := http.StatusInternalServerError
	var he mm.HTTPError
	if errors.As(err, &he) {
		code = he.StatusCode()
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   err.Error(),
		Code:    code,
		TraceID: traceID,
	})
}
```

The trace id propagates from `RequestID` (Step 5) into the error response, so a client error and the corresponding server log share a correlation key.

## Step 3 — Normalise paths in `Pre`

`CleanPath` belongs in `Pre` so the radix tree sees the cleaned URL: it normalises double slashes and dot segments, so `/api//v1/books/../books` is matched as `/api/v1/books`. It hands the router a shallow copy of the request with the cleaned path instead of redirecting the client.

```go
r.Pre(mw.CleanPath())
```

The upstream comment names `StripSlashes` as an alternative and prefers `CleanPath` for REST APIs. `StripSlashes` only removes trailing slashes, while `CleanPath` also resolves dot segments and repeated slashes.

## Step 4 — Register the fast routes before `Use`

`GETFast`, `HandleFast`, and `UseFast` register routes and middleware for the `FastHandler` path, which passes parameters as a third argument instead of through the request context. Since v1.2.0 the example registers every fast route before `Use`, because MuxMaster panics when a `HandleFast` route is registered after `Use` middleware: `Use` middleware never wraps fast routes, and the panic prevents a fast route from silently bypassing it. `/health` is a public probe, `/metrics` a demo scrape endpoint, and `/api/v1/fast/echo` reflects its own request, so none of them needs the `Use` stack.

```go
// FastHandler: bypasses context allocation — zero allocs for static routes.
// Ideal for health-check endpoints that are hit thousands of times per second.
r.GETFast("/health", func(w http.ResponseWriter, req *http.Request, _ mm.Params) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, `{"status":"ok"}`)
})

// UseFast registers middleware that applies only to HandleFast routes below.
r.UseFast(fastTimer)

// GETFast: metrics endpoint — no context allocation, direct Params argument.
r.GETFast("/metrics", metricsHandler)

// POSTFast via HandleFast: demonstrates HandleFast with explicit method.
r.HandleFast(http.MethodPost, "/api/v1/fast/echo", fastEcho)

```

`fastTimer` is a `FastMiddleware` (`func(FastHandler) FastHandler`): the same composition rules as standard middleware, with the fast-handler signature.

## Step 5 — Apply the global middleware stack

Seven layers of cross-cutting concerns: `RealIP` (rewrite `RemoteAddr` from `X-Forwarded-For` when the peer is a trusted proxy), `RequestID`, `Logger`, `Recoverer`, `ThrottleBacklog`, `WithValue` (inject the application version into every request context), and `Compress`.

```go
r.Use(
	// Overwrite r.RemoteAddr with X-Forwarded-For when peer is a trusted proxy.
	mw.RealIP(&localhost, &private10, &private172, &private192),

	// Assign or propagate X-Request-ID for distributed tracing.
	mw.RequestID(),

	// Structured request logging (method, path, status, latency).
	mw.Logger(os.Stdout),

	// Recover from panics — complements r.PanicHandler for middleware panics.
	mw.RecovererWithLogger(log),

	// Global rate limiting: max 200 concurrent requests, backlog 100, 5s timeout.
	mw.ThrottleBacklog(200, 100, 5*time.Second),

	// Inject a value into every request context — accessible via ctx.Value.
	mw.WithValue(appVersionKey{}, "v1.0.0"),

	// Compress responses >= 1 kB with gzip when the client accepts it.
	mw.Compress(gzip.BestSpeed),
)
```

`ThrottleBacklog(200, 100, 5s)` caps in-flight requests at 200 with a 100-request queue and a 5-second queue timeout — the queue absorbs short bursts without rejecting traffic, the timeout bounds how long a client waits on the queue.

## Step 6 — Register the public convenience routes

`/version` reads the value that `WithValue` injected in Step 5 — the usual Go pattern for application-wide values that handlers need to read. `/books` redirects the version-less path to its versioned canonical URL.

```go
// Standard HandlerFunc: version endpoint reads the injected app version.
r.GET("/version", func(w http.ResponseWriter, req *http.Request) {
	ver, _ := req.Context().Value(appVersionKey{}).(string)
	_ = mm.JSON(w, http.StatusOK, map[string]string{"version": ver})
})

// Redirect old URL to new one (demonstrates mm.Redirect helper).
r.GET("/books", func(w http.ResponseWriter, req *http.Request) {
	mm.Redirect(w, req, http.StatusMovedPermanently, "/api/v1/books")
})
```

The `/books` 301 demonstrates a canonical-URL redirect: the version-less path moves permanently to the versioned canonical so search engines and intermediaries transfer their accumulated signal to the right URL.

## Step 7 — Expose route introspection at `/debug/routes` and `/debug/lookup`

`r.Routes()` enumerates every registered route; `r.Lookup(method, path)` resolves a method/path against the dispatcher and returns the matched handler, parameters, and a found flag — invaluable for debugging dispatch decisions without touching the live traffic path.

```go
r.GET("/debug/routes", func(w http.ResponseWriter, req *http.Request) {
	_ = mm.JSON(w, http.StatusOK, r.Routes())
})

// Route lookup: demonstrates Lookup().
r.GET("/debug/lookup", func(w http.ResponseWriter, req *http.Request) {
	method := req.URL.Query().Get("method")
	path := req.URL.Query().Get("path")
	if method == "" || path == "" {
		_ = mm.Text(w, http.StatusBadRequest, "provide ?method=GET&path=/api/v1/books/1")
		return
	}
	handler, params, found := r.Lookup(method, path)
	type result struct {
		Found   bool      `json:"found"`
		Handler string    `json:"handler,omitempty"`
		Params  mm.Params `json:"params,omitempty"`
	}
	name := ""
	if handler != nil {
		name = fmt.Sprintf("%T", handler)
	}
	_ = mm.JSON(w, http.StatusOK, result{Found: found, Handler: name, Params: params})
})
```

Production deployments gate these endpoints behind authentication — exposing the full route table to an unauthenticated reader leaks API shape to attackers.

## Step 8 — Compose the `/api/v1` group with scoped middleware

`Group(prefix)` returns a sub-router that inherits the parent's stack and adds its own. The `/api/v1` group adds two scoped layers on top of the global stack: `Timeout(30s)` (per-request deadline) and `ThrottlePerIP(50, 10s)` (50 concurrent in-flight requests per source IP, 10-second queue timeout).

```go
v1 := r.Group("/api/v1")
v1.Use(
	// Per-request timeout: handlers that exceed 30 s get a cancelled context.
	mw.Timeout(30*time.Second),

	// Per-IP throttle: max 50 concurrent requests per client IP.
	mw.ThrottlePerIP(50, 10*time.Second, nil),
)
```

`ThrottlePerIP` is the per-client equivalent of `ThrottleBacklog`: instead of capping in-flight requests across the process, it caps them per source IP, so one misbehaving client cannot starve the rest of the user base.

## Step 9 — Register the books CRUD surface

The seven canonical CRUD routes are registered on a `books := v1.Group("/books")` sub-group. The example deliberately mixes `GET`/`POST`/etc. (`http.HandlerFunc`-shaped) with their `E`-suffixed counterparts (`GETE`/`POSTE`/`DELETEE`/etc., which return an error so the central `ErrorHandler` writes the response).

```go
books := v1.Group("/books")

// GET /api/v1/books — list with pagination.
books.GET("", store.listBooks)

// POST /api/v1/books — create; uses POSTE (error-returning handler).
books.POSTE("", store.createBook)

// Book IDs are always numeric (Store hands them out via a sequential
// counter), so every "/books/{id}..." route below uses the regex
// parameter {id:[0-9]+} instead of the plain named parameter :id.
//
// This is not just style: a regex parameter and a plain named parameter
// cannot share the same tree position (specification/routing.md §71 — a
// wildcard conflicts with an already-registered wildcard at the same
// position). Mixing ":id" here with "{id:[0-9]+}" on the sibling
// "/details" route below panics at registration. Using the regex form
// consistently also rejects non-numeric ids at the router, before they
// ever reach a handler. PathParam(r, "id") is unaffected: a regex
// parameter's captured value is still stored under its name, "id".

// HEAD /api/v1/books/{id:[0-9]+} — check existence without transferring the body.
books.HEAD("/{id:[0-9]+}", store.headBook)

// GET /api/v1/books/{id:[0-9]+} — get by id; PathParam demo.
books.GETE("/{id:[0-9]+}", store.getBook)

// GET /api/v1/books/{id:[0-9]+}/details — regex param: only numeric IDs.
// PathParam still returns the matched segment (e.g. "42") under key "id".
books.GET("/{id:[0-9]+}/details", store.getBookDetails)

// PUT /api/v1/books/{id:[0-9]+} — full replacement; PUTE demo.
books.PUTE("/{id:[0-9]+}", store.replaceBook)

// PATCH /api/v1/books/{id:[0-9]+} — partial update.
books.PATCH("/{id:[0-9]+}", store.patchBook)

// DELETE /api/v1/books/{id:[0-9]+} — delete; DELETEE demo.
books.DELETEE("/{id:[0-9]+}", store.deleteBook)
```

Since v1.2.0 every book route uses the regex parameter `{id:[0-9]+}` instead of `:id`. A regex parameter and a plain named parameter cannot share one tree position, so mixing `:id` with the `{id:[0-9]+}` of the `/details` route panics at registration. The regex also rejects non-numeric ids in the router, before they reach a handler, and `PathParam(r, "id")` still returns the value under the name `id`.

## Step 10 — Compose nested resources, multi-method routes, and a QUERY route

The books group also registers nested resources with two and three path parameters (`/{id:[0-9]+}/reviews/:rid`), `Match` and `ANY` for routes that share logic across methods, a `ServeFiles` tree, and an HTTP QUERY route. `books.QUERY("/search", store.searchBooks)` registers a route for the QUERY method defined in RFC 10008, which is safe and idempotent like GET but carries its query in the request body.

```go
// Nested resource: /api/v1/books/{id:[0-9]+}/reviews
// Two path parameters in a single route — uses ParamsFromContext.
books.GET("/{id:[0-9]+}/reviews", store.listReviews)
books.POSTE("/{id:[0-9]+}/reviews", store.createReview)

// Three path parameters: book → reviews → review.
// Demonstrates Params.Map() and RoutePattern().
books.GETE("/{id:[0-9]+}/reviews/:rid", store.getReview)

// Match registers one handler for multiple methods (GET + HEAD share logic).
books.Match(
	[]string{http.MethodGet, http.MethodHead},
	"/featured",
	http.HandlerFunc(store.featuredBooks),
)

// ANY registers a handler for every standard HTTP method.
books.ANY("/ping", func(w http.ResponseWriter, req *http.Request) {
	_ = mm.Text(w, http.StatusOK, fmt.Sprintf("pong from %s /api/v1/books/ping", req.Method))
})

// ServeFiles from the local filesystem. The catch-all param is "filepath".
// HEAD requests are automatically registered alongside GET by ServeFiles.
books.ServeFiles("/files/*filepath", http.Dir("./static/books"))

// QUERY /api/v1/books/search — HTTP QUERY (RFC 10008): safe and
// idempotent like GET, but carries a JSON query body like POST.
// store.searchBooks enforces the Content-Type validation RFC 10008
// section 2.1 requires (400 when missing, 415 when unsupported) and
// advertises the accepted query format via Accept-Query (RFC 10008
// section 3) — the router itself performs none of this validation.
books.QUERY("/search", store.searchBooks)
```

`books.ServeFiles("/files/*filepath", ...)` registers a catch-all under the books group — the dispatched path is everything after `/api/v1/books/files/` (the catch-all parameter `filepath`). HEAD is registered automatically.

## Step 11 — Validate the QUERY request in the handler

The MuxMaster router performs no validation of a QUERY request's `Content-Type` or body, so `searchBooks` does it: it answers `400` when `Content-Type` is missing and `415` when it is not `application/json`, as RFC 10008 §2.1 requires, and advertises the accepted query format with the `Accept-Query` response header from RFC 10008 §3. The page [HTTP QUERY method (RFC 10008)](/docs/http-query-method) explains the rest of MuxMaster's QUERY support.

```go
func (s *Store) searchBooks(w http.ResponseWriter, r *http.Request) {
	// Accept-Query is a Structured Field List (RFC 9651); "application/json"
	// is encoded here as a Structured Field String.
	w.Header().Set("Accept-Query", `"application/json"`)

	switch ct := r.Header.Get("Content-Type"); {
	case ct == "":
		// RFC 10008 section 2.1: fail when Content-Type is missing.
		_ = mm.JSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "Content-Type is required for a QUERY request",
			Code:  400,
		})
		return
	case ct != "application/json":
		// RFC 10008 section 2.1: fail on an unsupported media type.
		_ = mm.JSON(w, http.StatusUnsupportedMediaType, ErrorResponse{
			Error: fmt.Sprintf("unsupported query media type: %s (expected application/json)", ct),
			Code:  415,
		})
		return
	}

	var q searchQuery
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		_ = mm.JSON(w, http.StatusBadRequest, ErrorResponse{
			Error: fmt.Sprintf("invalid JSON query body: %v", err),
			Code:  400,
		})
		return
	}

	// …

	_ = mm.JSON(w, http.StatusOK, results)
}
```

Call the route with a JSON query document in the body:

```bash
curl -X QUERY http://localhost:8080/api/v1/books/search \
     -H 'Content-Type: application/json' \
     -d '{"genre":"tech"}'
```

## Step 12 — Scope CORS to the `/authors` subset via `With`

`With(...)` returns a group whose middleware applies only to the routes registered on the returned group — useful when CORS policy is per-resource rather than global. The example narrows allowed origins to two known callers and explicit methods/headers; `MaxAge` of 3600 caches the preflight at the browser for an hour.

```go
corsOpts := mw.CORSOptions{
	AllowedOrigins: []string{"https://bookshelf.example.com", "http://localhost:3000"},
	AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	AllowedHeaders: []string{"Content-Type", "Authorization"},
	MaxAge:         3600,
}
authors := v1.With(mw.CORS(corsOpts))
// XML endpoint: demonstrates mm.XML response helper + With() scoped CORS.
authors.GET("/authors/:id/xml", store.getAuthorXML)
```

When a path is wrapped with explicit `mw.CORS`, that middleware short-circuits OPTIONS preflight; `r.GlobalOPTIONS` from Step 2 only fires on paths without their own CORS handler.

## Step 13 — Inline-define a sub-group via `Route`, then a BasicAuth admin group

`Route(prefix, fn)` is the inline-callback variant of `Group`: the closure receives a `*Group` and registers everything that belongs under the prefix. Useful when the sub-group's lifetime is the registration site itself.

```go
v1.Route("/categories", func(g *mm.Group) {
	g.Use(mw.NoCache()) // categories change frequently — never cache
	g.GET("", listCategories)
	g.GET("/:slug", getCategoryBySlug)
})

// ── Admin (BasicAuth + NoCache) ───────────────────────────────────────────

admin := v1.Group("/admin")
admin.Use(
	mw.BasicAuth("Bookstore Admin", map[string]string{
		"admin": "s3cr3t!",
	}),
	mw.NoCache(),
	mw.SetHeader("X-Admin-Zone", "true"),
)
admin.GET("/dashboard", adminDashboard)
admin.DELETEE("/books/:id", store.adminDeleteBook)
admin.GET("/routes", func(w http.ResponseWriter, req *http.Request) {
	_ = mm.JSON(w, http.StatusOK, r.Routes())
})
```

The BasicAuth credentials are illustrative — production code stores hashed passwords in a database and rebuilds the middleware on rotation.

## Step 14 — `Mount` a legacy sub-handler at `/legacy`

`Mount(prefix, handler)` strips the prefix before forwarding to the sub-handler. Useful for migrating old APIs incrementally: the new router lives at `/`, the legacy implementation continues to handle `/legacy/*` URLs as if it were rooted at `/`.

```go
legacyMux := buildLegacyMux()
r.Mount("/legacy", legacyMux)
```

The strip-prefix behaviour lets the legacy code carry its own URL space unchanged — only the new router knows about the prefix.

## Step 15 — Walk the dispatch tree at startup for a sanity-check log line

`Walk` enumerates standard routes, `WalkFast` enumerates fast routes. The example logs counts so a startup line confirms the expected number of routes were registered — a quick smoke test against accidental drop or duplicate registration.

```go
var stdCount, fastCount int
_ = r.Walk(func(method, pattern string, _ http.Handler) error {
	stdCount++
	return nil
})
_ = r.WalkFast(func(method, pattern string, _ mm.FastHandler) error {
	fastCount++
	return nil
})
log.Info("routes registered", "std", stdCount, "fast", fastCount)
```

In production the same callback can populate a metrics gauge or write a manifest to disk for the deployment pipeline to verify against the previous build.

## Step 16 — Serve with hardened timeouts and graceful shutdown

The same shape as the graceful-shutdown example: goroutine-driven start, signal-driven drain, bounded grace period via `Shutdown(ctx)`. The timeout values match the throughput characteristics of a JSON API: short read window, moderate write window, long idle for keep-alive efficiency.

```go
srv := &http.Server{
	Addr:         ":8080",
	Handler:      r,
	ReadTimeout:  15 * time.Second,
	WriteTimeout: 30 * time.Second,
	IdleTimeout:  120 * time.Second,
}
```

For the full goroutine-and-Shutdown(ctx) drain, see the graceful-shutdown example — this file uses the identical pattern.

## Common questions

<section data-conversation="rest-api-patterns">

### How do I structure CRUD routes for a single resource?

Register the resource on a `Group`, for example `books := v1.Group("/books")`, with `GET ""`, `POSTE ""`, `GETE "/{id:[0-9]+}"`, `PUTE "/{id:[0-9]+}"`, `PATCH "/{id:[0-9]+}"`, and `DELETEE "/{id:[0-9]+}"`. The error-returning variants (`POSTE`, `GETE`, `PUTE`, `DELETEE`) hand failures to the central `ErrorHandler`, so handlers stay free of response-writing boilerplate.

### How do I validate the request body before touching the store?

Decode into a typed struct, validate with a separate function, and return `mm.Error(http.StatusUnprocessableEntity, ...)` when validation fails. The `ErrorHandler` from Step 2 turns the typed error into the JSON response with the right status. The example's `createBook` handler shows the pattern end-to-end.

### How do I version the API?

Mount each version under its own prefix — `r.Group("/api/v1")` in this example — and register the version-specific handlers on the group. Routes that survived unchanged across versions can be registered on a shared registration function; routes that diverge are registered separately. The example carries one version on purpose because adding `/v2` is documentation, not code.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/rest-api/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/examples/rest-api/main.go) at the v1.3.0 tag. The upstream file also contains the in-process `Store` (books, authors, reviews), every CRUD handler, the `buildLegacyMux` sub-handler, the FastHandler `metricsHandler` and `fastEcho`, and the `findBookByID` lookup helper — follow the link for the full program.

Source: <https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/examples/rest-api>
