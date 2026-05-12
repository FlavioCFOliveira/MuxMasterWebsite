---
datePublished: 2026-05-12
---

# Server-sent events example

A working **Server-Sent Events (SSE)** endpoint on MuxMaster with the opt-in `PoolRequestBundle` enabled. The example pairs a streaming `/events/:topic` route with a synchronous `/publish` endpoint and a periodic server-side `tick` so a subscriber sees live data even with no producer attached.

## Why this example is pool-safe

`PoolRequestBundle` recycles the per-request bundle the instant the handler returns. SSE handlers **do not return** until the client disconnects or the server shuts down — they sit inside their `for` loop streaming data. The request stays alive for the entire stream, so the pooled bundle is returned only **after** the SSE session ends. The lifetime contract holds; pooling is safe.

This is the inverse of WebSocket / `Hijack()` upgrades, which transfer ownership of the connection past `ServeHTTP` return — pooling is unsafe there. SSE is plain HTTP over `net/http` and respects the contract trivially.

## Step 1 — Construct the router

```go
hub := newHub()
log := slog.New(slog.NewTextHandler(os.Stdout, nil))

mux := mm.New()
mux.PoolRequestBundle = true
mux.Pre(mw.RequestID(), mw.RecovererWithLogger(log))

mux.GET("/events/:topic", hub.stream)
mux.POST("/publish", hub.publish)
mux.GET("/", indexHTML)

go hub.tick() // emit a server-side "tick" every 5 s on the "system" topic
```

`Pre` runs once per request, before routing — `RequestID` and `RecovererWithLogger` apply to both the streaming route and the synchronous publisher.

## Step 2 — The `hub` — a tiny topic-to-subscribers fan-out

The hub holds one `map[string]map[chan string]struct{}` (topic → set of subscriber channels) behind a `sync.RWMutex`. Reads (broadcasts) take the read lock; writes (subscribe / unsubscribe) take the write lock. A slow subscriber is **dropped** rather than blocking the broadcast — backpressure that would otherwise stall every subscriber on a topic.

```go
func (h *hub) broadcast(topic, msg string) int {
    h.mu.RLock()
    subs := h.topics[topic]
    delivered := 0
    for ch := range subs {
        select {
        case ch <- msg:
            delivered++
        default: // slow subscriber — drop the message rather than block
        }
    }
    h.mu.RUnlock()
    return delivered
}
```

`subscribe` returns a buffered channel (`make(chan string, 16)`); `unsubscribe` removes the entry and closes the channel.

## Step 3 — The `stream` handler

The SSE handler writes the canonical event-stream headers, performs the initial flush to send the response head immediately, subscribes, and then loops over `r.Context().Done()` and the subscriber channel. **`r.Context()` is safe to use under `PoolRequestBundle`** because the handler has not returned yet — the bundle is still live.

```go
func (h *hub) stream(w http.ResponseWriter, r *http.Request) {
    topic := mm.PathParam(r, "topic")
    if topic == "" {
        http.Error(w, "missing topic", http.StatusBadRequest)
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no") // for nginx proxies
    w.WriteHeader(http.StatusOK)

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }

    ch := h.subscribe(topic)
    defer h.unsubscribe(topic, ch)

    fmt.Fprintf(w, "event: ready\ndata: subscribed to %q\n\n", topic)
    flusher.Flush()

    for {
        select {
        case <-r.Context().Done():
            return
        case msg, ok := <-ch:
            if !ok {
                return
            }
            _, _ = fmt.Fprintf(w, "data: %s\n\n", msg)
            flusher.Flush()
        }
    }
}
```

Notes on the wire format: SSE separates events with a blank line. `data:` is the message body; `event:` names a custom event the client subscribes to via `addEventListener` (the example uses `event: ready` for the initial subscription confirmation). `X-Accel-Buffering: no` disables nginx's response buffering for the stream — without it, an nginx-fronted SSE endpoint will appear to hang for minutes.

## Step 4 — The `publish` handler

Synchronous POST endpoint that drains the body, parses the JSON, broadcasts to the named topic, and returns a delivery count. Body is drained **inline** with `io.ReadAll(io.LimitReader(r.Body, 1<<16))` — nothing escapes the handler, so the pooled bundle recycles cleanly.

```go
func (h *hub) publish(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
    if err != nil {
        http.Error(w, "bad body", http.StatusBadRequest)
        return
    }
    var p struct {
        Topic string `json:"topic"`
        Msg   string `json:"msg"`
    }
    if err := json.Unmarshal(body, &p); err != nil {
        http.Error(w, "invalid json", http.StatusBadRequest)
        return
    }
    if p.Topic == "" || p.Msg == "" {
        http.Error(w, "topic and msg required", http.StatusUnprocessableEntity)
        return
    }
    n := h.broadcast(p.Topic, p.Msg)
    _ = json.NewEncoder(w).Encode(map[string]any{
        "delivered": n,
        "topic":     p.Topic,
    })
}
```

The 64 KB `LimitReader` cap prevents a malicious publisher from exhausting server memory with an unbounded body. The 422 status code on empty `topic` / `msg` follows RFC 9110 §15.5.21 (Unprocessable Content) — the JSON parsed but did not validate.

## Step 5 — The periodic `tick`

A goroutine launched at boot broadcasts a "tick" message every 5 s on the `system` topic. It is the canonical pattern for a server-driven heartbeat — a subscriber sees activity even when no client is publishing.

```go
func (h *hub) tick() {
    t := time.NewTicker(5 * time.Second)
    defer t.Stop()
    for {
        select {
        case <-h.done:
            return
        case now := <-t.C:
            h.broadcast("system", fmt.Sprintf("tick at %s", now.Format(time.Kitchen)))
        }
    }
}
```

## Try it

```bash
# Terminal 1: start the server
go run .

# Terminal 2: subscribe to the news topic
curl -N http://localhost:8080/events/news

# Terminal 3: publish to the news topic
curl -X POST http://localhost:8080/publish \
     -d '{"topic":"news","msg":"hello"}'

# Or open http://localhost:8080/ in a browser — the page subscribes to /events/system.
```

## Frequently asked questions

<section data-conversation="server-sent-events-faq">

### Why is SSE safe with `PoolRequestBundle` when the connection outlives the handler?

The connection outlives the *call*, but the handler does **not return** until the connection closes — it sits inside the `for` loop streaming events. Under `PoolRequestBundle`, the bundle is recycled when the handler returns; for an SSE handler, that is when the stream ends. So the pool reclaim happens **after** the connection terminates, never during the stream. The contract holds.

### How do I bound memory if a subscriber stops reading?

The hub's `broadcast` uses `select` with a `default` branch (Step 2). When the subscriber's buffered channel (`make(chan string, 16)`) is full, the message is **dropped** rather than block the broadcast goroutine. A subscriber that does not read for 16 events gets dropped messages but never causes head-of-line blocking for other subscribers. For stronger guarantees, swap the `default` branch for `time.After` plus an unsubscribe action.

### How do I authenticate the publisher?

Wrap `/publish` with an auth middleware in a `Group`: `pub := mux.Group("/publish"); pub.Use(mw.JWTAuth(cfg)); pub.POST("", hub.publish)`. Or, for a server-to-server publisher, use `mw.APIKey` with a SHA-256-hashed key lookup. Both compose at registration time — zero per-request overhead on the publish path. The stream endpoint stays unauthenticated if it should be world-readable, or guarded with the same middleware family if subscriptions are per-tenant.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/server-sent-events/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/examples/server-sent-events/main.go) at the v1.1.0 tag. The upstream file also contains the full `hub` type with `subscribe`, `unsubscribe`, and `close`, plus the in-browser demo HTML — follow the link for the full program.

## See also

- [Maximum performance](/docs/max-performance#recipe-3--streaming-response-no-opt-in-pool-needed) — Recipe 3 explains why long-lived streams are pool-safe.
- [Reverse-proxy example](/examples/reverse-proxy) — another pool-safe pattern (the proxy returns synchronously before `ServeHTTP` exits).
- [Upload file example](/examples/upload-file) — the body-drain-before-spawn pattern for background work.
