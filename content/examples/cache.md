---
datePublished: 2026-05-08
---

# Cache example

Three complementary response-caching techniques on the same router: an in-memory TTL cache that avoids re-computing expensive responses within a configurable window, `Cache-Control` headers that instruct clients and intermediaries to keep their own copy, and ETag-driven conditional GETs that let unchanged resources skip the body entirely with a 304. Reach for this example when handler work is expensive and idempotent within a short horizon.

## Step 1 — Define the in-memory TTL cache

A small concurrent map with per-entry expiry and a background eviction goroutine. The standard library covers everything; no third-party cache package is required for the simple use case the example demonstrates.

```go
type entry struct {
    val       any
    expiresAt time.Time
}

type Cache struct {
    mu   sync.RWMutex
    data map[string]entry
}

func newCache() *Cache {
    c := &Cache{data: make(map[string]entry)}
    go c.evict()
    return c
}
```

The eviction goroutine walks the map every minute and deletes expired entries; without it, keys whose TTL passed but were never read again would stay in memory until the process exits.

## Step 2 — Implement Get / Set / Invalidate

`Set` writes a value with its expiry; `Get` returns the value only if it exists and has not expired (callers never observe stale data); `Invalidate` is prefix-based so a single mutation can clear every related key (`articles:` clears the list cache, the item cache for every id, and any future per-user variant).

```go
func (c *Cache) Set(key string, val any, ttl time.Duration) {
    c.mu.Lock()
    c.data[key] = entry{val: val, expiresAt: time.Now().Add(ttl)}
    c.mu.Unlock()
}

func (c *Cache) Get(key string) (any, bool) {
    c.mu.RLock()
    e, ok := c.data[key]
    c.mu.RUnlock()
    if !ok || time.Now().After(e.expiresAt) {
        return nil, false
    }
    return e.val, true
}

func (c *Cache) Invalidate(prefix string) {
    c.mu.Lock()
    for k := range c.data {
        if strings.HasPrefix(k, prefix) {
            delete(c.data, k)
        }
    }
    c.mu.Unlock()
}
```

The expired-on-read check inside `Get` matters: it guarantees a value retrieved between eviction passes is still fresh by the cache's own contract, even if the eviction goroutine is sleeping.

## Step 3 — Build the ETag helpers

A weak ETag (the `W/` prefix) declares that two ETag values represent semantically equivalent payloads but may differ byte-for-byte. JSON responses are appropriate weak-ETag candidates: serialisation order is implementation-defined, but two responses with the same logical content match for caching purposes.

```go
func etagFor(parts ...string) string {
    return fmt.Sprintf(`W/%q`, strings.Join(parts, ":"))
}

func checkNotModified(w http.ResponseWriter, r *http.Request, etag string) bool {
    if r.Header.Get("If-None-Match") == etag {
        w.Header().Set("ETag", etag)
        w.WriteHeader(http.StatusNotModified)
        return true
    }
    return false
}
```

`checkNotModified` writes the 304 short-circuit when the client's `If-None-Match` matches the current ETag and returns `true` so the handler can return immediately without writing a body.

## Step 4 — Construct the router with global middleware and JSON error handlers

Same shape as the other examples — `RequestID` + `Logger` + `Recoverer` on every request, JSON-shaped `NotFound` and `ErrorHandler` so error responses match the API's content type.

```go
r := mm.New()

r.Use(
    mw.RequestID(),
    mw.Logger(os.Stdout),
    mw.RecovererWithLogger(log),
)

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

The error handler maps `mm.HTTPError` to the right status code via `errors.As` — this is the canonical Go pattern for unwrapping a typed error past any number of `fmt.Errorf("%w", ...)` wrappings.

## Step 5 — Cache the article list with TTL + `Cache-Control`

The list endpoint is the canonical "expensive read, cheap write" target. The first request within each 30-second window populates the cache and emits `X-Cache: MISS`; subsequent requests hit the cache and emit `X-Cache: HIT`. The `Cache-Control: public, max-age=30` header lets reverse proxies and clients keep their own copies for the same window.

```go
r.GET("/articles", func(w http.ResponseWriter, r *http.Request) {
    const (
        ttl = 30 * time.Second
        key = "articles:list"
    )
    w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(ttl.Seconds())))

    if cached, ok := cache.Get(key); ok {
        w.Header().Set("X-Cache", "HIT")
        _ = mm.JSON(w, http.StatusOK, cached)
        return
    }

    list := store.list()
    cache.Set(key, list, ttl)
    w.Header().Set("X-Cache", "MISS")
    _ = mm.JSON(w, http.StatusOK, list)
})
```

The `X-Cache` header is non-standard but widely used; it is invaluable for observability — `curl -I` reveals at a glance whether a given request hit the cache or fell through to the store.

## Step 6 — Invalidate the cache on writes

A successful `POST /articles` creates a new article, which means the cached list is stale and the in-flight `Cache-Control: max-age=30` headers issued before the write are now lying. The handler calls `cache.Invalidate("articles:")` so the very next list request misses and rebuilds.

```go
r.POSTE("/articles", func(w http.ResponseWriter, r *http.Request) error {
    var body struct {
        Title string `json:"title"`
    }
    if err := decodeJSON(r, &body); err != nil {
        return mm.Error(http.StatusBadRequest, errors.New("invalid JSON"))
    }
    if strings.TrimSpace(body.Title) == "" {
        return mm.Error(http.StatusUnprocessableEntity, errors.New("title is required"))
    }

    article := store.create(body.Title)

    // A new article was added — the cached list is stale.
    cache.Invalidate("articles:")

    w.Header().Set("Location", fmt.Sprintf("/articles/%d", article.ID))
    return mm.JSON(w, http.StatusCreated, article)
})
```

Note that the previously-issued client and proxy caches retain their (now stale) copy until their `max-age` elapses — that is the trade-off TTL-based caching makes. Where freshness is non-negotiable, ETag-driven conditional GETs (Step 7) are a stronger guarantee.

## Step 7 — Use ETag + 304 on the single-item endpoint

The single-item endpoint computes a weak ETag from the article's mutable state (`id:title:done`) and returns 304 when the client's `If-None-Match` header matches. The `Cache-Control: public, max-age=60` header still gives the client a one-minute fresh window during which it does not need to revalidate at all.

```go
r.GETE("/articles/:id", func(w http.ResponseWriter, r *http.Request) error {
    id := mm.PathParam(r, "id")
    article, ok := store.get(id)
    if !ok {
        return mm.Error(http.StatusNotFound, fmt.Errorf("article %q not found", id))
    }

    etag := etagFor(fmt.Sprint(article.ID), article.Title, fmt.Sprint(article.Done))
    w.Header().Set("Cache-Control", "public, max-age=60")

    if checkNotModified(w, r, etag) {
        return nil // 304 already written — return early
    }

    w.Header().Set("ETag", etag)
    return mm.JSON(w, http.StatusOK, article)
})
```

When the article changes, the ETag changes — clients that retry with the old ETag fall through `checkNotModified` and receive a fresh 200 with the new body.

## Step 8 — Invalidate everything on a mutation that changes the ETag

Toggling the `done` flag changes the ETag, which means previously-cached client copies are no longer valid. The handler invalidates both the list cache (the article's `done` value appears in the listed payload) and effectively the per-item cache (the next GET regenerates the ETag).

```go
r.PUTE("/articles/:id/done", func(w http.ResponseWriter, r *http.Request) error {
    id := mm.PathParam(r, "id")
    article, ok := store.toggleDone(id)
    if !ok {
        return mm.Error(http.StatusNotFound, fmt.Errorf("article %q not found", id))
    }

    // Both the list and the item are now stale.
    cache.Invalidate("articles:")

    return mm.JSON(w, http.StatusOK, article)
})
```

The combination of TTL + ETag is what gives the cache its strong consistency guarantee: a client that revalidates with the old ETag receives 200 with the updated body, never a stale 304.

## Step 9 — Serve with hardened timeouts

The same timeout set as the other examples — closes the slowloris vector. Production deployments should also wrap the server start in a goroutine and drain on `SIGINT`/`SIGTERM` (see the graceful-shutdown example for the full pattern).

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

The example uses `srv.ListenAndServe()` directly to keep the file focused on caching; in production, the goroutine + `Shutdown(ctx)` shape applies here too.

## Common questions

<section data-conversation="cache-patterns">

### How do I cache an expensive handler response in memory?

Wrap the handler with a small `Cache` (the example provides one) keyed on a stable identifier (the URL or a tuple of URL + relevant headers). Read from the cache first; on miss, run the underlying logic, store the result with a TTL, and return it. The example demonstrates this pattern on the `/articles` list with a 30-second window.

### How do I invalidate a cached entry when the underlying data changes?

Either let the entry expire on its TTL or call `cache.Invalidate(prefix)` immediately after the mutation that made it stale. The example invalidates the `articles:` prefix after every `POST /articles` and every `PUT /articles/:id/done` so the next read regenerates the entry.

### How does the ETag round-trip work alongside the in-memory cache?

The single-item handler computes a weak ETag from the article's mutable fields (`id:title:done`) and returns 304 when the client's `If-None-Match` matches. The combination of TTL caching, `Cache-Control` headers, and ETag-driven conditional GETs is intentional: TTL makes repeat reads cheap, `Cache-Control` lets intermediaries help, and ETag guarantees clients never observe a stale body silently.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/cache/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/cache/main.go) at the v1.0.1 tag. The upstream file also includes the in-process article store, the `decodeJSON` body-limit helper, and the smoke-test commands in the package comment — follow the link for the full program.
