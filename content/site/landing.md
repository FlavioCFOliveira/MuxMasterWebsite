# MuxMaster

<!--
  TODO(spec): exact landing copy is open-questions.md item 5.
  This file is a placeholder. The handler currently renders the landing
  page directly from templates/pages/landing.html, not from this Markdown
  source; the file exists so the directory layout matches
  specification/content-sources.md and so the .md companion at /.md (when
  it lands) has a source.
-->

A high-performance, zero-dependency HTTP router for Go.

MuxMaster is a radix-tree HTTP router with O(k) lookups, zero allocations on static routes, and 100% compatibility with `net/http`.

## Highlights

- Zero dependencies — the router and middleware are implemented on the Go standard library alone.
- Radix-tree performance — static-route lookups are O(k) and allocation-free on the hot path.
- Idiomatic API — groups, scoped middleware, typed errors, and typed parameter helpers, without abandoning `net/http`.
- Production-ready middleware — RequestID, Recoverer, Logger, Compress, RealIP, Timeout, Throttle, BasicAuth, JWTAuth, OAuth2Introspect, APIKey, CORS.
