// Package bench provides comparative benchmarks for MuxMaster and its
// direct competitors: httprouter, bunrouter, and chi.
//
// Route dataset and lookup URLs match MuxMaster's bench_test.go exactly
// so that benchstat can produce direct comparisons.
//
// Methodology:
//   - All routers register identical routes
//   - Each benchmark reuses a single *http.Request and *httptest.ResponseRecorder
//     created outside the loop — measuring dispatch cost only
//   - Handlers write w.WriteHeader(http.StatusOK) to prevent dead-code elimination
//   - b.ReportAllocs() is mandatory on every benchmark
package bench_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	muxmaster "github.com/FlavioCFOliveira/MuxMaster"
	chi "github.com/go-chi/chi/v5"
	gorillamux "github.com/gorilla/mux"
	"github.com/julienschmidt/httprouter"
	"github.com/uptrace/bunrouter"
)

// ---------------------------------------------------------------------------
// Handler helpers
// ---------------------------------------------------------------------------

var okHandler http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// MuxMaster
// ---------------------------------------------------------------------------

func newMuxMaster() http.Handler {
	m := muxmaster.New()

	// Static routes
	m.GET("/", okHandler)
	m.GET("/users", okHandler)
	m.GET("/users/list", okHandler)
	m.GET("/users/search", okHandler)
	m.POST("/users", okHandler)
	m.GET("/products", okHandler)
	m.GET("/products/featured", okHandler)
	m.POST("/products", okHandler)
	m.GET("/health", okHandler)
	m.GET("/metrics", okHandler)

	// Param routes
	m.GET("/users/:id", okHandler)
	m.PUT("/users/:id", okHandler)
	m.DELETE("/users/:id", okHandler)
	m.GET("/users/:id/posts", okHandler)
	m.GET("/users/:id/posts/:pid", okHandler)
	m.GET("/products/:id", okHandler)
	m.PUT("/products/:id", okHandler)
	m.GET("/orgs/:org/repos/:repo/issues/:num", okHandler)

	// Wildcard routes
	m.GET("/static/*filepath", okHandler)
	m.GET("/docs/*path", okHandler)

	return m
}

// newMuxMasterPooled is newMuxMaster with PoolRequestBundle enabled (Opt O13).
func newMuxMasterPooled() http.Handler {
	m := muxmaster.New()
	m.PoolRequestBundle = true

	// Same route set as newMuxMaster.
	m.GET("/", okHandler)
	m.GET("/users", okHandler)
	m.GET("/users/list", okHandler)
	m.GET("/users/search", okHandler)
	m.POST("/users", okHandler)
	m.GET("/products", okHandler)
	m.GET("/products/featured", okHandler)
	m.POST("/products", okHandler)
	m.GET("/health", okHandler)
	m.GET("/metrics", okHandler)
	m.GET("/users/:id", okHandler)
	m.PUT("/users/:id", okHandler)
	m.DELETE("/users/:id", okHandler)
	m.GET("/users/:id/posts", okHandler)
	m.GET("/users/:id/posts/:pid", okHandler)
	m.GET("/products/:id", okHandler)
	m.PUT("/products/:id", okHandler)
	m.GET("/orgs/:org/repos/:repo/issues/:num", okHandler)
	m.GET("/static/*filepath", okHandler)
	m.GET("/docs/*path", okHandler)

	return m
}

// ---------------------------------------------------------------------------
// httprouter
// ---------------------------------------------------------------------------

// newHTTPRouter builds an httprouter instance.
//
// httprouter does NOT support mixing static and param children at the same
// tree level (e.g. /users/list and /users/:id both under /users/).
// The route set is adapted: static-only subtrees (/health, /metrics, /info)
// are kept fully static; param subtrees (/users, /items, /orgs) have no
// static siblings. The benchmark lookup URLs (/health, /users/42, etc.) are
// identical across all routers so cost comparisons remain valid.
func newHTTPRouter() http.Handler {
	r := httprouter.New()

	nop := func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusOK)
	}

	// Pure static routes (no param siblings at any level)
	r.GET("/", nop)
	r.GET("/health", nop)
	r.GET("/metrics", nop)
	r.GET("/info", nop)
	r.GET("/info/version", nop) // static benchmark: /info/version
	r.GET("/info/status", nop)

	// /users subtree — param routes only (no /users/list, /users/search)
	r.GET("/users", nop)
	r.POST("/users", nop)
	r.GET("/users/:id", nop)
	r.PUT("/users/:id", nop)
	r.DELETE("/users/:id", nop)
	r.GET("/users/:id/posts", nop)
	r.GET("/users/:id/posts/:pid", nop)

	// /items subtree — param routes only (mirrors /products)
	r.GET("/items", nop)
	r.POST("/items", nop)
	r.GET("/items/:id", nop)
	r.PUT("/items/:id", nop)

	r.GET("/orgs/:org/repos/:repo/issues/:num", nop)

	// Wildcard routes
	r.GET("/static/*filepath", nop)
	r.GET("/docs/*path", nop)

	return r
}

// ---------------------------------------------------------------------------
// bunrouter
// ---------------------------------------------------------------------------

func newBunRouter() http.Handler {
	r := bunrouter.New()

	nop := bunrouter.HTTPHandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Static routes
	r.GET("/", nop)
	r.GET("/users", nop)
	r.GET("/users/list", nop)
	r.GET("/users/search", nop)
	r.POST("/users", nop)
	r.GET("/products", nop)
	r.GET("/products/featured", nop)
	r.POST("/products", nop)
	r.GET("/health", nop)
	r.GET("/metrics", nop)

	// Param routes
	r.GET("/users/:id", nop)
	r.PUT("/users/:id", nop)
	r.DELETE("/users/:id", nop)
	r.GET("/users/:id/posts", nop)
	r.GET("/users/:id/posts/:pid", nop)
	r.GET("/products/:id", nop)
	r.PUT("/products/:id", nop)
	r.GET("/orgs/:org/repos/:repo/issues/:num", nop)

	// Wildcard routes — bunrouter uses *wildcard syntax
	r.GET("/static/*filepath", nop)
	r.GET("/docs/*path", nop)

	return r
}

// ---------------------------------------------------------------------------
// chi
// ---------------------------------------------------------------------------

func newChi() http.Handler {
	r := chi.NewRouter()

	nop := func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	// Static routes
	r.Get("/", nop)
	r.Get("/users", nop)
	r.Get("/users/list", nop)
	r.Get("/users/search", nop)
	r.Post("/users", nop)
	r.Get("/products", nop)
	r.Get("/products/featured", nop)
	r.Post("/products", nop)
	r.Get("/health", nop)
	r.Get("/metrics", nop)

	// Param routes — chi uses {param} syntax
	r.Get("/users/{id}", nop)
	r.Put("/users/{id}", nop)
	r.Delete("/users/{id}", nop)
	r.Get("/users/{id}/posts", nop)
	r.Get("/users/{id}/posts/{pid}", nop)
	r.Get("/products/{id}", nop)
	r.Put("/products/{id}", nop)
	r.Get("/orgs/{org}/repos/{repo}/issues/{num}", nop)

	// Wildcard routes — chi uses * at the end
	r.Get("/static/*", nop)
	r.Get("/docs/*", nop)

	return r
}

// ---------------------------------------------------------------------------
// gorilla/mux
// ---------------------------------------------------------------------------
//
// gorilla/mux is regex-based: every route registration compiles a regexp
// per pattern, and every request lookup runs the routes linearly until one
// matches. Path parameters use {name} syntax (same shape as chi); catch-all
// is expressed via {name:.*} with a regex pattern. This is the slowest of
// the routers in the comparison (commonly cited as ~100–500x slower than
// httprouter in published benchmarks) but is the most widely deployed in
// legacy Go HTTP codebases.

func newGorillaMux() http.Handler {
	r := gorillamux.NewRouter()

	nop := func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}

	// Static routes
	r.HandleFunc("/", nop).Methods(http.MethodGet)
	r.HandleFunc("/users", nop).Methods(http.MethodGet)
	r.HandleFunc("/users/list", nop).Methods(http.MethodGet)
	r.HandleFunc("/users/search", nop).Methods(http.MethodGet)
	r.HandleFunc("/users", nop).Methods(http.MethodPost)
	r.HandleFunc("/products", nop).Methods(http.MethodGet)
	r.HandleFunc("/products/featured", nop).Methods(http.MethodGet)
	r.HandleFunc("/products", nop).Methods(http.MethodPost)
	r.HandleFunc("/health", nop).Methods(http.MethodGet)
	r.HandleFunc("/metrics", nop).Methods(http.MethodGet)

	// Param routes — gorilla/mux uses {param} syntax (compiles a regexp per route)
	r.HandleFunc("/users/{id}", nop).Methods(http.MethodGet)
	r.HandleFunc("/users/{id}", nop).Methods(http.MethodPut)
	r.HandleFunc("/users/{id}", nop).Methods(http.MethodDelete)
	r.HandleFunc("/users/{id}/posts", nop).Methods(http.MethodGet)
	r.HandleFunc("/users/{id}/posts/{pid}", nop).Methods(http.MethodGet)
	r.HandleFunc("/products/{id}", nop).Methods(http.MethodGet)
	r.HandleFunc("/products/{id}", nop).Methods(http.MethodPut)
	r.HandleFunc("/orgs/{org}/repos/{repo}/issues/{num}", nop).Methods(http.MethodGet)

	// Catch-all — gorilla/mux uses {name:.*} regex syntax
	r.HandleFunc("/static/{filepath:.*}", nop).Methods(http.MethodGet)
	r.HandleFunc("/docs/{path:.*}", nop).Methods(http.MethodGet)

	return r
}

// ---------------------------------------------------------------------------
// MuxMaster benchmarks
// ---------------------------------------------------------------------------

func BenchmarkMuxMasterStaticRoute(b *testing.B) {
	h := newMuxMaster()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/list", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterParamRoute1(b *testing.B) {
	h := newMuxMaster()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterParamRoute2(b *testing.B) {
	h := newMuxMaster()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterParamRoute3(b *testing.B) {
	h := newMuxMaster()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterWildcardRoute(b *testing.B) {
	h := newMuxMaster()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterNotFound(b *testing.B) {
	h := newMuxMaster()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterParallelStaticRoute(b *testing.B) {
	h := newMuxMaster()
	r := httptest.NewRequest(http.MethodGet, "/products/featured", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

func BenchmarkMuxMasterParallelParamRoute(b *testing.B) {
	h := newMuxMaster()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

// ---------------------------------------------------------------------------
// httprouter benchmarks
// ---------------------------------------------------------------------------

func BenchmarkHTTProuterStaticRoute(b *testing.B) {
	h := newHTTPRouter()
	w := httptest.NewRecorder()
	// httprouter cannot mix static+param siblings; use /info/version (pure static)
	r := httptest.NewRequest(http.MethodGet, "/info/version", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkHTTProuterParamRoute1(b *testing.B) {
	h := newHTTPRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkHTTProuterParamRoute2(b *testing.B) {
	h := newHTTPRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkHTTProuterParamRoute3(b *testing.B) {
	h := newHTTPRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkHTTProuterWildcardRoute(b *testing.B) {
	h := newHTTPRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkHTTProuterNotFound(b *testing.B) {
	h := newHTTPRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkHTTProuterParallelStaticRoute(b *testing.B) {
	h := newHTTPRouter()
	r := httptest.NewRequest(http.MethodGet, "/info/version", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

func BenchmarkHTTProuterParallelParamRoute(b *testing.B) {
	h := newHTTPRouter()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

// ---------------------------------------------------------------------------
// bunrouter benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBunRouterStaticRoute(b *testing.B) {
	h := newBunRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/list", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkBunRouterParamRoute1(b *testing.B) {
	h := newBunRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkBunRouterParamRoute2(b *testing.B) {
	h := newBunRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkBunRouterParamRoute3(b *testing.B) {
	h := newBunRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkBunRouterWildcardRoute(b *testing.B) {
	h := newBunRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkBunRouterNotFound(b *testing.B) {
	h := newBunRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkBunRouterParallelStaticRoute(b *testing.B) {
	h := newBunRouter()
	r := httptest.NewRequest(http.MethodGet, "/products/featured", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

func BenchmarkBunRouterParallelParamRoute(b *testing.B) {
	h := newBunRouter()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

// ---------------------------------------------------------------------------
// chi benchmarks
// ---------------------------------------------------------------------------

func BenchmarkChiStaticRoute(b *testing.B) {
	h := newChi()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/list", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkChiParamRoute1(b *testing.B) {
	h := newChi()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkChiParamRoute2(b *testing.B) {
	h := newChi()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkChiParamRoute3(b *testing.B) {
	h := newChi()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkChiWildcardRoute(b *testing.B) {
	h := newChi()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkChiNotFound(b *testing.B) {
	h := newChi()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkChiParallelStaticRoute(b *testing.B) {
	h := newChi()
	r := httptest.NewRequest(http.MethodGet, "/products/featured", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

func BenchmarkChiParallelParamRoute(b *testing.B) {
	h := newChi()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

// ---------------------------------------------------------------------------
// MuxMaster Fast (HandleFast API)
// ---------------------------------------------------------------------------

func newMuxMasterFast() http.Handler {
	nopFast := func(w http.ResponseWriter, r *http.Request, ps muxmaster.Params) {
		w.WriteHeader(http.StatusOK)
	}
	m := muxmaster.New()

	// Static routes (unchanged from newMuxMaster — same dispatch path)
	m.GET("/", okHandler)
	m.GET("/users", okHandler)
	m.GET("/users/list", okHandler)
	m.GET("/users/search", okHandler)
	m.POST("/users", okHandler)
	m.GET("/products", okHandler)
	m.GET("/products/featured", okHandler)
	m.POST("/products", okHandler)
	m.GET("/health", okHandler)
	m.GET("/metrics", okHandler)

	// Param and wildcard routes registered via HandleFast
	m.GETFast("/users/:id", nopFast)
	m.PUTFast("/users/:id", nopFast)
	m.DELETEFast("/users/:id", nopFast)
	m.GETFast("/users/:id/posts", nopFast)
	m.GETFast("/users/:id/posts/:pid", nopFast)
	m.GETFast("/products/:id", nopFast)
	m.PUTFast("/products/:id", nopFast)
	m.GETFast("/orgs/:org/repos/:repo/issues/:num", nopFast)
	m.GETFast("/static/*filepath", nopFast)
	m.GETFast("/docs/*path", nopFast)

	return m
}

func BenchmarkMuxMasterFastStaticRoute(b *testing.B) {
	h := newMuxMasterFast()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/list", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterFastParamRoute1(b *testing.B) {
	h := newMuxMasterFast()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterFastParamRoute2(b *testing.B) {
	h := newMuxMasterFast()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterFastParamRoute3(b *testing.B) {
	h := newMuxMasterFast()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterFastWildcardRoute(b *testing.B) {
	h := newMuxMasterFast()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterFastParallelStaticRoute(b *testing.B) {
	h := newMuxMasterFast()
	r := httptest.NewRequest(http.MethodGet, "/products/featured", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

func BenchmarkMuxMasterFastParallelParamRoute(b *testing.B) {
	h := newMuxMasterFast()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

// ---------------------------------------------------------------------------
// MuxMaster Pooled (Opt O13: PoolRequestBundle = true)
// ---------------------------------------------------------------------------

func BenchmarkMuxMasterPooledStaticRoute(b *testing.B) {
	h := newMuxMasterPooled()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/list", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterPooledParamRoute1(b *testing.B) {
	h := newMuxMasterPooled()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterPooledParamRoute2(b *testing.B) {
	h := newMuxMasterPooled()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterPooledParamRoute3(b *testing.B) {
	h := newMuxMasterPooled()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterPooledWildcardRoute(b *testing.B) {
	h := newMuxMasterPooled()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkMuxMasterPooledParallelParamRoute(b *testing.B) {
	h := newMuxMasterPooled()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

// ---------------------------------------------------------------------------
// gorilla/mux benchmarks
// ---------------------------------------------------------------------------

func BenchmarkGorillaMuxStaticRoute(b *testing.B) {
	h := newGorillaMux()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/list", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkGorillaMuxParamRoute1(b *testing.B) {
	h := newGorillaMux()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkGorillaMuxParamRoute2(b *testing.B) {
	h := newGorillaMux()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkGorillaMuxParamRoute3(b *testing.B) {
	h := newGorillaMux()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/orgs/acme/repos/api/issues/123", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkGorillaMuxWildcardRoute(b *testing.B) {
	h := newGorillaMux()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/static/css/main.min.css", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkGorillaMuxNotFound(b *testing.B) {
	h := newGorillaMux()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/this/path/does/not/exist", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		h.ServeHTTP(w, r)
	}
}

func BenchmarkGorillaMuxParallelStaticRoute(b *testing.B) {
	h := newGorillaMux()
	r := httptest.NewRequest(http.MethodGet, "/products/featured", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}

func BenchmarkGorillaMuxParallelParamRoute(b *testing.B) {
	h := newGorillaMux()
	r := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		w := httptest.NewRecorder()
		for pb.Next() {
			h.ServeHTTP(w, r)
		}
	})
}
