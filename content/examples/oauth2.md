# OAuth2 example

OAuth 2.0 token introspection via the `OAuth2Introspect` middleware (RFC 7662). Reach for it when bearer tokens must be validated against an authorisation server rather than verified locally.

## Source

```go
// Package main demonstrates the hardened OAuth2 introspection stack.
//
// This example pairs OAuth2Introspect with the four invariants required by
// SECURITY.md "Composite token-handling stack" (CDX-S8-001):
//
//  1. HTTPS-only introspection endpoint (OAuth2Introspect rejects http://
//     unless AllowInsecureEndpoint is explicitly set — testing only).
//  2. Per-IP throttle with a bounded table (DefaultThrottlePerIPMaxTableSize)
//     so an attacker cannot exhaust memory by churning unique IPs.
//  3. RealIP configured with explicit trusted-proxy CIDRs so the rightmost
//     non-trusted XFF entry is selected (defends against attacker-injected
//     leftmost values, MSR-2026-0065).
//  4. Recoverer + PanicHandler to keep the process alive on a misbehaving
//     handler.
//
// Run: go run . (talks to a fake https introspection server in this demo).
package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"time"

	mm "github.com/FlavioCFOliveira/MuxMaster"
	mw "github.com/FlavioCFOliveira/MuxMaster/middleware"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Mock IDP — in production this is your auth server. We use TLS because
	// OAuth2Introspect rejects plaintext endpoints (CDX-S8-001 invariant 1).
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

	// Trusted reverse-proxy CIDR — MUST be the actual edge proxy network.
	trustedProxy, err := netip.ParsePrefix("127.0.0.0/8")
	if err != nil {
		log.Error("invalid CIDR", "err", err)
		os.Exit(1)
	}

	r := mm.New()

	// ── Invariant 3: rightmost-walk XFF (RealIP). Pre wraps every route. ─
	r.Pre(mw.RealIP(&trustedProxy))

	// ── Invariant 4: bounded per-IP throttle. ─
	r.Use(mw.ThrottlePerIP(50, 2*time.Second, nil))

	// ── Invariant 1: HTTPS-only introspection endpoint. ─
	r.Use(mw.OAuth2Introspect(mw.OAuth2Options{
		Endpoint:   idp.URL, // https:// from httptest.NewTLSServer
		HTTPClient: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}},
		CacheTTL:   30 * time.Second,
	}))

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

	srv := &http.Server{Addr: ":8080", Handler: r}
	log.Info("oauth2 hardened stack listening — see SECURITY.md CDX-S8-001",
		"addr", srv.Addr,
		"try", fmt.Sprintf("curl -H 'Authorization: Bearer good-token' http://localhost:8080/api/me"))
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server error", "err", err)
	}
}
```

[`examples/oauth2/main.go` at v1.0.1](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/oauth2/main.go)

## Common questions

<section data-conversation="oauth2-patterns">

### How do I add OAuth2 login with an external provider?

Register two handlers — `/auth/login` redirects to the provider's authorisation URL with a state parameter, and `/auth/callback` exchanges the returned code for a token and creates a local session. The example program ships both handlers with state validation and PKCE; copy the file into a new project and replace the provider configuration.

### How do I store the state parameter so the callback can verify it?

Sign the state with a per-process secret and stash it in a short-lived cookie (`HttpOnly`, `Secure`, `SameSite=Lax`). On the callback, read the cookie and verify the signature. The cookie is preferred over a server-side store because the OAuth2 spec already requires the state parameter to be unguessable; signing makes forgery infeasible.

### Why does the example use PKCE even with a server-side flow?

PKCE protects against authorisation-code interception when the redirect URI passes through a less-trusted environment (browser extension, mobile app, untrusted proxy). The CPU cost is negligible and PKCE is mandatory in OAuth 2.1, so enabling it now keeps the example forward-compatible.

</section>
