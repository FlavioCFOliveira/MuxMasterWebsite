---
datePublished: 2026-05-08
---

# JWT example

Bearer-token authentication via the `JWTAuth` middleware, configured with `RequireExpiry: true` per RFC 8725 §4.4. The example issues HS256 tokens with the standard library only (MuxMaster validates tokens but intentionally does not issue them — issuance is application-specific) and protects an `/api` group with the validating middleware.

## Step 1 — Define the token payload

The payload mirrors the standard JWT claims plus one custom claim. `sub`, `iat`, and `exp` are validated by `mw.JWTAuth` once the token reaches the protected route; the non-standard `name` claim travels in `JWTClaims.RawPayload` and is read by the handlers.

```go
type tokenPayload struct {
    Sub  string `json:"sub"`  // subject — user ID
    Name string `json:"name"` // custom claim — username
    IAT  int64  `json:"iat"`  // issued at (Unix seconds)
    EXP  int64  `json:"exp"`  // expires at (Unix seconds)
}
```

`exp` is critical: with `RequireExpiry: true` (Step 6), tokens without it are rejected outright, which closes the "valid-forever once stolen" risk RFC 8725 §4.4 calls out.

## Step 2 — Sign the token with HS256

The token is the canonical compact form `header.payload.signature`, with each segment base64url-encoded (no padding) and the signature an HMAC-SHA256 over `header + "." + payload`. The standard library covers everything; no third-party JWT package is required.

```go
func signToken(c tokenPayload, secret []byte) (string, error) {
    const rawHeader = `{"alg":"HS256","typ":"JWT"}`
    hdr := base64.RawURLEncoding.EncodeToString([]byte(rawHeader))

    payloadJSON, err := json.Marshal(c)
    if err != nil {
        return "", err
    }
    pld := base64.RawURLEncoding.EncodeToString(payloadJSON)

    signingInput := hdr + "." + pld
    mac := hmac.New(sha256.New, secret)
    _, _ = io.WriteString(mac, signingInput)
    sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

    return signingInput + "." + sig, nil
}
```

The header is a constant string — the algorithm whitelist on the validating side (Step 6) is what guarantees an attacker cannot trick the server into accepting `"alg":"none"`.

## Step 3 — Construct the router with global middleware

`RequestID`, `Logger`, and `Recoverer` belong on every request. `Use` is the right call site — these middlewares run after route resolution and observe the actual matched handler in their logs.

```go
r := mm.New()

r.Use(
    mw.RequestID(),
    mw.Logger(os.Stdout),
    mw.RecovererWithLogger(log),
)
```

`RecovererWithLogger` writes the panic + stack trace through the same structured logger as the request log, so a single grep finds both the panic and the surrounding request lifecycle.

## Step 4 — Wire JSON-shaped error and not-found handlers

The default not-found and error handlers write plain text. For an API the customer expects JSON, so the example replaces both with handlers that return `{"error": "..."}` and a status code mapped from `mm.HTTPError` when the handler returns one.

```go
r.NotFound = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    _ = mm.JSON(w, http.StatusNotFound, errMsg("not found"))
})
r.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
    code := http.StatusInternalServerError
    var he mm.HTTPError
    if errors.As(err, &he) {
        code = he.StatusCode()
    }
    _ = mm.JSON(w, code, errMsg(err.Error()))
}
```

`errors.As(err, &he)` is the canonical Go pattern for unwrapping a typed error; it survives any number of `fmt.Errorf("%w", ...)` wrappings between the handler and the dispatcher.

## Step 5 — Issue tokens at `POST /auth/login`

`POSTE` is the error-returning variant of `POST` (the `E` suffix). The handler decodes the body, looks up the user, and issues a token — any failure is signalled by returning an error, which the `ErrorHandler` from Step 4 turns into the JSON response.

```go
r.POSTE("/auth/login", func(w http.ResponseWriter, r *http.Request) error {
    var body struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        return mm.Error(http.StatusBadRequest, errors.New("invalid JSON"))
    }
    u, ok := findUser(body.Username, body.Password)
    if !ok {
        return mm.Error(http.StatusUnauthorized, errors.New("invalid credentials"))
    }
    token, err := issueToken(u.id, body.Username, secret, tokenTTL)
    if err != nil {
        return err
    }
    return mm.JSON(w, http.StatusOK, map[string]string{"token": token})
})
```

`mm.Error(code, err)` wraps the error with the desired HTTP status; the `ErrorHandler` reads the status back via the `HTTPError` interface (Step 4).

## Step 6 — Configure `JWTAuth` with `RequireExpiry: true`

This is the validating side. The configuration is hardened per `SECURITY.md` CDX-S8-001: the algorithm allow-list is restricted to `HS256` (so a token forged with `alg: none` is rejected), and `RequireExpiry: true` rejects any token without an `exp` claim.

```go
jwtAuth := mw.JWTAuth(mw.JWTOptions{
    Secret:        secret,
    Algorithms:    []string{"HS256"},
    RequireExpiry: true,
})
```

The middleware does the rest: signature verified with constant-time HMAC comparison, `exp` and `nbf` checked against `time.Now()` (with optional `ClockSkew`), and the RFC 7515 §4.1.11 "crit" header rejected. On success, `JWTClaims` is injected into the request context.

## Step 7 — Apply the middleware to a `/api` group

Group-scoped `Use` is the canonical way to put the validating middleware in front of a family of routes. Routes registered on `api` inherit it; routes outside the group (`/auth/login`, `/health`) stay public.

```go
api := r.Group("/api")
api.Use(jwtAuth)
```

When `/auth/refresh` needs the same protection but does not belong inside `/api`, the example wires the middleware inline on that single route (`r.Handle(http.MethodPost, "/auth/refresh", jwtAuth(refresh))`) rather than create a one-route group.

## Step 8 — Read claims inside protected handlers

Inside the protected handlers the claims live on the request context. `GetJWTClaims` returns them; `usernameFromClaims` decodes the custom `name` claim out of `RawPayload` (the standard claims are typed fields on `JWTClaims`).

```go
api.GET("/me", func(w http.ResponseWriter, r *http.Request) {
    c, _ := mw.GetJWTClaims(r.Context())
    _ = mm.JSON(w, http.StatusOK, map[string]any{
        "user_id":  c.Subject,
        "username": usernameFromClaims(c),
        "issued":   c.IssuedAt.UTC().Format(time.RFC3339),
        "expires":  c.ExpiresAt.UTC().Format(time.RFC3339),
    })
})
```

The `_` on the second return value of `GetJWTClaims` is safe inside the protected group because the middleware short-circuits with 401 when claims are absent — the handler only runs after a successful validation.

## Step 9 — Serve with hardened timeouts and graceful shutdown

The same timeout set as the graceful-shutdown example, plus the same signal-driven shutdown. The pattern is repeated across examples on purpose: every production server should set `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`, and every server should drain on `SIGINT`/`SIGTERM`.

```go
srv := &http.Server{
    Addr:              ":8080",
    Handler:           r,
    ReadHeaderTimeout: 30 * time.Second,
    ReadTimeout:       60 * time.Second,
    WriteTimeout:      60 * time.Second,
    IdleTimeout:       120 * time.Second,
    MaxHeaderBytes:    1 << 20,
}
```

For the full goroutine-driven start and `Shutdown(ctx)` drain, see the graceful-shutdown example — the JWT example uses the identical pattern in the same file.

## Common questions

<section data-conversation="jwt-patterns">

### How do I verify a JWT on every protected request?

Mount the protected routes inside a group, then call `g.Use(jwt.Authenticate(verifier))` once. The middleware extracts the token from the `Authorization: Bearer <token>` header, verifies it with the supplied `Verifier`, and attaches the parsed claims to the request context.

### What happens if the token is expired?

The verifier returns an error and the middleware responds with `401 Unauthorized` + `WWW-Authenticate: Bearer error="invalid_token"`. The example respects RFC 6750 so well-known clients (curl, httpie, OpenAPI consumers) surface a precise error message instead of a generic 401.

### How do I read the user id from inside a protected handler?

Read the claims from the request context with `mw.GetJWTClaims(r.Context())` and pull the `Subject` field. Custom claims live in `JWTClaims.RawPayload`; the example's `usernameFromClaims` helper decodes the non-standard `name` claim out of that byte slice.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/jwt/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/jwt/main.go) at the v1.0.1 tag. The upstream file also includes the in-process user store, the `usernameFromClaims` helper, and the `/auth/refresh` route — follow the link for the full program.
