# OAuth2 example

OAuth 2.0 token introspection via the `OAuth2Introspect` middleware (RFC 7662). Reach for it when bearer tokens must be validated against an authorisation server rather than verified locally. The example also demonstrates the four invariants of the hardened token-handling stack required by `SECURITY.md` CDX-S8-001.

## Step 1 — Stand up a TLS introspection endpoint for the demo

The introspection middleware refuses plaintext endpoints — it returns an error at construction time when the URL scheme is `http://` (CDX-S8-001 invariant 1). To keep the example self-contained, the program starts an in-process TLS test server that mimics the authorisation server's introspection response.

```go
idp := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    _ = r.ParseForm()
    token := r.FormValue("token")
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(map[string]any{
        "active": token == "good-token",
        "sub":    "user-1",
        "exp":    time.Now().Add(60 * time.Second).Unix(),
    })
}))
defer idp.Close()
```

In production the `idp.URL` is the real authorisation server's introspection endpoint and the per-request response carries the actual token's claims and validity.

## Step 2 — Declare the trusted reverse-proxy CIDR for RealIP

`RealIP` walks `X-Forwarded-For` from the rightmost entry and stops at the first non-trusted hop, which defeats attacker-injected leftmost values (MSR-2026-0065). The trusted-proxy list MUST be the actual edge proxy network — never `0.0.0.0/0`.

```go
trustedProxy, err := netip.ParsePrefix("127.0.0.0/8")
if err != nil {
    log.Error("invalid CIDR", "err", err)
    os.Exit(1)
}
```

The example uses `127.0.0.0/8` because the demo runs entirely on localhost; substitute the real edge subnet (the load balancer's private CIDR or a single proxy IP) before deploying.

## Step 3 — Construct the router and attach RealIP as a Pre middleware

`Pre` middleware runs before the radix-tree lookup and sees the raw URL. `RealIP` belongs there because subsequent middleware (the per-IP throttle) and downstream handlers MUST observe the real client address, not the proxy's address.

```go
r := mm.New()
r.Pre(mw.RealIP(&trustedProxy))
```

The pointer-to-prefix argument is the trusted-proxy list; the middleware accepts a slice when more than one CIDR is trusted.

## Step 4 — Bound memory with a per-IP throttle

`ThrottlePerIP` enforces a maximum request rate per client IP and stores its counters in a bounded table (default `DefaultThrottlePerIPMaxTableSize`). The bound is what makes the throttle safe under attack — an adversary churning unique IPs cannot exhaust process memory (CDX-S8-001 invariant 2).

```go
r.Use(mw.ThrottlePerIP(50, 2*time.Second, nil))
```

The arguments are: 50 requests per 2-second window per IP, no custom rejection handler (the middleware writes 429 with a `Retry-After` header).

## Step 5 — Validate bearer tokens with `OAuth2Introspect`

`OAuth2Introspect` is the hardened stack's centrepiece (CDX-S8-001 invariant 1). It extracts the bearer token from the `Authorization` header, calls the introspection endpoint, caches the response for `CacheTTL`, and rejects requests whose tokens come back inactive. Cached active responses save the round-trip until the cache expires.

```go
r.Use(mw.OAuth2Introspect(mw.OAuth2Options{
    Endpoint:   idp.URL, // https:// from httptest.NewTLSServer
    HTTPClient: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}},
    CacheTTL:   30 * time.Second,
}))
```

The `InsecureSkipVerify` setting is exclusive to the demo, where the test server's certificate is self-signed; production code MUST omit the custom `Transport` (or use a trust store containing the authorisation server's CA).

## Step 6 — Read the introspected claims inside the protected handler

After the middleware accepts a token, the parsed claims live on the request context. The handler retrieves them with `GetOAuth2Claims(ctx)` and reflects the subject and active flag in a JSON response. There is no token parsing or verification inside the handler — that work belongs to the middleware.

```go
r.GET("/api/me", func(w http.ResponseWriter, req *http.Request) {
    claims, ok := mw.GetOAuth2Claims(req.Context())
    if !ok {
        http.Error(w, "missing claims", http.StatusInternalServerError)
        return
    }
    _ = mm.JSON(w, http.StatusOK, map[string]any{
        "subject": claims.Subject,
        "active":  claims.Active,
    })
})
```

The `!ok` branch is a defensive sanity check; under the hardened stack it cannot fire because the middleware short-circuits inactive tokens with 401 before the handler runs.

## Step 7 — Start the server and surface the demo invocation

The example listens on `:8080` and logs the curl command a reader can paste to exercise the protected route. `errors.Is(err, http.ErrServerClosed)` is the canonical way to filter the benign "server stopped on Shutdown" signal out of the unhappy-path log.

```go
srv := &http.Server{Addr: ":8080", Handler: r}
log.Info("oauth2 hardened stack listening — see SECURITY.md CDX-S8-001",
    "addr", srv.Addr,
    "try", fmt.Sprintf("curl -H 'Authorization: Bearer good-token' http://localhost:8080/api/me"))
if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
    log.Error("server error", "err", err)
}
```

The fourth invariant in the SECURITY.md stack — `Recoverer` plus a `PanicHandler` — is intentionally omitted from this example to keep the listing readable; the routing and authn examples both demonstrate it.

## Common questions

<section data-conversation="oauth2-patterns">

### How do I protect an API route with OAuth2 token introspection?

Wrap the protected routes (or the entire router) with `mw.OAuth2Introspect`. The middleware extracts the bearer token, calls the introspection endpoint, caches the response, and rejects inactive tokens with 401 before the handler runs. The handler reads the parsed claims from the request context with `mw.GetOAuth2Claims`.

### Why does `OAuth2Introspect` reject `http://` endpoints?

CDX-S8-001 invariant 1 — token introspection MUST run over TLS to prevent the network from observing or substituting tokens. The middleware refuses non-HTTPS endpoints at construction time; the `AllowInsecureEndpoint` escape hatch exists for tests only and is never appropriate in production.

### How does the per-IP throttle stay safe under a churning-IP attack?

`ThrottlePerIP` stores its counters in a bounded table (default `DefaultThrottlePerIPMaxTableSize`). When the table fills, the middleware evicts entries — the memory footprint stays bounded regardless of how many distinct IPs the attacker uses (CDX-S8-001 invariant 2).

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/oauth2/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/oauth2/main.go) at the v1.0.1 tag. Follow that link for the complete file (imports, build tags, package comment).
