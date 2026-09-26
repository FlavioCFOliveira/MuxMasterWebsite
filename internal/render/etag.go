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

// MatchesIfNoneMatch reports whether the request's If-None-Match header
// matches the given ETag. It allocates nothing.
func MatchesIfNoneMatch(r *http.Request, etag string) bool {
	return matchesIfNoneMatch(r.Header["If-None-Match"], etag)
}

// matchesIfNoneMatch applies the weak comparison that RFC 9110 § 13.1.2
// requires for If-None-Match: a "W/" prefix on a candidate is ignored, and
// "*" matches any current representation. Every header line may hold a
// comma-separated list, so each line is scanned in place instead of being
// split.
func matchesIfNoneMatch(values []string, etag string) bool {
	for _, v := range values {
		for v != "" {
			var candidate string
			if i := strings.IndexByte(v, ','); i >= 0 {
				candidate, v = v[:i], v[i+1:]
			} else {
				candidate, v = v, ""
			}
			candidate = strings.TrimSpace(candidate)
			if candidate == "*" {
				return true
			}
			candidate = strings.TrimPrefix(candidate, "W/")
			if candidate == etag {
				return true
			}
		}
	}
	return false
}

// AcceptsGzip reports whether the Accept-Encoding header lines allow a gzip
// response, following RFC 9110 § 12.5.3 and § 8.4.1.3:
//
//   - coding names are case-insensitive, and "x-gzip" is an alias of "gzip";
//   - a coding whose q-value is 0 is "not acceptable";
//   - when neither "gzip" nor "x-gzip" is listed, "*" with a non-zero
//     q-value accepts it.
//
// It allocates nothing: the lines are scanned in place.
func AcceptsGzip(values []string) bool {
	gzipListed, gzipOK := false, false
	starListed, starOK := false, false
	for _, v := range values {
		for v != "" {
			var item string
			if i := strings.IndexByte(v, ','); i >= 0 {
				item, v = v[:i], v[i+1:]
			} else {
				item, v = v, ""
			}
			name, params, _ := strings.Cut(item, ";")
			name = strings.TrimSpace(name)
			switch {
			case strings.EqualFold(name, "gzip"), strings.EqualFold(name, "x-gzip"):
				gzipListed = true
				if !qIsZero(params) {
					gzipOK = true
				}
			case name == "*":
				starListed = true
				starOK = !qIsZero(params)
			}
		}
	}
	if gzipListed {
		return gzipOK
	}
	return starListed && starOK
}

// qIsZero reports whether the parameter list of one Accept-Encoding item
// carries a q-value of zero ("q=0", "q=0.0", "q=0.00", "q=0.000"). A
// missing q-value means 1.
func qIsZero(params string) bool {
	for params != "" {
		var p string
		if i := strings.IndexByte(params, ';'); i >= 0 {
			p, params = params[:i], params[i+1:]
		} else {
			p, params = params, ""
		}
		key, val, ok := strings.Cut(strings.TrimSpace(p), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		val = strings.TrimSpace(val)
		if val == "" || val[0] != '0' {
			return false
		}
		for i := 1; i < len(val); i++ {
			if val[i] != '.' && val[i] != '0' {
				return false
			}
		}
		return true
	}
	return false
}
