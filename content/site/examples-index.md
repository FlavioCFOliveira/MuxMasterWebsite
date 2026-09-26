---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

Thirteen runnable programs cover the production patterns MuxMaster is built for. **Start with [REST API](/examples/rest-api)** for a complete bookstore service, including an HTTP QUERY (RFC 10008) search route, then read **[Maximum performance](/examples/max-performance)** to see `PoolRequestBundle`, `PoolFastParams`, `HandleFast`, and `UseFast` working together, with an in-process `/bench` endpoint that compares a default and a pooled router on your own hardware.

The next four examples show how the [pool lifetime contract](/docs/max-performance#lifetime-contract--what-you-must-not-do) shapes a program. **Versioning** and **server-sent events** run with `PoolRequestBundle` enabled. **Upload file** applies the drain-before-spawn rule to background work. **Reverse proxy** keeps the pool off, because a reverse proxy built on `net/http.Transport` is not pool-safe. The remaining seven cover authentication, caching, graceful shutdown, server-side rendering, and static files.

Every code excerpt is taken from the upstream program at the v1.3.0 tag, and every page links to the full upstream source.
