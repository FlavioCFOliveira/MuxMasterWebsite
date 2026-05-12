---
datePublished: 2026-05-12
---

Thirteen runnable programs cover the production patterns MuxMaster is built for. **Start with [REST API](/examples/rest-api)** for the canonical CRUD service, then jump straight to **[Maximum performance](/examples/max-performance)** to see every v1.1.0 opt-in working together (`PoolRequestBundle`, `PoolFastParams`, `HandleFast`, `UseFast`, plus an in-process `/bench` endpoint that reports the speed-up on your hardware).

The four v1.1.0 patterns under it — **versioning**, **server-sent events**, **upload file**, **reverse proxy** — each demonstrate a different way to satisfy the [pool lifetime contract](/docs/max-performance#lifetime-contract--what-you-must-not-do): synchronous handlers, long-lived streams, the drain-before-spawn rule for background work, and synchronous proxying. The remaining seven cover authentication, caching, operational concerns, and content rendering.

Every example mirrors its upstream `main.go` at the v1.1.0 tag verbatim — the code is the source of truth.
