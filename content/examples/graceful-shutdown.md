# Graceful shutdown example

The production-recommended pattern for serving a MuxMaster router behind `http.Server`: signal-driven shutdown, in-flight request drain, the full set of slowloris-defeating timeouts, and a cooperative handler that observes `ctx.Done()` so the timeout middleware can preempt long-running work (`SECURITY.md` MM-2026-0019).

## Step 1 — Pin the timeout and grace-period constants

The four `http.Server` timeouts and the shutdown deadline are best held as named constants so the deployment can tune them in one place. The values below close the slowloris vector: a malicious client cannot dribble headers or stall a write to hold a goroutine indefinitely.

```go
const (
    listenAddr       = ":8080"
    gracefulTimeout  = 30 * time.Second
    readHeader       = 10 * time.Second
    readTimeout      = 30 * time.Second
    writeTimeout     = 30 * time.Second
    idleTimeout      = 90 * time.Second
    maxHeaderBytes   = 1 << 20 // 1 MiB
    cooperativeSleep = 3 * time.Second
)
```

`gracefulTimeout` is the upper bound on the drain phase — after it elapses the remaining connections are closed mid-flight. The other timeouts are passed straight to the `http.Server`.

## Step 2 — Construct the router with Pre and Use middleware

`Recoverer` and `RequestID` belong in `Pre` because they MUST run for every request before route resolution (a panic in the lookup path itself, an external request id propagated from a load balancer). `Timeout` and `Logger` run in `Use` after the handler is selected, so they observe the actual matched route.

```go
r := mm.New()

// Pre middleware wraps BOTH stdlib and FastHandler routes.
r.Pre(mw.RecovererWithLogger(logger))
r.Pre(mw.RequestID())

// Per-request deadline. Handlers MUST observe ctx.Done() to be preempted.
r.Use(mw.Timeout(5 * time.Second))
r.Use(mw.Logger(os.Stdout))
```

The `Timeout(5 * time.Second)` is the per-request budget — independent of `gracefulTimeout`, which bounds shutdown rather than individual requests.

## Step 3 — Register the cooperative slow handler

A handler that ignores `ctx.Done()` cannot be preempted by the `Timeout` middleware nor by a graceful shutdown — its goroutine continues past the deadline and the process exits dirty. The `slowHandler` below shows the canonical cooperative shape: a `select` between the work and the cancellation channel.

```go
r.GET("/slow", slowHandler)

func slowHandler(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    select {
    case <-time.After(cooperativeSleep):
        fmt.Fprintln(w, "slow work done")
    case <-ctx.Done():
        // ctx.Err() is context.Canceled (client gone or shutdown)
        // or context.DeadlineExceeded (Timeout middleware fired).
        http.Error(w, "request cancelled: "+ctx.Err().Error(), http.StatusGatewayTimeout)
    }
}
```

The `ctx.Err()` discriminator distinguishes the two cancellation reasons — useful in metrics and log lines because client-gone and timeout-fired have different operational meanings.

## Step 4 — Build the `http.Server` with the timeout set wired in

`http.Server`'s zero values do not protect against slowloris. Setting `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` to the constants from Step 1 closes every stall vector.

```go
srv := &http.Server{
    Addr:              listenAddr,
    Handler:           r,
    ReadHeaderTimeout: readHeader,
    ReadTimeout:       readTimeout,
    WriteTimeout:      writeTimeout,
    IdleTimeout:       idleTimeout,
    MaxHeaderBytes:    maxHeaderBytes,
    ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
}
```

`ErrorLog` routes the server's internal errors (e.g. "TLS handshake error from <ip>") through the same structured logger as the handlers, so operators see one stream rather than two.

## Step 5 — Start the server in a goroutine and surface its terminal error

`ListenAndServe` blocks; the program needs to wait for a signal at the same time. The canonical Go pattern is to run the server in a goroutine and report its error (if any) over a buffered channel of size 1.

```go
serverErr := make(chan error, 1)
go func() {
    logger.Info("server starting", "addr", listenAddr)
    if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        serverErr <- err
        return
    }
    serverErr <- nil
}()
```

`http.ErrServerClosed` is the signal that `Shutdown` (Step 7) closed the listener — a clean exit, not an error. Filtering it out keeps the unhappy-path log free of noise.

## Step 6 — Wait for `SIGINT`, `SIGTERM`, or a server failure

`signal.Notify` registers the channel with the OS so the runtime delivers the named signals as values on the channel rather than killing the process. The `select` waits on either a signal or an early server failure.

```go
stop := make(chan os.Signal, 1)
signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

select {
case err := <-serverErr:
    if err != nil {
        logger.Error("server failed before shutdown", "err", err)
        os.Exit(1)
    }
    return
case sig := <-stop:
    logger.Info("shutdown signal received — draining in-flight requests",
        "signal", sig.String(), "deadline", gracefulTimeout)
}
```

If the server fails before any signal arrives, the program exits 1 with a structured error log. Container orchestrators (systemd, Kubernetes) read the exit code to decide whether to restart.

## Step 7 — Drain in-flight requests with a bounded deadline

`Shutdown(ctx)` stops the listener, lets in-flight handlers finish, and returns when the connections are all closed — or when `ctx` expires, whichever happens first. The `context.WithTimeout` derives a fresh context that bounds the drain phase by `gracefulTimeout`.

```go
ctx, cancel := context.WithTimeout(context.Background(), gracefulTimeout)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    // Shutdown(ctx) returns the ctx error if the deadline expires before
    // every connection drained — surface it as a non-zero exit so an
    // orchestrator (systemd, k8s) can record the unclean shutdown.
    logger.Error("graceful shutdown timed out — connections were closed mid-flight",
        "err", err)
    os.Exit(1)
}

logger.Info("clean shutdown complete")
```

A non-zero exit on timeout matters: the process closed connections that were still mid-write, and a downstream orchestrator may want to alert on that distinct from a clean shutdown.

## Common questions

<section data-conversation="graceful-shutdown-patterns">

### How do I shut down a MuxMaster server without dropping in-flight requests?

Call `srv.Shutdown(ctx)` on the `*http.Server` (not the router itself) — `Shutdown` stops accepting new connections, waits for in-flight handlers to complete, and returns when the listener and the active connections have all closed. The example program does this in response to `SIGINT`/`SIGTERM`.

### How do I bound the grace period?

Construct the context passed to `Shutdown` with a deadline (`context.WithTimeout(ctx, 30*time.Second)`). When the deadline elapses, `Shutdown` returns `context.DeadlineExceeded` and the in-flight requests are dropped — a forced exit after a bounded wait, never an infinite hang.

### How do I make a long-running handler honour the shutdown signal?

Read `req.Context()` and `select` on `ctx.Done()` alongside the work. The example's `slowHandler` shows the pattern: `case <-time.After(...)` for the success path, `case <-ctx.Done()` for the cancellation path. A handler that ignores the context cannot be preempted by the timeout middleware OR by graceful shutdown.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/graceful-shutdown/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/graceful-shutdown/main.go) at the v1.0.1 tag. Follow that link for the complete file, including the package comment, imports, and the run instructions.
