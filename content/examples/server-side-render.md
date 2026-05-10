# Server Side Render example

A multi-page guestbook rendered by Go's `html/template`, demonstrating layout inheritance, the POST–Redirect–GET pattern with flash messages, form re-population on validation failure, embedded static assets via `embed.FS`, and a themed 404 page. Reach for this example when serving HTML pages directly from MuxMaster — the same pattern powers this documentation website.

## Step 1 — Embed templates and static files into the binary

`embed.FS` reads the `templates/` and `static/` directories at compile time and bakes them into the binary. The deployment artefact is a single executable; there is no filesystem dependency at runtime.

```go
//go:embed templates static
var files embed.FS
```

This is the same primitive the documentation website uses to embed `/content/` — see `embed.go` at the repository root.

## Step 2 — Pre-parse one `*template.Template` per page

Every page is parsed at startup as `base.html` + `<page>.html` together, so each template set has exactly one definition per `{{block "content"}}` and `{{block "title"}}`. There are no runtime conflicts between pages and no per-request parsing cost.

```go
type Templates struct {
    pages    map[string]*template.Template
    notFound *template.Template
}

func loadTemplates() *Templates {
    base := "templates/base.html"
    pages := make(map[string]*template.Template)
    for _, name := range []string{"home", "about", "guestbook"} {
        pages[name] = template.Must(template.ParseFS(
            files, base, "templates/"+name+".html",
        ))
    }
    return &Templates{
        pages:    pages,
        notFound: template.Must(template.ParseFS(files, "templates/404.html")),
    }
}
```

`template.Must` panics at startup if any template fails to parse; the binary refuses to come up with broken assets, which is the right failure mode for static content.

## Step 3 — Define `render` and `renderNotFound`

The render helper looks up the page, sets the content type, and executes the `base` template (which inherits from `<page>.html` via the block definitions). When `Execute` fails after headers have been sent, the only honest action is to log — the response is already partially on the wire.

```go
func (t *Templates) render(w http.ResponseWriter, name string, data PageData) {
    tmpl, ok := t.pages[name]
    if !ok {
        http.Error(w, "unknown template: "+name, http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
        // Headers are already sent — we can only log at this point.
        slog.Error("template render error", "page", name, "err", err)
    }
}
```

`renderNotFound` is the same pattern minus the data payload — the 404 template is parsed standalone so it has no dependency on any page-specific block.

## Step 4 — Define the page data struct

`PageData` is the value passed to every `Execute`. It carries flash messages, form values for re-population on validation failure, per-field errors, and the domain payload. Templates access fields with `{{.Flash}}`, `{{.Entries}}`, etc.

```go
type PageData struct {
    Flash     string            // transient message shown once
    FlashType string            // CSS class: "success" or "error"
    Entries   []*Entry          // guestbook entries
    Form      map[string]string // re-populated form values after a failed POST
    Errors    map[string]string // per-field validation errors
}
```

A flat struct beats a nested view-model for templating: every field shows up as a top-level dot expression, which is the most readable form for designers reading the templates without writing Go.

## Step 5 — Validate the guestbook form

Validation is a pure function returning a `formErrors` map (field name → message) and a boolean. The handler decides what to do on failure (re-render the page, in this example); the validator stays free of HTTP concerns.

```go
func validateGuestbook(name, message string) (formErrors, bool) {
    errs := make(formErrors)
    if strings.TrimSpace(name) == "" {
        errs["name"] = "Name is required."
    } else if len(name) > 100 {
        errs["name"] = "Name must be 100 characters or fewer."
    }
    if strings.TrimSpace(message) == "" {
        errs["message"] = "Message is required."
    } else if len(message) > 1000 {
        errs["message"] = "Message must be 1000 characters or fewer."
    }
    return errs, len(errs) == 0
}
```

Returning the map of errors (rather than a single error) is what enables per-field highlighting in the re-rendered form — the template loops over `.Errors.name`, `.Errors.message` and styles each input independently.

## Step 6 — Construct the router and wire the themed 404 + global middleware

`r.NotFound` is the override slot for the dispatcher's not-found handler. The example renders the themed 404 template; everything else stays consistent with the other examples (`RequestID`, `Logger`, `Recoverer`, plus `CleanPath` in `Pre` so URLs with double slashes are normalised before route matching).

```go
r := mm.New()

r.NotFound = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
    tmpl.renderNotFound(w)
})

r.Use(
    mw.RequestID(),
    mw.Logger(os.Stdout),
    mw.RecovererWithLogger(log),
)

r.Pre(mw.CleanPath())
```

`CleanPath` belongs in `Pre` because it must run before the radix tree resolves the URL — the cleaned path is what gets matched.

## Step 7 — Serve the embedded `/static/*` files

`fs.Sub` re-roots the embedded filesystem at `"static/"` so `ServeFiles` receives paths like `/style.css` (the catch-all `*filepath` value) and finds the file directly. No filesystem layout decisions leak into the URL space.

```go
staticFS, err := fs.Sub(files, "static")
if err != nil {
    log.Error("cannot create static sub-FS", "err", err)
    os.Exit(1)
}
r.ServeFiles("/static/*filepath", http.FS(staticFS))
```

`ServeFiles` handles `Content-Type` from the file extension, conditional GETs (`If-None-Match`, `If-Modified-Since`), and `Range` requests — there is no need to write any of that by hand.

## Step 8 — Register the GET handlers (home, about, guestbook list)

The `/guestbook` GET reads the `?ok=1` query parameter the POST handler redirects to and surfaces it as a flash message in the response. No server-side session is required; the redirect URL is the entire mechanism.

```go
r.GET("/", func(w http.ResponseWriter, _ *http.Request) {
    tmpl.render(w, "home", PageData{})
})

r.GET("/about", func(w http.ResponseWriter, _ *http.Request) {
    tmpl.render(w, "about", PageData{})
})

r.GET("/guestbook", func(w http.ResponseWriter, r *http.Request) {
    data := PageData{Entries: store.all()}
    if r.URL.Query().Get("ok") == "1" {
        data.Flash = "Your entry has been added — thank you!"
        data.FlashType = "success"
    }
    tmpl.render(w, "guestbook", data)
})
```

Storing flash state in the URL keeps the example self-contained — the moment the user navigates away, the flag is gone.

## Step 9 — Apply POST–Redirect–GET on the form submission

The POST handler validates, stores on success, then redirects to GET (303 See Other). On validation failure the form re-renders inline with the submitted values (`Form` map) and per-field errors (`Errors` map) so the user loses no input.

```go
r.POST("/guestbook", func(w http.ResponseWriter, r *http.Request) {
    if err := r.ParseForm(); err != nil {
        http.Error(w, "bad request", http.StatusBadRequest)
        return
    }
    name := r.FormValue("name")
    message := r.FormValue("message")

    errs, ok := validateGuestbook(name, message)
    if !ok {
        // Re-render the form with submitted values and error messages.
        tmpl.render(w, "guestbook", PageData{
            Entries: store.all(),
            Form:    map[string]string{"name": name, "message": message},
            Errors:  errs,
        })
        return
    }

    store.add(name, message)

    // Redirect to GET to prevent duplicate submissions on browser refresh.
    http.Redirect(w, r, "/guestbook?ok=1", http.StatusSeeOther)
})
```

The 303 redirect is what defeats the "Resubmit form?" browser dialog: the browser remembers the redirect target as a GET, so refreshing the page re-fetches the list rather than re-submitting the form.

## Step 10 — Serve with hardened timeouts

The same timeout set as the other examples — closes the slowloris vector. Production deployments should also wrap the server start in a goroutine and drain on `SIGINT`/`SIGTERM` (see the graceful-shutdown example).

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

Setting these on the surrounding `http.Server` is the application's job — MuxMaster is an `http.Handler` and cannot configure them itself.

## Common questions

<section data-conversation="ssr-patterns">

### How do I render an HTML template from a MuxMaster handler?

Pre-parse one `*template.Template` per page at startup (so there is no runtime parse cost), then call `tmpl.ExecuteTemplate(w, "base", data)` from the handler. The example wraps this in a `Templates.render` helper that also sets `Content-Type: text/html; charset=utf-8` and logs render errors after headers are sent.

### How do I show a flash message exactly once after a successful POST?

Use POST–Redirect–GET: the POST handler stores the entry and replies with `303 See Other` to a URL carrying a `?ok=1` query parameter; the GET handler reads the parameter and renders the flash message. The example uses this pattern on `/guestbook` so refreshing the page does not re-submit the form.

### How do I re-populate the form when validation fails?

Pass the submitted values back to the template via `PageData.Form` (a `map[string]string`) and the per-field errors via `PageData.Errors`. The template reads them with `{{.Form.name}}` and `{{.Errors.name}}`. The example's validator returns the map directly so the handler does not have to translate error strings field-by-field.

</section>

## Upstream source

Every code excerpt above is lifted verbatim from [`examples/server-side-render/main.go`](https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.0.1/examples/server-side-render/main.go) at the v1.0.1 tag. The upstream directory also contains the `templates/` (base, home, about, guestbook, 404) and `static/style.css` files the example embeds via `//go:embed`.
