---
datePublished: 2026-05-12
dateModified: 2026-09-26
---

# Security Policy

## Supported Versions

Only the latest release receives security fixes. Older versions are not maintained.

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report security issues to **flaviocfo@gmail.com** with:

- A description of the vulnerability and its potential impact
- Steps to reproduce or a minimal proof-of-concept
- Any suggested mitigations you are aware of

You will receive an acknowledgement within 72 hours. We aim to release a fix
within 14 days for critical issues and 30 days for others. We will credit you
in the release notes unless you prefer to remain anonymous.

## Current security behaviour

The router and its middleware behave as follows. Each statement describes
MuxMaster v1.3.0.

- **Route parameters survive wrapped contexts.** `ParamsFromContext()`
  returns the route parameters even when middleware registered with `Use()`
  wraps the request context (for example `Timeout`, `WithValue`, or
  `RealIP`). When the context is not the router's own, a fallback walks the
  wrapped context chain to find the parameters.
- **Redirect targets never carry a request-supplied host.** The router's own
  trailing-slash and fixed-path redirects build the `Location` header from
  the path and the raw query only. A request line in absolute form (for
  example `GET http://example.com/x HTTP/1.1`) cannot place its scheme or
  host in the `Location` header.
- **Redirect `Location` encoding.** The same redirects percent-encode every
  ASCII control byte (0x00–0x1F, 0x7F) per RFC 9110 §5.5 and every
  backslash (`\` → `%5C`). The backslash encoding prevents the WHATWG
  "special authority slashes" shape, in which a browser resolves a
  `Location` such as `/\example.com/` to another origin. This is a
  deliberate divergence from `net/http.Redirect`, which encodes neither;
  the `muxmaster.Redirect` helper delegates to `net/http.Redirect` and
  therefore does not apply it. `%5C` decodes back to `\` on the follow-up
  request, so a registered backslash route is still reached on the same
  origin. A backslash can reach a redirect target only through a route the
  operator registered.
- **Root `HandleFast` refuses `Use()` middleware.** `Mux.HandleFast` panics
  when middleware registered with `Use()` is present, as `Group.HandleFast`
  does, so a fast route is never registered without the middleware the
  operator expects to wrap it. See "Pre vs Use security boundary" below.
- **`ServeFiles` raw-path guard.** `Mux.ServeFiles` and `Group.ServeFiles`
  panic at registration when the Mux has both `UseRawPath` and
  `UnescapePathValues` enabled, because the captured path could contain
  decoded `/` characters that `http.FileServer` treats as separators. Both
  variants share one check and the same panic message.
- **`CORS` sends `Vary: Origin` on every response.** It adds the header
  with `Header.Add`, keeping other `Vary` values, to every response it
  produces or forwards, including requests without `Origin` and its own
  400 and 403 responses. A shared cache therefore never serves a response
  fetched without CORS relevance to a CORS-relevant origin.
- **`BasicAuth` compares credentials in constant time across all users.**
  Every request scans all registered users with `crypto/subtle`, and both
  the supplied and the stored password are hashed with SHA-256 before the
  comparison, so the comparison always runs on two 32-byte digests
  regardless of the supplied password's length. Because every user is
  scanned, the cost of a request grows with the number of registered users;
  the measured cost is published on [/benchmarks](/benchmarks).
- **`OAuth2Introspect` redacts credentials.** The construction-time
  `slog.Warn` and `slog.Info` lines log the endpoint's host and scheme only,
  never the full URL. The construction-time panics for a malformed
  `Endpoint`, an `Endpoint` without a host, and an `Endpoint` with userinfo
  redact credentials embedded in the URL.
- **`OAuth2Introspect` default client bounds its connections.** With no
  `HTTPClient` configured, the default client uses its own transport,
  copied once from `http.DefaultTransport` without initialising it, and
  allows at most 100 connections per introspection host (exact for
  HTTP/1.1). Concurrent cache misses therefore reuse connections instead of
  opening one per call. Construct the middleware at startup.
- **`Recoverer` does not corrupt a started response.** It writes its 500
  response only if the handler has not yet sent a status or body bytes.
- **Header values are allocated per request.** `CORS`, `NoCache`,
  `SetHeader`, the `JSON`, `XML`, and `Text` response helpers, and the
  router's automatic 405 and `OPTIONS` responses allocate each header-value
  slice per request. Code that mutates a header slice in place (for example
  `w.Header()[k][0] = ...`) affects only the current response.
- **`Compress` and `Logger` honour 1xx informational responses.** When a
  handler sends a 1xx status (for example 103 Early Hints) followed by a
  final status, `Compress` forwards the final status and `Logger` records
  the final status, using the same predicate as `net/http`.

## Operator-facing defaults requiring opt-in

Three middleware components ship with backwards-compatible defaults
that are unsafe in production. Each emits a `slog.Warn` at construction
time when the unsafe default is in effect; search startup logs for these
warnings.

| Middleware                 | Unsafe default                                  | Safe production setting                                |
|----------------------------|-------------------------------------------------|--------------------------------------------------------|
| `JWTAuth`                  | `RequireExpiry: false` (no `exp` ⇒ replayable) | `RequireExpiry: true` (RFC 8725 §4.4)                  |
| `RealIP()`                 | called with no CIDRs                            | `RealIP(&proxyCIDR)` with explicit trusted CIDR list   |
| `OAuth2Introspect`         | `AllowInsecureEndpoint: true`                   | leave `false` — HTTPS endpoint only                    |

The upstream README "Security defaults" section reproduces this matrix and
the recommended stack shown in "Composite token-handling stack" below.

## Thread-Safety Contract

All public `Mux` fields (`PanicHandler`, `NotFound`, `MethodNotAllowed`,
`GlobalOPTIONS`, `ErrorHandler`, `RedirectTrailingSlash`, `RedirectFixedPath`,
`CaseInsensitive`, `UseRawPath`, `UnescapePathValues`, `RedirectCode`,
`HandleMethodNotAllowed`, `HandleOPTIONS`, `PoolFastParams`,
`PoolRequestBundle`) **must be set before the first call to `ServeHTTP`**. On the first request these values are atomically
captured into a frozen `muxConfig` snapshot and every subsequent dispatch
reads from that snapshot — direct field mutation after serving begins is
ignored by the dispatch path and races with the snapshot's first read.

To reconfigure handlers after serving starts, mutate the field and then call
`Mux.Rebuild()`. `Rebuild()` atomically resets the snapshot and the lazy
NotFound/405/OPTIONS/redirect handler caches so the next request re-reads
every field. `Rebuild()` is safe to call concurrently with `ServeHTTP`.

`Use()` and `Pre()` are safe to call concurrently with `Handle()` during
route registration (before serving), but must not be called concurrently with
active requests.

## HTTP/2 Cleartext (h2c) Upgrade Handling

MuxMaster does not implement h2c-specific code. A request carrying
`Upgrade: h2c` and `Connection: Upgrade` headers is routed and dispatched
identically to any other request. By default, `net/http` does not perform
the h2c upgrade (RFC 7540 §3.4); the server responds with 200 OK and the
upgrade is silently ignored. To enable HTTP/2 over unencrypted TCP, operators must
explicitly configure the server (Go 1.27+: `http.Server.Protocols` with
`UnencryptedHTTP2`; earlier versions: use `golang.org/x/net/http2/h2c`);
MuxMaster's dispatcher operates identically in both cases. Routing and
middleware behaviour is unaffected by the Upgrade headers; treat h2c like
any other client request.

## Timeout Middleware

`middleware.Timeout` cancels the request context after the configured
duration but does not preempt the handler goroutine. Go has no preemption
primitive for blocked syscalls. **Handlers must actively check `ctx.Done()`**
(or use context-aware I/O) to be preempted. Handlers that ignore the context
continue running until they return, regardless of the timeout; under load
this accumulates goroutines and can exhaust memory or upstream connections.

Handlers MUST observe `ctx.Done()` on every blocking call (database, network,
file I/O). Use the `*Context` variants of standard-library APIs
(`sql.DB.QueryContext`, HTTP request bodies via `r.Context()`, and so on).

Example of a cooperative handler:

```go
func myHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    select {
    case result := <-doWork(ctx):
        w.Write(result)
    case <-ctx.Done():
        http.Error(w, "request timeout", http.StatusGatewayTimeout)
    }
}
```

## Slowloris / Server Timeouts

MuxMaster is an `http.Handler` and does not configure the underlying
`http.Server`. To mitigate Slowloris and similar attacks, always set timeouts
on your server:

```go
srv := &http.Server{
    Handler:           mux,
    ReadHeaderTimeout: 30 * time.Second,
    ReadTimeout:       60 * time.Second,
    WriteTimeout:      60 * time.Second,
    IdleTimeout:       120 * time.Second,
    MaxHeaderBytes:    1 << 20, // 1 MiB
}
```

## Accepted Known Limitations

### Route-existence timing

Dispatch to a **registered** route (200) and dispatch to an **unregistered**
path (404) at the same depth take measurably different time. The difference
is intrinsic to radix-tree lookup: httprouter, chi, and bunrouter show a
difference of the same order under the same measurement. If this is a
concern, use a WAF or add uniform response delays via middleware.

### Path normalisation accepted behaviour

The following routing behaviours are intentional and documented as operator
responsibilities:

- **`%61dmin` matches `/admin`** when `UseRawPath=false` (the default).
  `net/http` decodes `%61` to `a` during URL parsing per RFC 3986 §6.2.2.2,
  so the router sees `/admin`. To enforce byte-exact path matching set
  `mux.UseRawPath = true` — patterns then match against `r.URL.RawPath`,
  which preserves the percent-encoded form.

#### UseRawPath traversal

When BOTH `UseRawPath = true` AND `UnescapePathValues = true`, `%2f`
inside a single path segment is matched as one segment by the radix tree
(because `/` is preserved as the path separator only via a literal
slash) and is then DECODED in the captured param value. A request such
as `/files/..%2fetc%2fpasswd` against the route `/files/:filepath`
binds `:filepath` to the literal string `..\x2fetc\x2fpasswd` — i.e. the
captured value contains a real slash.

Handlers that pass `ParamsFromContext(...).Get("filepath")` to
`os.Open`, `http.FileServer`, or any URL/file API WITHOUT calling
`path.Clean` (and rejecting values that contain `..`) allow directory
traversal. The `CleanPath` middleware does NOT normalise post-decode
values; it only canonicalises the request path before dispatch.

**Mitigation.** Choose one of:

1. Leave `UnescapePathValues = false` (default) and let the handler
   call `url.PathUnescape` only after `path.Clean` and a `..` check.
2. Use `http.FileServer` (as in `examples/static-site/`), which applies
   `path.Clean` internally and prevents escaped traversal sequences from
   escaping the configured root.

When `UseRawPath` and `UnescapePathValues` are both set, MuxMaster
emits a one-time `slog.Warn` at the first `Handle`/`HandleFast` call
to make the misconfiguration visible in startup logs. Additionally,
`Mux.ServeFiles` and `Group.ServeFiles` PANIC at registration when both
flags are set, since `http.FileServer` would treat decoded slashes as path
separators inside the static-file root.

**UseRawPath + CleanPath interaction:** When `UseRawPath = true`, the router
matches against `r.URL.RawPath` (the percent-encoded form, which `net/http`
sets only when it differs from the decoded `r.URL.Path`). If `CleanPath` is
registered (as a Pre-gate), it cleans the decoded path via `path.Clean`, then
compares the result against a cleaned RawPath: if they diverge (e.g. for
`/a/%2e%2e/b`, `Path` is `/a/../b`, which cleans to `/b`, while `path.Clean`
does not treat the encoded `%2e%2e` in `RawPath` as `..`), `CleanPath` zeroes
`RawPath` to prevent an encoded traversal bypass. `CleanPath` therefore
protects against encoded traversal even with `UseRawPath = true`. Handlers
still must not pass RawPath values directly to file I/O; always decode and
validate.

- **Catch-all `*filepath` parameters carry raw bytes**, including any
  `..` traversal sequences. The router does NOT sanitise catch-all values
  — that is the boundary between router and storage backend. Handlers
  serving files MUST call `path.Clean` and must reject paths that escape
  their root (e.g. via `filepath.IsLocal` or by checking
  `filepath.Rel(root, joined)`). `Mux.ServeFiles` already delegates to
  `http.FileServer`, which performs path cleaning.

- **Path parameters may contain any byte, including CR/LF.** When
  `UseRawPath=false` (the default), `net/http` decodes percent-encoded
  sequences before routing. A request like `/users/%0D%0ASet-Cookie:%20hacked`
  becomes `/users/\r\nSet-Cookie: hacked` in `r.URL.Path`, and captured
  parameters carry the literal CR/LF bytes. Handlers must NOT echo
  parameters directly into headers or logs without escaping (use `url.PathEscape`
  for header injection mitigation, `html.EscapeString` for logs). The router
  itself does not strip or reject CR/LF; that is the handler's responsibility.

- **Order-independent static and parameter routes.** Routes can be registered
  in any order: `r.GET("/users/:id", h)` followed by `r.GET("/users/active", h)`
  succeeds without panic. Static routes are never shadowed by param routes at
  the same depth.

- **`RedirectFixedPath=true` discloses route existence via the redirect
  status.** The default is `false` precisely because path cleaning before
  dispatch can convert non-existent paths into observable hits. Leave the
  default unless you understand the disclosure trade-off.

- **`middleware.CleanPath` (registered via `Pre`) normalises dot segments
  before dispatch.** It does NOT bypass authentication — middleware
  registered via `Use` still wraps the dispatched handler — but it can
  cause requests to land on a different handler than the raw path
  suggests. CleanPath is intended for clients that emit `/foo/../bar`
  and similar; do not use it on endpoints where the literal path is
  semantically meaningful. **Ordering:** When composing CleanPath with
  path-inspecting Pre-gates (e.g. authorization checks that reject `/admin/*`),
  register CleanPath FIRST — a gate registered before CleanPath sees the
  raw path and can be bypassed by `/admin/../public`, `//admin`, or
  `%2e%2e`-encoded variants. **Query fragment in parameters:** Path
  parameters may contain `?` and `#` if percent-encoded in the request URL
  (`%3F` and `%23`). When handlers construct URLs with captured parameters,
  use `url.QueryEscape` (for query-string values) or `url.PathEscape` (for
  path segments) to prevent accidental query/fragment boundary shifts.

### Mount inner handler path sanitisation

`Mux.Mount` and `Group.Mount` forward requests to an inner handler with the
mount prefix stripped. The router does NOT clean the request path before
passing it to the inner handler. When the inner handler is another `*Mux`,
its routing receives the uncleaned path; if a request like `/mount/../secret`
reaches the mount (e.g. via `CleanPath` *after* the Mount prefix is consumed),
the inner handler receives `/../secret` and may dispatch it differently than
the outer router did. This is expected behaviour: inner handlers are responsible
for their own path validation and cleanup (e.g. calling `CleanPath` or validating
input). The inner `Mux` must register its own `CleanPath` Pre-gate.

### OAuth2 introspection cache and token revocation

The `OAuth2Introspect` middleware caches the introspection response
(keyed by sha256 of the bearer token) for `CacheTTL` seconds. If the IDP
revokes a token mid-cache-window, MuxMaster continues to honour the
cached `active=true` response until the cache entry expires — a window
of up to `CacheTTL`. For high-security endpoints, set
`OAuth2Options.CacheTTL = -1` to disable caching entirely; every request
will hit the introspection endpoint, but the singleflight group
coalesces concurrent calls for the same token, so the IDP load growth is
bounded by distinct-token concurrency rather than total request rate.

### OAuth2 singleflight leader cancellation

When `OAuth2Introspect` has concurrent requests for the same token, the
first request (the "leader") performs the introspection call while others
(the "followers") wait for its result via an in-process singleflight group
keyed by `sha256(token)` (MuxMaster has no external dependencies).
The leader performs introspection with a detached
`context.WithTimeout(context.WithoutCancel(r.Context()), 30s)`: the
leader's request-scoped context values are kept, but its cancellation and
deadline are not, so a leader cancelled by a `Timeout` middleware or a
client disconnect does not fail its followers. The 30s timeout is a fixed
upper bound on introspection latency; it is **not** configurable and is
separate from `CacheTTL` (which controls cache expiry, not introspection
timeout).

### Compress sniff-buffer per-connection memory

`middleware.Compress` buffers up to 8 KiB per stalled connection while
sniffing whether the response body is large enough to be compressed.
Total memory is `N × 8 KiB` for N concurrent stalled connections. The
middleware does not enforce a per-connection timeout; operators MUST set
`http.Server.ReadHeaderTimeout`, `http.Server.WriteTimeout` and a
connection cap on the listener to bound this exposure.

### Reflect-based ctx field offset

The tiered `reqBundle` optimisation (1 alloc per request with parameters)
relies on `unsafe.Add` over the offset of the private `ctx` field of
`*http.Request`, derived once via reflection at `init()`. If a future Go
version removes or renames the field, the offset cannot be resolved and
MuxMaster automatically falls back to `r.WithContext` (2 allocs per
request) without crashing. CI should cross-build against `gotip`
periodically to surface drift before a stable Go release.

### RealIP misconfiguration

`middleware.RealIP()` called with no trusted-proxy CIDR list trusts every
peer — any client can spoof `X-Forwarded-For` / `X-Real-IP` and the router
will accept it as the real client IP. This is only safe behind a single
trusted proxy that strips inbound XFF; in any other deployment it lets
clients spoof their address and defeats `ThrottlePerIP` and IP-based
access controls. Always pass the proxy CIDR list explicitly:

```go
proxyCIDR := netip.MustParsePrefix("10.0.0.0/8")
mux.Use(middleware.RealIP(&proxyCIDR))
```

A `slog.Warn` is emitted at construction time when `RealIP()` is called
without CIDRs.

### RealIP + ThrottlePerIP ordering

`middleware.ThrottlePerIP` with a nil `keyFn` keys on `r.RemoteAddr`. If
`RealIP` is not registered (or is registered AFTER `ThrottlePerIP`), every
request behind a reverse proxy carries the LB's own address as
`RemoteAddr` and the per-IP limit collapses to a global rate limit. Always
register `RealIP` first so `r.RemoteAddr` reflects the true client IP
before throttling decisions are made:

```go
mux.Use(middleware.RealIP(&proxyCIDR))           // first
mux.Use(middleware.ThrottlePerIP(50, ts, nil))   // then
```

A `slog.Warn` is emitted at construction time when `ThrottlePerIP` is
called with a nil keyFn.

### Startup-time route registration cost

Registering routes with `Handle`, `GET`, `POST`, and related methods incurs
a one-time cost at application startup. The router's radix tree uses
copy-on-write semantics to guarantee rollback safety (see "Registration
Panics Rollback Guarantee" below), and this cost grows linearly with the
route count, not quadratically.

Registration is performed only at application startup from the
application's own code; it is not reachable from HTTP requests, and dynamic
route registration after serving begins is unsupported. Applications that
construct their route table from untrusted configuration (e.g. a remote
endpoint, a database, or user-supplied YAML) should bound the maximum route
count, because whoever controls that configuration controls the startup time.

### Timing differences in authentication and throttling

Several code paths take measurably different time depending on their input.
The following are accepted as architectural or derived from the standard
library; removing them would require either Go runtime changes (outside
MuxMaster's scope) or padding that would add latency to every valid request.
Operators concerned about statistical timing measurements from an adjacent
network should rate-limit aggressively and monitor for prefix-scan probes.

- **`BasicAuth` valid versus invalid password.** The two outcomes run
  different code after authentication (`next.ServeHTTP` versus
  `http.Error` with `WWW-Authenticate`), so they differ in time by design.
  The credential comparison itself is constant time over all users.
- **`BasicAuth` user exists versus does not exist.** The lookup scans all
  registered users in constant time; the remaining difference comes from
  other code paths (handler dispatch, middleware chain).
- **`BasicAuth` password length.** Both passwords are hashed to 32-byte
  digests before the comparison, so the comparison time does not depend
  on the length of the supplied password.
- **`APIKey` hit versus miss.** The `map[[32]byte]string` lookup reveals
  whether a key exists. `APIKey` keeps the map-based lookup because a
  constant-time alternative would compare every registered key with
  `subtle.ConstantTimeCompare` on every request (O(n) per request), which
  is only worthwhile for very small key sets.
- **`ThrottleBacklog` fill level.** The fast-path acquisition is a single
  atomic load and compare-and-swap, so its time does not depend on how
  close the in-use count is to the configured limit.
- **Route existence.** See "Route-existence timing" above.
- **`JWTAuth` HS256 versus ES256.** The two verification paths differ by a
  negligible amount.
- **`JWTAuth` `alg=none` versus HS256.** `JWTAuth` never accepts the
  unsigned `alg=none` format: `"none"` is not a supported algorithm, and
  listing it in `Algorithms` panics at construction. A token whose header
  declares `alg=none`, sent to an HS256-configured `JWTAuth`, is rejected
  with 401 at the algorithm allow-list check. That rejection path takes a
  different time from HMAC verification, which reveals only that the token
  was rejected.
- **ECDSA zero signature versus random signature.** The standard library's
  `ecdsa.Verify` returns at slightly different times depending on signature
  shape. The signature is rejected either way; the difference only reveals
  that the signature is zero, which is publicly observable in the request.
- **`OAuth2Introspect` cache hit, active versus inactive.** The 401 and 200
  response paths differ in execution length by design. The difference
  reveals nothing beyond what the HTTP status code already exposes.

### Composition of timing differences

Three independent timing differences can combine to enable user-correlation
in multi-tenant deployments or reconnaissance of IdP infrastructure:

1. **JWT algorithm-path timing:** Configuring both HMAC (HS256) and RSA
   (RS256) algorithms reveals, through response latency, which algorithm
   family the server accepted the token with (see "JWT Mixed-Family
   Algorithms" below).
2. **OAuth2 introspection cache timing:** The local token cache hits or
   misses depending on whether a token was previously seen by **any**
   client, so the response time reveals whether a token was recently
   active on the service.
3. **Route existence timing:** Dispatch to a registered route and to an
   unregistered path at the same depth take different time (see
   "Route-existence timing" above).

**Composite vector:** A client can submit multiple tokens and measure
response latencies to infer: (a) which algorithm family is configured,
(b) which tokens have been recently used by other clients, and
(c) which API paths are registered. Combined, this enables correlation
of tokens across different clients if they are retried by multiple users,
or reconnaissance of the internal endpoint topology.

**Recommended deployment posture:**

- **Option 1 (strict):** Configure a single algorithm family per endpoint
  (HS256 or RS256, not both). This removes the algorithm-path timing
  difference entirely.

- **Option 2 (layered):** If mixed algorithms are required, apply uniform
  response-time padding at the edge (reverse proxy, WAF, or a `Pre`-registered
  middleware that adds fixed or random delays). This compresses the observable
  latency deltas below the statistical threshold reachable in a few hundred
  requests.

- **Option 3 (rate-limit):** Use `middleware.ThrottlePerIP` to rate-limit
  token validation attempts to tens of requests per minute per client.
  This makes collecting sufficient samples for statistical analysis infeasible
  in practice.

- **Option 4 (combination):** Deploy a reverse proxy or WAF with DDoS
  scrubbing and request rate limiting, plus strict algorithm configuration
  at the IdP level (do not mix families).

The OAuth2 cache-hit/miss difference is inherent to token caching: set
`OAuth2Options.CacheTTL = -1` to disable caching if this exposure is
unacceptable. The route-existence difference is intrinsic to any radix-tree
router; httprouter, chi, and bunrouter exhibit similar timing.

### JWT Subject Claim Validation

`JWTAuth` does NOT validate the `sub` (subject) claim — it performs only
signature verification and (optionally) expiry checking. When a JWT is
verified as authentic, the `sub` claim value is **not** authoritative for
identifying the token's bearer; anyone with access to a single valid
signing key can mint tokens with arbitrary `sub` values. Authorization
logic must bind the token to the (issuer, subject) pair: do not authorise
a request based on `sub` alone, and do not assume that two tokens with the
same `sub` but different `iss` (issuer) claims belong to the same principal.
Extract both `iss` and `sub`, verify they match your expected issuer and
subject registry, and enforce additional checks (e.g. IP, user-agent, request
time) if needed.

### JWT Expiry Claim Handling

`JWTAuth` treats `exp: null` (the JSON null value) and `exp: 0` as if the
`exp` claim is absent — the token is never expired by the JWT expiry check.
When `RequireExpiry: true` (the recommended production setting), a token
without an `exp` claim or with `exp` null/0 fails validation. When
`RequireExpiry: false` (the default, kept for backward compatibility), such
tokens pass expiry validation (though they may fail on other checks like issuer or
signature). Tokens created by non-standard JWT libraries or hand-crafted
edge cases must carry a numeric, positive Unix timestamp in the `exp` claim
for expiry validation to work correctly.

### JWT Mixed-Family Algorithms

`JWTAuth` configured with HS\* and RS\*/ES\* algorithms in the same
`Algorithms` list reveals the algorithm code path through response latency:
HMAC and RSA verification take measurably different time. A client
submitting tokens labelled with different `alg` values can determine which
path the server runs from the response time alone, which narrows the search
space for algorithm-confusion attacks (RFC 8725 §3.1). Configure each
endpoint with a single algorithm family. Mixed-family configuration emits a
`slog.Warn` at construction time.

### Recoverer Middleware Panic Logging

`middleware.Recoverer` and `middleware.RecovererWithLogger` catch panics
and log, at Error level (via the slog default logger or the supplied
logger), the raw panic value and the full stack trace. A panic that
includes sensitive data (e.g. `panic("user_id=%d, api_key=%s", userID, key)`)
will be logged verbatim. Log sinks (files, centralized logging services,
SIEM systems) must be protected with appropriate access controls and
sanitisation rules. Do not use Recoverer as the sole defense against
information leakage in panic messages — validate and sanitise panic
recovery output at the log ingestion layer.

### SIEM Re-interpretation of Log Fields

MuxMaster's `Logger` middleware writes one plain-text line per request
(`<time> <method> <path> <status> <duration>`) containing user-controlled
data: the request method and path. When either contains a control byte, a
non-ASCII byte, `"` or `\`, it is written with Go string-literal escaping
(`strconv.QuoteToASCII`, without the surrounding quotes), so a CR/LF in the
path appears as the literal two-character sequences `\r` and `\n`, never as
raw bytes. No raw CR, LF, or TAB byte reaches the log. A SIEM or log parser
that later unescapes these sequences may re-interpret them as control
characters. This is a log-consumer concern. Operators must configure their
log aggregation layer to treat these escapes as text.

### BasicAuth Brute-Force

`middleware.BasicAuth` does not rate-limit authentication attempts. Compose it
with `middleware.ThrottlePerIP` to mitigate online brute-force:

```go
mux.Use(
    middleware.ThrottlePerIP(10, time.Second, nil),
    middleware.BasicAuth("realm", creds),
)
```

### BREACH and response compression

`middleware.Compress` does not mitigate the BREACH attack. When a response
compressed with gzip echoes one URL or query parameter that the client
controls alongside a secret, the client can recover the secret one
character at a time by observing the compressed response size.

**Mitigations**, in decreasing order of safety:

1. **Do not compress endpoints that echo user-controlled input near secrets.**
   The simplest fix: register `middleware.Compress` only on routes that do
   not echo client-controlled data into the body, or build a separate
   middleware chain for sensitive endpoints.

2. **Move secrets out of the response body.** Put OAuth2 scopes, CSRF
   tokens, session IDs and JWTs in headers, cookies, or dedicated endpoints
   that are never reachable via client-controlled input.

3. **Variable-length random padding.** If 1 and 2 are not feasible, append
   a random-length (>= 256 bytes, length randomised per request) random
   payload to the response body. Fixed-length padding is **not**
   sufficient — the random content compresses to similar sizes per request.

Example mitigation pattern (variable-length random padding):

```go
func tokenInfo(w http.ResponseWriter, r *http.Request) {
    q := r.URL.Query().Get("q")
    padN := 256 + rand.Intn(256) // length itself is randomised
    pad := make([]byte, padN)
    _, _ = rand.Read(pad)
    body := fmt.Sprintf(`{"scope":"%s","query":"%s","pad":"%x"}`,
        oauthScope, q, pad)
    _, _ = io.WriteString(w, body)
}
```

MuxMaster cannot apply these mitigations on the operator's behalf because
they require domain knowledge of which response fields are secret vs
user-controlled.

### Registration Panics Rollback Guarantee

If `Handle` (or any route registration method) panics — for example due to a
route conflict — the radix tree is guaranteed to remain unchanged. The two-phase copy-on-write implementation (path copying instead of full tree cloning) ensures that any mutation panicking mid-insertion rolls back atomically. **Do not catch and ignore registration panics.** Let them crash `main()` during development — the tree's consistency is protected, but the error is a sign of a bug (route conflict, invalid pattern, etc.). The rollback guarantee is enforced by test `TestRegistrationRollback_PanicMidInsert_LiveTreeUntouched` in the test suite.

### Recoverer Must Be Outermost

`middleware.Recoverer` (or `RecovererWithLogger`) only catches panics in
middleware and handlers registered *inside* it. Register it as the outermost
middleware — or use `r.Pre(middleware.RecovererWithLogger(logger))` — to
ensure it wraps the full dispatch chain.

### Composite token-handling stack

A bearer-token endpoint that combines OAuth2 introspection (or JWT) with
per-IP throttling and reverse-proxy IP forwarding depends on FOUR distinct
settings. Leaving any one of them off weakens the whole stack: a passive
observer can capture a bearer token over plaintext HTTP, replay it
indefinitely (no `exp`), and forge `X-Forwarded-For` to bypass the per-IP
throttle. The recommended stack turns ALL of the following invariants ON:

| # | Invariant                                              | Mechanism                                            | Default |
|---|--------------------------------------------------------|------------------------------------------------------|---------|
| 1 | Introspection endpoint MUST be HTTPS                   | `OAuth2Options.Endpoint` panics on non-HTTPS         | enforced |
| 2 | JWT tokens MUST carry `exp`                            | `JWTOptions.RequireExpiry = true`                    | OFF (opt-in) |
| 3 | XFF must be parsed RIGHTMOST past trusted CIDRs        | `RealIP(trustedCIDRs...)` walks rightmost-leftward   | enforced when `len(trustedCIDRs)>0` |
| 4 | Per-IP table size must be CAPPED                       | `ThrottlePerIPCapped` (default `100_000`)            | enforced via `ThrottlePerIP` wrapper |

A configuration that leaves any of these OFF (e.g. opens
`AllowInsecureEndpoint` "for testing", omits `RequireExpiry`, calls
`RealIP()` with no CIDRs, or builds a custom throttle without a cap)
is unsafe. The recommended pattern is:

```go
trusted, _ := netip.ParsePrefix("10.0.0.0/8")
mux.Pre(
    middleware.RealIP(&trusted),                  // (3) rightmost XFF
)
mux.Use(
    middleware.ThrottlePerIP(100, 5*time.Second, nil), // (4) capped per-IP
    middleware.JWTAuth(middleware.JWTOptions{
        Secret:        secret,
        Algorithms:    []string{"HS256"},
        RequireExpiry: true,                       // (2) reject no-exp
    }),
    // OR use OAuth2Introspect with HTTPS endpoint:
    // middleware.OAuth2Introspect(middleware.OAuth2Options{
    //   Endpoint: "https://idp.example/introspect",  // (1) HTTPS only
    //   ...
    // }),
)
```

The [JWT](/examples/jwt) and [OAuth2](/examples/oauth2) examples
demonstrate the full pattern.

### Layered panic recovery

Up to four layers may catch a panic. From outermost to innermost:

| Layer | Frame | Catches |
|---|---|---|
| `net/http` per-connection recover | around the whole handler | anything that escapes every layer below; logs "http: panic serving ..." and closes the connection |
| `Mux.PanicHandler` (when set) | deferred in `Mux.ServeHTTP` | panics from `Pre` middleware and from dispatch (`Use` / `UseFast` middleware, handlers of both route types), unless an inner layer catches them first |
| `RecovererWithLogger` registered with `Pre` | around every `Pre` middleware registered after it and the dispatch | panics from those `Pre` middleware, from dispatch and from both route types |
| `RecovererWithLogger` registered with `Use` | around the `Handle` routes registered after it | panics from the `Use` middleware registered after it and from those handlers |

A panic is caught by the **innermost** recovering frame that encloses it.
Consequently, with a `Pre`-registered `RecovererWithLogger`, a panic in a
handler is handled by the Recoverer (plain 500) and `PanicHandler` never
sees it; `PanicHandler` then only receives panics raised by `Pre`
middleware registered before the Recoverer.

**Boundary rules.**

1. `Mux.PanicHandler` MUST NOT panic. Its frame is outside every
   MuxMaster-owned recovery, including a `Pre`-registered Recoverer, so a
   secondary panic propagates to `net/http`, which closes the connection
   mid-response. There is no goroutine leak and no process crash, but the
   client sees a reset stream — confusing for HTTP/2 multiplexing and
   reverse-proxy retries.
2. For uniform recovery across stdlib and FastHandler routes, either set
   `Mux.PanicHandler` (it covers both route types and `Pre` middleware)
   or register `RecovererWithLogger` as the first `r.Pre(...)` middleware.
   A `Use`-registered Recoverer cannot cover `HandleFast` routes (see
   "Pre vs Use security boundary" below).
3. Choose one primary mechanism. If you configure both, decide which one
   owns the 500 response: the innermost wins.

### Pre vs Use security boundary

`Mux.Pre()` and `Mux.Use()` register middleware in different positions
of the dispatch pipeline. Mistaking one for the other has direct
authentication/authorisation consequences when `HandleFast` routes are
also in play.

| Middleware family        | Wraps `Handle` (stdlib)? | Wraps `HandleFast`?       | Mode                                |
|--------------------------|--------------------------|---------------------------|-------------------------------------|
| `r.Pre(...)`             | YES                      | YES                       | Outside dispatch — `http.Handler`.  |
| `r.Use(...)`             | YES                      | NO — panics at register.  | Inside dispatch — `http.Handler`.   |
| `r.UseFast(...)`         | NO                       | YES                       | Inside dispatch — `FastMiddleware`. |

**Implications.**

- An auth gate (e.g. `JWTAuth`) registered via `r.Use(...)` does NOT cover
  `HandleFast` routes — MuxMaster panics at `HandleFast` registration to
  expose this immediately. Either (a) register the auth via `r.Pre(...)` so
  it covers BOTH route types, or (b) duplicate the auth as a
  `FastMiddleware` and register it via `r.UseFast(...)`.
- Because `Pre` runs OUTSIDE the dispatch (in `Mux.ServeHTTP` before the
  radix-tree lookup), it sees the path BEFORE any route-specific handler
  decision — useful for `CleanPath`, `RealIP`, `RecovererWithLogger`,
  request ID assignment, or any policy that must be uniform across the
  router.
- `Use` runs INSIDE the dispatch (after the route is matched) and is
  baked into the wrapped handler at registration time. Its only path
  to a fast route is via `UseFast`.

Operators reviewing an auth/CORS/rate-limit policy by reading `Use(...)`
calls SHOULD also inspect `HandleFast(...)` registrations and
`UseFast(...)` calls; otherwise a fast-route bypass is invisible.

**Exception — Asterisk-form `OPTIONS * HTTP/1.1`:** Auth gates registered
in `Pre()` do not see asterisk-form OPTIONS requests by default, because
`net/http` intercepts and answers them before `Mux.ServeHTTP` is called
(under the default server configuration `DisableGeneralOptionsHandler ==
false`). This does not expose any route: `net/http`'s response is fixed
(200 OK, `Content-Length: 0`, no `Allow` header) and route-independent. To
route these requests through MuxMaster and apply `Pre()` middleware to them,
set `http.Server.DisableGeneralOptionsHandler = true`.

## HTTP/1.1 Request Smuggling

Request smuggling (CL.TE / TE.CL / TE.TE) is handled by Go's `net/http`
package at the framing layer. No action is required at the MuxMaster level.

## Differential Error Responses

Differential error responses (404 vs 405 vs 301) are intentional HTTP
semantics and are present in all HTTP routers. If normalising error responses
is required for your threat model, use a WAF or a custom `NotFound` /
`MethodNotAllowed` handler.

## ThrottlePerIPCapped saturation

When the per-IP throttle table reaches `maxTableSize` and every slot is
in active use (`refs > 0`), new client IPs receive HTTP 503 immediately.
A client controlling at least `maxTableSize` distinct IPs (default
100 000) and keeping their requests open can sustain this lockout for
as long as the connections remain. A request that times out releases its
slot, so the table drains when those connections end.

**Required operator mitigations:**

- Deploy upstream DDoS scrubbing (Cloudflare, AWS Shield, GCP Cloud Armor).
- Configure `http.Server{ReadHeaderTimeout, IdleTimeout, ReadTimeout}` so
  slow-handler connections cannot hold throttle slots indefinitely.
- Reduce `maxTableSize` for high-sensitivity endpoints — the cap
  intentionally trades fairness for memory safety.

This is accepted behaviour: the cap exists precisely to prevent
unbounded memory growth when the set of client IPs keeps changing.

## Upstream source

The security-sensitive behaviours on this page (the `Pre` versus `Use` boundary, redirect encoding, the `ServeFiles` raw-path guard, panic recovery) are implemented in [`mux.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/mux.go), [`group.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/group.go), and [`tree.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/tree.go) at the v1.3.0 tag; the middleware behaviours are in the [`middleware`](https://github.com/FlavioCFOliveira/MuxMaster/tree/v1.3.0/middleware) package.
