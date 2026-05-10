# Response Helpers

MuxMaster provides a small set of functions that write complete HTTP responses in one call. They set the appropriate `Content-Type` header, call `WriteHeader`, and write the body.

## Functions

### JSON

```go
func JSON(w http.ResponseWriter, code int, v any) error
```

Marshals `v` to JSON, sets `Content-Type: application/json; charset=utf-8`, writes the status code, and writes the body.

```go
// Object
muxmaster.JSON(w, http.StatusOK, user)

// Slice
muxmaster.JSON(w, http.StatusOK, users)

// Inline map
muxmaster.JSON(w, http.StatusCreated, map[string]any{
    "id":   42,
    "name": "Alice",
})

// Error response
muxmaster.JSON(w, http.StatusNotFound, map[string]string{
    "error": "user not found",
})
```

Passing `0` as the code defaults to `200 OK`.

Returns the marshalling or write error; it does not write anything if marshalling fails.

---

### XML

```go
func XML(w http.ResponseWriter, code int, v any) error
```

Marshals `v` to XML, sets `Content-Type: application/xml; charset=utf-8`, and writes the response.

```go
type User struct {
    XMLName xml.Name `xml:"user"`
    ID      int      `xml:"id,attr"`
    Name    string   `xml:"name"`
}

muxmaster.XML(w, http.StatusOK, User{ID: 42, Name: "Alice"})
```

---

### Text

```go
func Text(w http.ResponseWriter, code int, s string) error
```

Writes `s` as plain text with `Content-Type: text/plain; charset=utf-8`.

```go
muxmaster.Text(w, http.StatusOK, "pong")
muxmaster.Text(w, http.StatusOK, fmt.Sprintf("hello, %s", name))
```

---

### Redirect

```go
func Redirect(w http.ResponseWriter, r *http.Request, code int, url string)
```

Issues an HTTP redirect. Delegates to `http.Redirect`.

```go
muxmaster.Redirect(w, r, http.StatusMovedPermanently, "/new-path")
muxmaster.Redirect(w, r, http.StatusFound, "https://example.com")
```

---

### NoContent

```go
func NoContent(w http.ResponseWriter)
```

Writes a `204 No Content` response with no body.

```go
mux.DELETE("/users/:id", func(w http.ResponseWriter, r *http.Request) {
    db.DeleteUser(muxmaster.PathParam(r, "id"))
    muxmaster.NoContent(w)
})
```

---

## Using Helpers with HandlerFuncE

The helpers return errors, which makes them natural to use in error-returning handlers:

```go
mux.GETE("/users/:id", func(w http.ResponseWriter, r *http.Request) error {
    id, err := muxmaster.ParamsFromContext(r.Context()).Int("id")
    if err != nil {
        return muxmaster.Error(http.StatusBadRequest, err)
    }
    user, err := db.FindUser(id)
    if err != nil {
        return muxmaster.Error(http.StatusNotFound, errors.New("not found"))
    }
    return muxmaster.JSON(w, http.StatusOK, user)
})
```

If `JSON` fails (e.g. the value cannot be marshalled), the error is returned to the `ErrorHandler`.

---

## Using Standard `encoding/json` Directly

For streaming or large responses where you want more control, use `encoding/json` directly:

```go
mux.GET("/users", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    if err := json.NewEncoder(w).Encode(users); err != nil {
        log.Printf("encode error: %v", err)
    }
})
```

The helpers are a convenience for the common case; they do not restrict your options.

---

## See Also

- [Error Handling](error-handling.md) — `HandlerFuncE` and `ErrorHandler`
- [Getting Started](getting-started.md#step-5----json-responses) — JSON responses in context

## Upstream source

The `JSON`, `Text`, `XML`, `Stream`, and conditional-GET helpers covered above are implemented in [`response.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/response.go) in the upstream repository.

## Common questions

<section data-conversation="response-patterns">

### How do I write a JSON response from a handler?

Call `mux.JSON(w, status, value)`. The helper sets `Content-Type: application/json; charset=utf-8`, encodes the value with `encoding/json`, writes the status, and returns any error from the encoder so callers can decide whether to log or surface it.

### How do I serve a conditional GET (304)?

Set the `ETag` (or `Last-Modified`) header before writing the body and call `mux.IfNoneMatch(w, r, etag)` (or `IfModifiedSince`). The helper returns `true` and writes the 304 short-circuit when the request's conditional headers match; the handler returns immediately without writing the body.

### How do I stream a response without buffering it in memory?

Use `mux.Stream(w, contentType, reader)`. The helper sets the content type, copies the reader to the response writer with a fixed-size buffer, and flushes between chunks where the underlying writer supports it. The reader is closed after the copy completes.

</section>
