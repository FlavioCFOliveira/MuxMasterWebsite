package render

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Response is an HTTP response computed once at startup and served,
// unchanged, for the lifetime of the process
// (specification/rendering-and-caching.md § "Pre-computed header values").
//
// It holds an identity representation and, when compression pays off, a gzip
// representation. Every header value is pre-built as a []string so Serve
// assigns it straight into the response header map: no formatting, no
// canonicalisation, no allocation per request.
//
// The header slices are shared by every request. Each has len == cap, so a
// later Header().Add on the same key copies instead of writing into the
// shared array; nothing on the serving path mutates them in place.
type Response struct {
	identity representation
	gzip     *representation // nil when no gzip representation is stored

	status       int
	contentType  []string
	cacheControl []string
	lastModified []string // nil when the response carries no Last-Modified
	vary         []string // nil when the response does not vary

	lastModifiedTime time.Time // truncated to the second, for If-Modified-Since
}

// representation is one encoding of the body with the header values that
// depend on its bytes.
type representation struct {
	body            []byte
	etag            string
	etagHdr         []string
	contentLength   []string
	contentEncoding []string // nil for identity
}

// ResponseOptions describes a response to pre-compute with NewResponse.
type ResponseOptions struct {
	Body         []byte
	ContentType  string
	CacheControl string
	Status       int
	// LastModified is emitted as Last-Modified and honoured through
	// If-Modified-Since. The zero value omits both.
	LastModified time.Time
	// Gzip requests a gzip representation. It is stored only when it is
	// smaller than the identity body; otherwise the response is identity
	// only and carries no Vary header.
	Gzip bool
	// GzipBody, when non-nil, is the already-compressed body to use instead
	// of compressing Body again (the prerender pass compresses each route
	// once).
	GzipBody []byte
	// Vary forces Vary: Accept-Encoding even when no gzip representation
	// is stored (pre-rendered routes always vary, per the spec).
	Vary bool
}

// NewResponse pre-computes a Response: the gzip body (compressed once, at the
// best compression level, because the cost is paid only at startup), one
// strong ETag per representation, and every header value.
func NewResponse(o ResponseOptions) (*Response, error) {
	if o.Status == 0 {
		o.Status = http.StatusOK
	}
	resp := &Response{
		identity:     newRepresentation(o.Body, ""),
		status:       o.Status,
		contentType:  []string{o.ContentType},
		cacheControl: []string{o.CacheControl},
	}
	if !o.LastModified.IsZero() {
		lm := o.LastModified.UTC().Truncate(time.Second)
		resp.lastModified = []string{lm.Format(http.TimeFormat)}
		resp.lastModifiedTime = lm
	}
	if o.Gzip {
		gz := o.GzipBody
		if gz == nil {
			var err error
			if gz, err = gzipBytes(o.Body); err != nil {
				return nil, err
			}
		}
		if len(gz) < len(o.Body) {
			rep := newRepresentation(gz, "gzip")
			resp.gzip = &rep
		}
	}
	if resp.gzip != nil || o.Vary {
		resp.vary = []string{"Accept-Encoding"}
	}
	return resp, nil
}

func newRepresentation(body []byte, encoding string) representation {
	etag := ETag(body)
	rep := representation{
		body:          body,
		etag:          etag,
		etagHdr:       []string{etag},
		contentLength: []string{strconv.Itoa(len(body))},
	}
	if encoding != "" {
		rep.contentEncoding = []string{encoding}
	}
	return rep
}

// gzipBytes compresses b with the standard library. The output is
// deterministic: the gzip header carries no name and a zero modification
// time, so the same input always yields the same bytes and the same ETag.
func gzipBytes(b []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("render: gzip: %w", err)
	}
	if _, err := zw.Write(b); err != nil {
		return nil, fmt.Errorf("render: gzip: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("render: gzip: %w", err)
	}
	return buf.Bytes(), nil
}

// Identity returns the identity body. Callers MUST NOT modify it.
func (p *Response) Identity() []byte { return p.identity.body }

// GzipBody returns the gzip body, or nil when none is stored. Callers MUST
// NOT modify it.
func (p *Response) GzipBody() []byte {
	if p.gzip == nil {
		return nil
	}
	return p.gzip.body
}

// ETags returns the identity ETag and the gzip ETag ("" when no gzip body is
// stored).
func (p *Response) ETags() (identity, gzipTag string) {
	if p.gzip != nil {
		gzipTag = p.gzip.etag
	}
	return p.identity.etag, gzipTag
}

// Serve writes the response. It selects the representation from
// Accept-Encoding, answers 304 when the client's validator is current, and
// otherwise writes the stored bytes. HEAD needs no special case: net/http
// discards the body of a HEAD response.
func (p *Response) Serve(w http.ResponseWriter, r *http.Request) {
	rep := &p.identity
	if p.gzip != nil && AcceptsGzip(r.Header["Accept-Encoding"]) {
		rep = p.gzip
	}

	h := w.Header()
	h["Etag"] = rep.etagHdr
	h["Cache-Control"] = p.cacheControl
	if p.lastModified != nil {
		h["Last-Modified"] = p.lastModified
	}
	// Vary is set on the 304 as well, so a cache keys the empty-body
	// response by the same dimension as the 200 it revalidates.
	if p.vary != nil {
		h["Vary"] = p.vary
	}

	if p.notModified(r, rep.etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	h["Content-Type"] = p.contentType
	h["Content-Length"] = rep.contentLength
	if rep.contentEncoding != nil {
		h["Content-Encoding"] = rep.contentEncoding
	}
	w.WriteHeader(p.status)
	_, _ = w.Write(rep.body)
}

// notModified evaluates the preconditions of RFC 9110 § 13.2.2 that apply to
// GET and HEAD: If-None-Match when present, otherwise If-Modified-Since.
func (p *Response) notModified(r *http.Request, etag string) bool {
	if inm := r.Header["If-None-Match"]; len(inm) > 0 {
		return matchesIfNoneMatch(inm, etag)
	}
	if p.lastModified == nil {
		return false
	}
	ims := r.Header["If-Modified-Since"]
	if len(ims) == 0 {
		return false
	}
	// A client normally echoes the exact Last-Modified value it received:
	// compare the strings first and skip date parsing.
	if ims[0] == p.lastModified[0] {
		return true
	}
	t, err := http.ParseTime(ims[0])
	return err == nil && !p.lastModifiedTime.After(t)
}
