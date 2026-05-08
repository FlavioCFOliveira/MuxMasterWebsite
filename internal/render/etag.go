package render

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strings"
)

// ETag computes the strong validator format defined in
// specification/rendering-and-caching.md: the first 16 chars of a base64-url
// SHA-256 over the body bytes, double-quoted.
func ETag(body []byte) string {
	sum := sha256.Sum256(body)
	enc := base64.RawURLEncoding.EncodeToString(sum[:])
	if len(enc) > 16 {
		enc = enc[:16]
	}
	return `"` + enc + `"`
}

// MatchesIfNoneMatch reports whether the request's If-None-Match header matches
// the given ETag. RFC 7232 allows a comma-separated list and the wildcard "*".
func MatchesIfNoneMatch(r *http.Request, etag string) bool {
	hdr := r.Header.Get("If-None-Match")
	if hdr == "" {
		return false
	}
	for _, candidate := range strings.Split(hdr, ",") {
		c := strings.TrimSpace(candidate)
		if c == "*" {
			return true
		}
		// Strong-strong comparison; we never emit weak ETags.
		if c == etag {
			return true
		}
	}
	return false
}
