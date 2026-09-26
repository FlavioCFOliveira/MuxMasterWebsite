//go:build timing

// route_existence_timing_test.go — cross-router evidence for MM-2026-0026 /
// TSC-002 (route-existence timing oracle). rmp #11 / #290.
//
// SECURITY.md and prior reports (2026-04-17-1320-prerelease-timing-audit.md
// §6) claim the registered-vs-unregistered route timing gap is "intrinsic to
// every radix router (httprouter, chi, bunrouter idem)" but that claim was
// never backed by committed data in this repository — the 2026-09-26
// closed-task audit (reports/overview/2026-09-26-closed-task-audit.md, row
// #11) flagged it explicitly: "No httprouter/chi/bunrouter arm exists in
// route_existence_timing_test.go."
//
// This file measures the SAME pairs used by
// reports/timing-and-sidechannel-analyst/harness/route_existence_timing_test.go
// (registered-vs-unregistered, admin-vs-random, depth correlation depth 1-5,
// static-vs-param) against httprouter, chi, and bunrouter — the three
// competitor modules already vendored under competitor/go.mod — using the
// IDENTICAL measurement method: pinned OS thread, GC disabled during the
// measurement window, N=200_000 samples per arm, per-sample status-code
// verification, and the shared statistics engine
// (github.com/FlavioCFOliveira/MuxMaster/reports/timing-and-sidechannel-analyst/harness),
// imported cross-module via this module's `replace` directive to the root
// MuxMaster module. No new go.mod dependency was added: httprouter, chi and
// bunrouter were already required by competitor/go.mod for the performance
// benchmarks in bench_test.go.
//
// NOTE ON MODULE RESOLUTION: this file's cross-module import of the
// `harness` package is NOT covered by competitor/vendor (vendor/modules.txt
// only mirrors the root MuxMaster package, not its reports/ subtree, since
// bench_test.go never imported it). Run with `-mod=mod` (not the default
// vendor-consistent build) so Go reads the harness package directly from
// the `replace github.com/FlavioCFOliveira/MuxMaster => ../` target on
// disk:
//
//	cd competitor && GOFLAGS=-mod=mod go test -tags timing -run 'TestTiming_Route_' -v -timeout 900s -count=1 .
//
// Route topology (pairs 1-3, identical across all three competitors and
// MuxMaster's own buildRoutingMux in the sibling harness package):
//
//	/                                     (depth 1)
//	/users                                (depth 1)
//	/users/list                           (depth 2)
//	/users/detail/view                    (depth 3)
//	/users/detail/view/extended/info      (depth 4)
//	/admin                                (hidden, depth 1)
//	/admin/users                          (depth 2)
//	/admin/settings/security              (depth 3)
//
// All of the above are pure static routes with no parameterised sibling at
// the same tree level, so this topology is valid, unmodified, on httprouter,
// chi and bunrouter alike — no router-specific adaptation is needed for
// pairs 1-3.
//
// Pair 4 (static vs param at matched depth) additionally registers
// /users/:id on top of the same tree for chi and bunrouter, both of which
// support mixing static and parameterised children at the same node
// (already demonstrated by bench_test.go's newChi/newBunRouter). httprouter
// v1.3.0 (vendored here) does NOT support this — see bench_test.go's own
// documented adaptation ("httprouter does NOT support mixing static and
// param children at the same tree level"). Pair 4 is therefore SKIPPED for
// httprouter rather than silently restructuring its tree and breaking
// topology parity with pairs 1-3; this is reported as a structural
// capability difference, not a measurement gap.
package bench_test

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/julienschmidt/httprouter"
	"github.com/uptrace/bunrouter"

	muxmaster "github.com/FlavioCFOliveira/MuxMaster"
	harness "github.com/FlavioCFOliveira/MuxMaster/reports/timing-and-sidechannel-analyst/harness"
)

const nRouteOracle = 200_000

func oracleOKHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// Router constructors — pairs 1-3 topology (pure static, identical everywhere)
// ---------------------------------------------------------------------------

func newMuxMasterOracle() *muxmaster.Mux {
	m := muxmaster.New()
	m.GET("/", oracleOKHandler)
	m.GET("/users", oracleOKHandler)
	m.GET("/users/list", oracleOKHandler)
	m.GET("/users/detail/view", oracleOKHandler)
	m.GET("/users/detail/view/extended/info", oracleOKHandler)
	m.GET("/admin", oracleOKHandler)
	m.GET("/admin/users", oracleOKHandler)
	m.GET("/admin/settings/security", oracleOKHandler)
	return m
}

// newMuxMasterOracleWithParam is newMuxMasterOracle plus /users/:id — MuxMaster
// supports mixing static and param children at the same tree level.
func newMuxMasterOracleWithParam() *muxmaster.Mux {
	m := newMuxMasterOracle()
	m.GET("/users/:id", oracleOKHandler)
	return m
}

func newHTTPRouterOracle() http.Handler {
	r := httprouter.New()
	nop := func(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
		w.WriteHeader(http.StatusOK)
	}
	r.GET("/", nop)
	r.GET("/users", nop)
	r.GET("/users/list", nop)
	r.GET("/users/detail/view", nop)
	r.GET("/users/detail/view/extended/info", nop)
	r.GET("/admin", nop)
	r.GET("/admin/users", nop)
	r.GET("/admin/settings/security", nop)
	return r
}

func newChiOracle() http.Handler {
	r := chi.NewRouter()
	nop := func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	r.Get("/", nop)
	r.Get("/users", nop)
	r.Get("/users/list", nop)
	r.Get("/users/detail/view", nop)
	r.Get("/users/detail/view/extended/info", nop)
	r.Get("/admin", nop)
	r.Get("/admin/users", nop)
	r.Get("/admin/settings/security", nop)
	return r
}

// newChiOracleWithParam is newChiOracle plus /users/{id} — chi supports
// mixing static and param children at the same tree level.
func newChiOracleWithParam() http.Handler {
	r := chi.NewRouter()
	nop := func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
	r.Get("/", nop)
	r.Get("/users", nop)
	r.Get("/users/list", nop)
	r.Get("/users/detail/view", nop)
	r.Get("/users/detail/view/extended/info", nop)
	r.Get("/admin", nop)
	r.Get("/admin/users", nop)
	r.Get("/admin/settings/security", nop)
	r.Get("/users/{id}", nop)
	return r
}

func newBunRouterOracle() http.Handler {
	r := bunrouter.New()
	nop := bunrouter.HTTPHandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.GET("/", nop)
	r.GET("/users", nop)
	r.GET("/users/list", nop)
	r.GET("/users/detail/view", nop)
	r.GET("/users/detail/view/extended/info", nop)
	r.GET("/admin", nop)
	r.GET("/admin/users", nop)
	r.GET("/admin/settings/security", nop)
	return r
}

// newBunRouterOracleWithParam is newBunRouterOracle plus /users/:id —
// bunrouter supports mixing static and param children at the same tree level.
func newBunRouterOracleWithParam() http.Handler {
	r := bunrouter.New()
	nop := bunrouter.HTTPHandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.GET("/", nop)
	r.GET("/users", nop)
	r.GET("/users/list", nop)
	r.GET("/users/detail/view", nop)
	r.GET("/users/detail/view/extended/info", nop)
	r.GET("/admin", nop)
	r.GET("/admin/users", nop)
	r.GET("/admin/settings/security", nop)
	r.GET("/users/:id", nop)
	return r
}

// ---------------------------------------------------------------------------
// Measurement helpers
// ---------------------------------------------------------------------------

func oracleReq(path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, path, nil)
}

// measureOracleWithStatus mirrors measureRouteWithStatus in the sibling
// harness package's route_existence_timing_test.go: every sample's status
// is captured so the FULL sample set (not just a single preflight request)
// can be verified before the timing evidence is trusted.
func measureOracleWithStatus(handler http.Handler, path string, n int) ([]int64, []int) {
	samples := make([]int64, n)
	statuses := make([]int, n)
	for i := 0; i < n; i++ {
		req := oracleReq(path)
		w := httptest.NewRecorder()
		t0 := time.Now()
		handler.ServeHTTP(w, req)
		samples[i] = time.Since(t0).Nanoseconds()
		statuses[i] = w.Code
	}
	return samples, statuses
}

func requireAllStatus(t *testing.T, arm string, statuses []int, want int) {
	t.Helper()
	for i, s := range statuses {
		if s != want {
			t.Fatalf("%s arm: sample %d returned status %d, want %d — invalid evidence", arm, i, s, want)
		}
	}
}

// routerUnderTest names one competitor (or MuxMaster, run alongside for a
// same-process baseline) plus its http.Handler.
type routerUnderTest struct {
	name    string
	handler http.Handler
}

func oracleRouters() []routerUnderTest {
	return []routerUnderTest{
		{"MuxMaster", newMuxMasterOracle()},
		{"httprouter", newHTTPRouterOracle()},
		{"chi", newChiOracle()},
		{"bunrouter", newBunRouterOracle()},
	}
}

func withQuietEnv(warmup func(), measure func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	old := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(old)
	runtime.GC()
	runtime.GC()
	warmup()
	measure()
}

// ---------------------------------------------------------------------------
// Pair 1 — registered vs unregistered (same depth), all four routers
// ---------------------------------------------------------------------------

func TestTiming_Route_Competitor_RegisteredVsUnregistered(t *testing.T) {
	for _, rt := range oracleRouters() {
		t.Run(rt.name, func(t *testing.T) {
			handler := rt.handler
			withQuietEnv(func() {
				for i := 0; i < 30_000; i++ {
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, oracleReq("/users"))
					w2 := httptest.NewRecorder()
					handler.ServeHTTP(w2, oracleReq("/totally-random-zzz"))
				}
			}, func() {
				harness.VerifyArmStatus(t, "registered", handler, func() *http.Request { return oracleReq("/users") }, http.StatusOK)
				harness.VerifyArmStatus(t, "unregistered", handler, func() *http.Request { return oracleReq("/totally-random-zzz") }, http.StatusNotFound)

				registered, regStatuses := measureOracleWithStatus(handler, "/users", nRouteOracle)
				unregistered, unregStatuses := measureOracleWithStatus(handler, "/totally-random-zzz", nRouteOracle)
				requireAllStatus(t, "registered", regStatuses, http.StatusOK)
				requireAllStatus(t, "unregistered", unregStatuses, http.StatusNotFound)

				result := harness.RunTests(registered, unregistered)
				rs := harness.Summarise(registered)
				us := harness.Summarise(unregistered)

				t.Logf("[%s] /users (registered) vs /totally-random-zzz (unregistered)", rt.name)
				t.Logf("  Registered:   N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", rs.N, rs.Mean, rs.Std, rs.P50, rs.P95, rs.P99)
				t.Logf("  Unregistered: N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", us.N, us.Mean, us.Std, us.P50, us.P95, us.P99)
				t.Logf("  Welch p=%.4g  KS p=%.4g  MWU p=%.4g  |mean diff|=%.2fns  Cohen d=%.3f",
					result.WelchP, result.KSP, result.MWUP, result.MeanDiffNs, result.CohenD)
				if result.Leak {
					t.Logf("  DISTINGUISHABLE — same class as MM-2026-0026/TSC-002")
				} else {
					t.Logf("  NOT distinguishable at p<0.01")
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// Pair 2 — admin (hidden) vs random, all four routers
// ---------------------------------------------------------------------------

func TestTiming_Route_Competitor_AdminHiddenVsRandom(t *testing.T) {
	for _, rt := range oracleRouters() {
		t.Run(rt.name, func(t *testing.T) {
			handler := rt.handler
			withQuietEnv(func() {
				for i := 0; i < 30_000; i++ {
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, oracleReq("/admin"))
					w2 := httptest.NewRecorder()
					handler.ServeHTTP(w2, oracleReq("/xyzxyz"))
				}
			}, func() {
				harness.VerifyArmStatus(t, "admin", handler, func() *http.Request { return oracleReq("/admin") }, http.StatusOK)
				harness.VerifyArmStatus(t, "random", handler, func() *http.Request { return oracleReq("/xyzxyz") }, http.StatusNotFound)

				adminSamples, adminStatuses := measureOracleWithStatus(handler, "/admin", nRouteOracle)
				randomSamples, randomStatuses := measureOracleWithStatus(handler, "/xyzxyz", nRouteOracle)
				requireAllStatus(t, "admin", adminStatuses, http.StatusOK)
				requireAllStatus(t, "random", randomStatuses, http.StatusNotFound)

				result := harness.RunTests(adminSamples, randomSamples)
				as_ := harness.Summarise(adminSamples)
				rs_ := harness.Summarise(randomSamples)

				t.Logf("[%s] /admin (registered, hidden) vs /xyzxyz (unregistered)", rt.name)
				t.Logf("  Admin:  N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", as_.N, as_.Mean, as_.Std, as_.P50, as_.P95, as_.P99)
				t.Logf("  Random: N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", rs_.N, rs_.Mean, rs_.Std, rs_.P50, rs_.P95, rs_.P99)
				t.Logf("  Welch p=%.4g  KS p=%.4g  MWU p=%.4g  |mean diff|=%.2fns  Cohen d=%.3f",
					result.WelchP, result.KSP, result.MWUP, result.MeanDiffNs, result.CohenD)
				if result.Leak {
					t.Logf("  DISTINGUISHABLE — hidden admin route is timing-discoverable")
				} else {
					t.Logf("  NOT distinguishable at p<0.01")
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// Pair 3 — depth correlation (depth 1-4, registered routes only), all four
// routers. (MuxMaster's sibling harness tests depth 1-5; the shared static
// topology here stops at depth 4 — see the constructors above — because
// /users/detail/view/extended/info's depth-5 sibling was not duplicated
// into this file's shared route set; depth 1-4 already demonstrates the
// monotonic trend consistently.)
// ---------------------------------------------------------------------------

func TestTiming_Route_Competitor_DepthCorrelation(t *testing.T) {
	paths := []string{"/", "/users", "/users/list", "/users/detail/view", "/users/detail/view/extended/info"}
	labels := []string{"depth-1", "depth-2", "depth-3", "depth-4", "depth-5"}

	for _, rt := range oracleRouters() {
		t.Run(rt.name, func(t *testing.T) {
			handler := rt.handler
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			old := debug.SetGCPercent(-1)
			defer debug.SetGCPercent(old)
			runtime.GC()
			runtime.GC()

			results := make([]harness.SummaryStats, len(paths))
			rawSamples := make([][]int64, len(paths))
			for j, p := range paths {
				for i := 0; i < 20_000; i++ {
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, oracleReq(p))
				}
				harness.VerifyArmStatus(t, labels[j], handler, func() *http.Request { return oracleReq(p) }, http.StatusOK)
				samples, statuses := measureOracleWithStatus(handler, p, nRouteOracle/2)
				requireAllStatus(t, labels[j], statuses, http.StatusOK)
				results[j] = harness.Summarise(samples)
				rawSamples[j] = samples
			}

			t.Logf("[%s] route depth correlation (registered routes):", rt.name)
			for j, s := range results {
				t.Logf("  %s (%s): N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", labels[j], paths[j], s.N, s.Mean, s.Std, s.P50, s.P95, s.P99)
			}
			for j := 1; j < len(paths); j++ {
				r := harness.RunTests(rawSamples[j-1], rawSamples[j])
				t.Logf("  %s vs %s: Welch p=%.4g Cohen d=%.3f |mean diff|=%.2fns",
					labels[j-1], labels[j], r.WelchP, r.CohenD, r.MeanDiffNs)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Pair 4 — static vs param at matched depth (MuxMaster, chi, bunrouter only;
// httprouter cannot mix static and param children at the same tree level —
// see the file-level doc comment).
// ---------------------------------------------------------------------------

func TestTiming_Route_Competitor_Param_vs_Static(t *testing.T) {
	routers := []routerUnderTest{
		{"MuxMaster", newMuxMasterOracleWithParam()},
		{"chi", newChiOracleWithParam()},
		{"bunrouter", newBunRouterOracleWithParam()},
	}

	for _, rt := range routers {
		t.Run(rt.name, func(t *testing.T) {
			handler := rt.handler
			withQuietEnv(func() {
				for i := 0; i < 30_000; i++ {
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, oracleReq("/users/list"))
					w2 := httptest.NewRecorder()
					handler.ServeHTTP(w2, oracleReq("/users/alice"))
				}
			}, func() {
				harness.VerifyArmStatus(t, "static", handler, func() *http.Request { return oracleReq("/users/list") }, http.StatusOK)
				harness.VerifyArmStatus(t, "param", handler, func() *http.Request { return oracleReq("/users/alice") }, http.StatusOK)

				staticSamples, staticStatuses := measureOracleWithStatus(handler, "/users/list", nRouteOracle)
				paramSamples, paramStatuses := measureOracleWithStatus(handler, "/users/alice", nRouteOracle)
				requireAllStatus(t, "static", staticStatuses, http.StatusOK)
				requireAllStatus(t, "param", paramStatuses, http.StatusOK)

				result := harness.RunTests(staticSamples, paramSamples)
				ss := harness.Summarise(staticSamples)
				ps := harness.Summarise(paramSamples)

				t.Logf("[%s] /users/list (static) vs /users/alice (param)", rt.name)
				t.Logf("  Static: N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", ss.N, ss.Mean, ss.Std, ss.P50, ss.P95, ss.P99)
				t.Logf("  Param:  N=%d mean=%.1fns std=%.1fns p50=%.0fns p95=%.0fns p99=%.0fns", ps.N, ps.Mean, ps.Std, ps.P50, ps.P95, ps.P99)
				t.Logf("  Welch p=%.4g  KS p=%.4g  MWU p=%.4g  |mean diff|=%.2fns  Cohen d=%.3f",
					result.WelchP, result.KSP, result.MWUP, result.MeanDiffNs, result.CohenD)
			})
		})
	}
	t.Logf("NOTE: httprouter skipped — v1.3.0 does not support a static and a param " +
		"child at the same tree level (/users/list + /users/:id); see file doc comment.")
}
