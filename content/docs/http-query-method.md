---
datePublished: 2026-09-26
dateModified: 2026-09-26
---

# HTTP QUERY method (RFC 10008)

The HTTP QUERY method is a safe and idempotent HTTP method, like GET, that carries request content in the body, like POST. It is standardised by [RFC 10008](https://www.rfc-editor.org/rfc/rfc10008) (June 2026). MuxMaster has supported QUERY as a first-class method since v1.2.0: it has its own constant, its own registration helpers on `*Mux` and `*Group`, and the same dispatch, `Allow` header, and redirect handling as every other method the router accepts.

## When to use QUERY

The HTTP QUERY method that MuxMaster supports is defined in [RFC 10008, Section 2](https://www.rfc-editor.org/rfc/rfc10008#section-2) as follows:

> The QUERY method is used to initiate a server-side query. Unlike the GET method, which requests a representation of the resource identified by the target URI (as defined by Section 7.1 of [HTTP]), the QUERY method is used to ask the target resource to perform a query operation within the scope of that target resource.
>
> The content of the request and its media type define the query. The origin server determines the scope of the operation based on the target resource.
>
> — [RFC 10008, Section 2](https://www.rfc-editor.org/rfc/rfc10008#section-2)

In a MuxMaster application, QUERY is the method for a read-only request whose parameters are too large or too structured for a URL query string. A GET request can only carry its parameters in the URL; a POST request can carry a body, but POST is neither safe nor idempotent, so clients, proxies, and caches cannot retry or reuse it. A QUERY request carries the query document in the body and keeps the safe and idempotent guarantees of GET, as defined in RFC 10008 §2.

Typical uses of a QUERY route in MuxMaster are search endpoints that accept a JSON filter document, reporting endpoints with many criteria, and APIs that forward a query language (for example SQL or GraphQL text) in the request body.

## The MuxMaster QUERY API

MuxMaster exposes one constant and five registration methods for the HTTP QUERY method. All of them are defined in [`mux.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/mux.go) and [`group.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/group.go) at v1.3.0.

| Symbol | Signature | Purpose |
|---|---|---|
| `MethodQuery` | `const MethodQuery = "QUERY"` | The QUERY method token, for use with `Handle`, `HandleFunc`, `HandleE`, `HandleFast`, `Match`, and in tests. |
| `Mux.QUERY` | `func (m *Mux) QUERY(pattern string, h http.HandlerFunc)` | Registers an `http.HandlerFunc` for QUERY requests on `pattern`. |
| `Mux.QUERYE` | `func (m *Mux) QUERYE(pattern string, h HandlerFuncE)` | Registers an error-returning handler; a returned error goes to `Mux.ErrorHandler`, or produces a `500` when no handler is set. |
| `Mux.QUERYFast` | `func (m *Mux) QUERYFast(pattern string, h FastHandler)` | Registers a `FastHandler`, which receives the path parameters as a third argument. |
| `Group.QUERY` | `func (g *Group) QUERY(path string, h http.HandlerFunc)` | Registers an `http.HandlerFunc` for QUERY requests under the group prefix. |
| `Group.QUERYE` | `func (g *Group) QUERYE(path string, h HandlerFuncE)` | Registers an error-returning handler for QUERY requests under the group prefix. |

MuxMaster's `Group` has no `QUERYFast` method; a fast QUERY route on a MuxMaster group is registered with `Group.HandleFast(muxmaster.MethodQuery, path, h)`, as the upstream test `TestQueryMethod_Group_HandleFast_NoDedicatedShortcut` does.

Each MuxMaster QUERY helper is a shorthand for the generic registration call with `MethodQuery`: `mux.QUERY(p, h)` registers the same route as `mux.HandleFunc(muxmaster.MethodQuery, p, h)`, and `mux.QUERYFast(p, h)` the same route as `mux.HandleFast(muxmaster.MethodQuery, p, h)`.

## Step 1 — Register a QUERY route

A QUERY route in MuxMaster is registered like any other route: call `Mux.QUERY` (or `Group.QUERY`) with a path pattern and an `http.HandlerFunc`. Path parameters, groups, and middleware registered with `Pre` and `Use` apply to QUERY routes exactly as they apply to GET or POST routes.

Steps 1 and 2 together form a complete `main.go`. It starts with this package clause and these imports:

```go
package main

import (
	"encoding/json"
	"log"
	"mime"
	"net/http"

	"github.com/FlavioCFOliveira/MuxMaster"
)
```

```go
func main() {
	mux := muxmaster.New()
	mux.QUERY("/books/search", searchBooks)

	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

The same QUERY route can be registered with MuxMaster's generic API as `mux.Handle(muxmaster.MethodQuery, "/books/search", http.HandlerFunc(searchBooks))`. On a group, `api := mux.Group("/api/v1")` followed by `api.QUERY("/books/search", searchBooks)` registers `/api/v1/books/search`.

## Step 2 — Validate the request content in the handler

A MuxMaster QUERY handler must validate the `Content-Type` header and the body itself, because the router performs no validation of either. RFC 10008 §2.1 requires the server to fail a QUERY request whose `Content-Type` is missing or inconsistent with the content (`400 Bad Request`), whose media type it does not support (`415 Unsupported Media Type`), or whose content it cannot process (`422 Unprocessable Content`). The handler below accepts JSON only, limits the body to 1 MiB, and advertises its accepted format with the `Accept-Query` response header defined in RFC 10008 §3.

```go
// BookQuery is the query document a client sends in the QUERY request body.
type BookQuery struct {
	Author string `json:"author"`
	Year   int    `json:"year"`
}

func searchBooks(w http.ResponseWriter, r *http.Request) {
	// RFC 10008 §2.1: the router does not check Content-Type; the handler must.
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, "missing or invalid Content-Type", http.StatusBadRequest)
		return
	}
	if mediaType != "application/json" {
		http.Error(w, "unsupported query format", http.StatusUnsupportedMediaType)
		return
	}

	var q BookQuery
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&q); err != nil {
		http.Error(w, "malformed query", http.StatusBadRequest)
		return
	}

	// RFC 10008 §3: advertise the accepted query format.
	w.Header().Set("Accept-Query", `"application/json"`)
	_ = muxmaster.JSON(w, http.StatusOK, map[string]any{"author": q.Author, "year": q.Year, "results": []string{}})
}
```

This excerpt uses only the standard library and the MuxMaster v1.3.0 API (`encoding/json`, `mime`, `net/http`, `muxmaster.JSON`). It was compiled and exercised against MuxMaster v1.3.0 for this page: a JSON QUERY request returned `200`, a QUERY request without `Content-Type` returned `400`, and a `text/plain` request returned `415`. The upstream [cookbook recipe](/docs/cookbook#query-endpoint-rfc-10008) shows a variant of the same handler.

## Step 3 — Call the QUERY route with curl

Send a QUERY request to the MuxMaster route with `curl` by setting the method with `-X QUERY`, the media type with `-H`, and the query document with `--data`.

```bash
curl -i -X QUERY http://localhost:8080/books/search \
  -H 'Content-Type: application/json' \
  --data '{"author":"Ursula K. Le Guin","year":1969}'
```

The response to this QUERY request carries `200 OK`, the header `Accept-Query: "application/json"`, and the JSON body written by the handler. Sending the same command without the `Content-Type` header returns `400 Bad Request` from the handler, not from the MuxMaster router.

## Method dispatch: ANY, Allow, 405, and OPTIONS

MuxMaster treats the HTTP QUERY method as a member of its fixed method set, so the router's method-level behaviours include QUERY:

- **`ANY` registers QUERY.** MuxMaster's `Mux.ANY` and `Group.ANY` register the handler for GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, CONNECT, TRACE, and QUERY. Since v1.2.0, a QUERY request to an `ANY` route reaches its handler; in v1.1.0 it received `405` or `404`.
- **`Match` accepts QUERY.** In MuxMaster, `mux.Match([]string{http.MethodGet, muxmaster.MethodQuery}, "/m", h)` registers `h` for both methods.
- **`405 Method Not Allowed` lists QUERY.** When `HandleMethodNotAllowed` is `true` (the default), MuxMaster answers a request with an unregistered method on a path that has a QUERY route with `405` and lists QUERY in the `Allow` header. With GET and QUERY registered on `/books/search`, a POST request receives `Allow: GET, QUERY, OPTIONS`.
- **Automatic `OPTIONS` lists QUERY.** When `HandleOPTIONS` is `true` (the default), MuxMaster answers an OPTIONS request to the same path with `204 No Content` and `Allow: GET, QUERY, OPTIONS`.
- **`Allow` order.** MuxMaster writes the `Allow` header in the fixed order GET, HEAD, POST, PUT, PATCH, DELETE, CONNECT, TRACE, QUERY, OPTIONS, as the upstream test `TestQueryMethod_AllowHeader_FullOrder` asserts.

## Redirects preserve the QUERY method and body

MuxMaster redirects a QUERY request with `307 Temporary Redirect`, which preserves the method and the body, when `RedirectCode` is left at its default of `0`. This applies to both automatic redirects: `RedirectTrailingSlash` (on by default) and `RedirectFixedPath` (off by default). GET and HEAD requests receive `301 Moved Permanently` instead. A non-zero `Mux.RedirectCode` replaces the default for every method, QUERY included; set it to `308 Permanent Redirect` if you need a permanent redirect that still preserves the method and body. RFC 10008 §2.5 notes that the exceptions that let clients turn a redirected POST into a GET after `301` or `302` do not apply to QUERY.

With only `/books/search` registered on a MuxMaster router, a QUERY request to `/books/search/` receives `307` with `Location: /books/search`, and a GET request to the same path receives `301`.

## What the handler is responsible for

MuxMaster routes a QUERY request to its handler without inspecting the request content. The handler, or middleware the application adds, is responsible for:

- **Content validation.** Checking the `Content-Type` and the body of a QUERY request and failing with `400`, `415`, or `422` as RFC 10008 §2.1 requires (see Step 2); MuxMaster performs no such check.
- **Advertising query formats.** Setting the `Accept-Query` response header (RFC 10008 §3) when the resource wants to advertise the media types it accepts. MuxMaster never sets it.
- **Caching.** RFC 10008 §2.7 makes QUERY responses cacheable, with a cache key that includes the request content. MuxMaster does not implement HTTP response caching; cache headers and any cache in front of the router are the application's responsibility.
- **CORS.** QUERY is not a CORS-safelisted method, so a browser sends a preflight `OPTIONS` request before a cross-origin QUERY request (RFC 10008 §4). When you use the MuxMaster `CORS` middleware, list `"QUERY"` in `CORSOptions.AllowedMethods`.

## Why MuxMaster defines `MethodQuery`

MuxMaster defines `MethodQuery` because Go 1.27's `net/http` package does not define an `http.MethodQuery` constant; the addition is tracked by the Go project as [golang/go#80058](https://github.com/golang/go/issues/80058). The value of `muxmaster.MethodQuery` is the string `"QUERY"`, the method token that RFC 10008 registers. If a future Go release adds `http.MethodQuery`, its value will also be `"QUERY"`, so MuxMaster's constant will remain equal to it; according to the GoDoc of `MethodQuery` at v1.3.0, MuxMaster does not deprecate or remove the constant when that happens.

Before v1.2.0, MuxMaster panicked when a route was registered for QUERY, for example with `Handle("QUERY", …)`. The panic message was `muxmaster: unsupported HTTP method 'QUERY'`, raised in [`mux.go` at v1.1.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/mux.go#L389). Since v1.2.0, QUERY is one of the ten methods MuxMaster accepts: GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, CONNECT, TRACE, and QUERY.

## Common questions

<section data-conversation="http-query-method-support">

### Does MuxMaster support the HTTP QUERY method?

Yes: MuxMaster has supported the HTTP QUERY method defined in RFC 10008 since v1.2.0, released on 2026-09-26.

MuxMaster's QUERY support includes the `MethodQuery` constant, the `Mux.QUERY`, `Mux.QUERYE`, `Mux.QUERYFast`, `Group.QUERY`, and `Group.QUERYE` helpers, QUERY in `ANY`, and QUERY in the `Allow` header of `405` and automatic `OPTIONS` responses.

### How do I register a QUERY route in MuxMaster?

In MuxMaster, call `mux.QUERY(pattern, handler)` with an `http.HandlerFunc`, for example `mux.QUERY("/books/search", searchBooks)`.

In MuxMaster, use `mux.QUERYE` for an error-returning QUERY handler, `mux.QUERYFast` for a `FastHandler`, `group.QUERY` or `group.QUERYE` inside a group, and `group.HandleFast(muxmaster.MethodQuery, path, h)` for a fast QUERY route on a group.

### Which status code does MuxMaster use to redirect a QUERY request?

MuxMaster redirects a QUERY request with `307 Temporary Redirect` by default, for both trailing-slash and fixed-path redirects, so the client repeats the QUERY method with the same body.

MuxMaster redirects GET and HEAD with `301 Moved Permanently`. A non-zero `Mux.RedirectCode` overrides the default for every method, QUERY included.

### Does MuxMaster validate the Content-Type or body of a QUERY request?

No: the MuxMaster router performs no validation of a QUERY request's `Content-Type` header or body, so the handler must validate both.

RFC 10008 §2.1 specifies the failures a MuxMaster QUERY handler should return: `400` for a missing or inconsistent `Content-Type`, `415` for an unsupported media type, and `422` for content that cannot be processed.

### Why does MuxMaster define `MethodQuery` instead of using `net/http`?

MuxMaster defines `MethodQuery` because Go 1.27's `net/http` package has no `http.MethodQuery` constant yet (golang/go#80058).

The value of MuxMaster's `MethodQuery` constant is `"QUERY"` and will remain equal to any future `http.MethodQuery`, so code that uses it will not need to change.

</section>

## Sources

Every statement on this page was checked against the upstream MuxMaster source at the v1.3.0 tag and against RFC 10008, except the pre-v1.2.0 panic message, which was checked against the source at the v1.1.0 tag.

- [`mux.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/mux.go) — `MethodQuery`, `Mux.QUERY`, `Mux.QUERYE`, `Mux.QUERYFast`, `ANY`, the `Allow` order, and the default redirect code.
- [`mux.go` at v1.1.0](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/mux.go#L389) — the pre-v1.2.0 panic message `muxmaster: unsupported HTTP method 'QUERY'`.
- [`group.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/group.go) — `Group.QUERY`, `Group.QUERYE`, and `Group.ANY`.
- [`query_method_test.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/query_method_test.go) — the tests for dispatch, `ANY`, `Match`, the `Allow` header, and redirects.
- [`docs/routing.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/docs/routing.md) — method helpers and redirect status codes.
- [`README.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/README.md) — the supported method set.
- [`CHANGELOG.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/CHANGELOG.md) — the `[1.2.0]` entry that added QUERY support.
- [`release-notes/v1.2.0-20260926.md`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/release-notes/v1.2.0-20260926.md) — the v1.2.0 highlights and upgrade notes (also on this site as [Release notes v1.2.0](/releases/v1.2.0)).
- [RFC 10008: The HTTP QUERY Method](https://www.rfc-editor.org/rfc/rfc10008) — §2 (method definition, quoted above), §2.1 (media types and failure codes), §2.5 (redirection), §2.7 (caching), §3 (`Accept-Query`), §4 (security considerations and CORS).
- [golang/go#80058](https://github.com/golang/go/issues/80058) — the Go issue that tracks `http.MethodQuery`.
