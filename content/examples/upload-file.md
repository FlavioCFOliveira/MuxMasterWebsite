---
datePublished: 2026-05-12
---

# Upload-file example

Multipart file upload with **`PoolRequestBundle` enabled** and the **critical body-drain-before-spawn pattern** that makes background processing safe. Three handlers show the spectrum: single-file synchronous upload, multi-file synchronous upload, and asynchronous processing where the goroutine must not retain `*http.Request`.

## Why this example is pool-safe

`PoolRequestBundle` recycles the per-request bundle the instant the handler returns. A goroutine that reads `r.Body` (or `r.MultipartReader()`) **after** that return observes either a zeroed bundle or another concurrent request's state — a silent use-after-free. The fix is mechanical and reproducible:

**Drain everything you need into local values before spawning the goroutine. Never capture `r` itself.**

```go
// Wrong — r is recycled the moment the outer handler returns
go func() {
    io.Copy(dst, r.Body)
}()

// Right — drain inline, then spawn with captured values
body, _ := io.ReadAll(r.Body)
go func() {
    process(body)
}()
```

This example shows the safe and unsafe patterns side by side so the contract is unambiguous.

## Step 1 — Construct the router

Note the `ReadTimeout: 60 * time.Second` on the `http.Server` — file uploads run longer than the default 5 s read timeout, so the value is bumped to a sensible upload-friendly bound.

```go
mux := mm.New()
// Pool-safe ONLY if every upload handler drains the body before return.
// The three handlers below are correctly written for this contract.
mux.PoolRequestBundle = true

mux.Pre(mw.RequestID(), mw.RecovererWithLogger(log))

mux.POST("/upload", singleUpload)        // 1 file, sync
mux.POST("/multi", multiUpload)          // N files, sync
mux.POST("/async", asyncProcessUpload)   // drain → spawn goroutine

srv := &http.Server{
    Addr:        ":8080",
    Handler:     mux,
    ReadTimeout: 60 * time.Second, // upload-friendly
}
```

A `maxBody = 32 << 20` constant (32 MiB) is applied to every handler via `http.MaxBytesReader` so a hostile uploader cannot exhaust process memory.

## Step 2 — Single-file synchronous upload

The simplest pattern: cap the body, parse the multipart form, copy the file to disk, return the SHA-256 + byte count. Everything is materialised before the handler returns — bundle recycling happens trivially.

```go
func singleUpload(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, maxBody)
    if err := r.ParseMultipartForm(2 << 20); err != nil {
        http.Error(w, "parse error: "+err.Error(), http.StatusBadRequest)
        return
    }

    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "missing 'file' field", http.StatusBadRequest)
        return
    }
    defer file.Close()

    sum, written, err := saveFile(file, header.Filename)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    _ = json.NewEncoder(w).Encode(map[string]any{
        "filename": header.Filename,
        "size":     written,
        "sha256":   sum,
    })
}
```

`ParseMultipartForm(2 << 20)` is the **in-memory cap** for form values — anything beyond 2 MiB is spilled to disk. `MaxBytesReader` is the **wire-level cap** that protects against the whole request body. The two caps are independent and both should be set.

## Step 3 — Multi-file upload in one POST

The `multipart/form-data` spec allows multiple files in one form; `r.MultipartForm.File` is a `map[string][]*multipart.FileHeader`. Loop over both axes and apply the same `saveFile` helper.

```go
func multiUpload(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, maxBody)
    if err := r.ParseMultipartForm(2 << 20); err != nil {
        http.Error(w, "parse error: "+err.Error(), http.StatusBadRequest)
        return
    }

    type result struct {
        Filename string `json:"filename"`
        Size     int64  `json:"size"`
        SHA256   string `json:"sha256"`
    }
    var results []result

    for _, headers := range r.MultipartForm.File {
        for _, h := range headers {
            f, err := h.Open()
            if err != nil { /* … */ return }
            sum, n, err := saveFile(f, h.Filename)
            _ = f.Close()
            if err != nil { /* … */ return }
            results = append(results, result{Filename: h.Filename, Size: n, SHA256: sum})
        }
    }
    _ = json.NewEncoder(w).Encode(map[string]any{"files": results})
}
```

## Step 4 — Async processing: the body-drain-before-spawn pattern

The canonical safe pattern under `PoolRequestBundle`. Every value the goroutine needs is **snapshotted into local variables** before the `go` statement. The goroutine has no reference to `r`, so the bundle recycles cleanly.

```go
func asyncProcessUpload(w http.ResponseWriter, r *http.Request) {
    r.Body = http.MaxBytesReader(w, r.Body, maxBody)
    if err := r.ParseMultipartForm(2 << 20); err != nil {
        http.Error(w, "parse error: "+err.Error(), http.StatusBadRequest)
        return
    }
    file, header, err := r.FormFile("file")
    if err != nil {
        http.Error(w, "missing 'file' field", http.StatusBadRequest)
        return
    }
    defer file.Close()

    // Snapshot EVERYTHING the goroutine needs into local values.
    // `data`, `filename`, `requestID` are captured by value; `r` is not.
    var buf bytes.Buffer
    if _, err := io.Copy(&buf, file); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    data := buf.Bytes()
    filename := header.Filename
    requestID := mw.GetRequestID(r.Context())

    // Now the goroutine has no reference to r — safe under Pool.
    go func() {
        // Simulate background processing — e.g. virus scan, thumbnailing.
        time.Sleep(500 * time.Millisecond)
        sum := sha256.Sum256(data)
        fmt.Fprintf(os.Stderr,
            "[bg] processed req=%s file=%q size=%d sha256=%x\n",
            requestID, filename, len(data), sum)
    }()

    w.WriteHeader(http.StatusAccepted)
    _ = json.NewEncoder(w).Encode(map[string]any{
        "accepted":   true,
        "filename":   filename,
        "size_bytes": len(data),
        "request_id": requestID,
    })
}
```

Three values cross the goroutine boundary: `data` (a `[]byte` copy owned by `buf` — independent of `r.Body`), `filename` (a string, immutable), and `requestID` (a string extracted via `mw.GetRequestID(r.Context())`). The handler responds 202 Accepted with the request ID so the client can correlate logs.

## Step 5 — `saveFile` with path-traversal guard

The shared helper hashes the body while writing it to disk. The filename is normalised with `filepath.Base` and `..` / `.` are rejected — without that guard, a malicious client could write into arbitrary paths via `../../../etc/passwd`.

```go
func saveFile(src multipart.File, filename string) (string, int64, error) {
    if filename == "" {
        return "", 0, fmt.Errorf("empty filename")
    }
    // Reject path traversal.
    safe := filepath.Base(filename)
    if safe == "." || safe == ".." {
        return "", 0, fmt.Errorf("invalid filename")
    }

    dst, err := os.Create(filepath.Join(uploadDir, safe))
    if err != nil {
        return "", 0, fmt.Errorf("create: %w", err)
    }
    defer dst.Close()

    hasher := sha256.New()
    written, err := io.Copy(io.MultiWriter(dst, hasher), src)
    if err != nil {
        return "", 0, fmt.Errorf("copy: %w", err)
    }
    return hex.EncodeToString(hasher.Sum(nil)), written, nil
}
```

`io.MultiWriter(dst, hasher)` runs the file through both the disk writer and the SHA-256 hasher in a single streaming pass — no second read of the body is needed.

## Try it

```bash
mkdir -p /tmp/muxmaster-uploads
go run .

# Single-file
curl -F 'file=@/path/to/some/file' http://localhost:8080/upload

# Multi-file
curl -F 'file=@/path/to/some/file' \
     -F 'file=@/path/to/another/file' \
     http://localhost:8080/multi

# Async processing — returns 202 Accepted, processing logs to stderr
curl -F 'file=@/path/to/some/file' http://localhost:8080/async
```

## Frequently asked questions

<section data-conversation="upload-file-faq">

### Why must I drain the body before `go`?

`PoolRequestBundle` recycles `*http.Request` when the handler returns. A goroutine that holds `r.Body` past return reads from a recycled `io.ReadCloser` — either zeroed, or attached to another concurrent request. Draining the body into a `[]byte` **inside** the handler captures the bytes by value; the goroutine then operates on a copy that is independent of the bundle. The `asyncProcessUpload` handler in Step 4 is the canonical safe pattern.

### What does `MaxBytesReader` actually cap?

`http.MaxBytesReader(w, r.Body, n)` wraps `r.Body` so any read past `n` bytes returns an error and writes a `413 Payload Too Large` to `w`. The cap is **wire-level**: it counts bytes off the connection before they are buffered. It is independent of `ParseMultipartForm(memCap)`, which caps how much of the parsed multipart form is held in memory (the rest spills to disk). Set both.

### How do I checksum without buffering the whole upload?

Use `io.MultiWriter`. The `saveFile` helper in Step 5 wires `io.Copy(io.MultiWriter(dst, hasher), src)` — every byte read from the upload is written to disk and to the SHA-256 hasher in a single streaming pass. Peak memory is one read buffer (32 KiB by default), not the full file size. Replace `sha256.New()` with `xxhash.New64()` or any other `hash.Hash` if you want a faster non-cryptographic checksum.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/upload-file/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.1.0/examples/upload-file/main.go) at the v1.1.0 tag. The upstream file also contains the in-browser upload form HTML and the graceful-shutdown wiring — follow the link for the full program.

## See also

- [Maximum performance](/docs/max-performance#lifetime-contract--what-you-must-not-do) — the lifetime contract this example respects, with the full enumeration of unsafe captures.
- [Server-sent events example](/examples/server-sent-events) — another long-lived handler pattern (stream until disconnect) that satisfies the pool contract.
- [Reverse-proxy example](/examples/reverse-proxy) — synchronous proxying as a third pool-safe shape.
