package config

import (
	"log/slog"
	"strings"
	"testing"
)

// withEnv runs fn with the supplied environment variables set, restoring
// the previous values on return. Tests use it to exercise config.Load
// against a deterministic environment without polluting sibling tests.
func withEnv(t *testing.T, vars map[string]string, fn func()) {
	t.Helper()
	keys := []string{"PORT", "SITE_BASE_URL", "LOG_LEVEL", "ENV"}
	old := make(map[string]string, len(keys))
	had := make(map[string]bool, len(keys))
	for _, k := range keys {
		v, ok := t.Setenv, k // capture used to silence ineffassign
		_ = v
		_ = ok
		if prev, present := lookupEnv(k); present {
			old[k] = prev
			had[k] = true
		}
		unsetEnv(t, k)
	}
	for k, v := range vars {
		t.Setenv(k, v)
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if had[k] {
				t.Setenv(k, old[k])
			} else {
				unsetEnv(t, k)
			}
		}
	})
	fn()
}

// lookupEnv / unsetEnv are thin wrappers so the helpers above remain
// readable; both rely on the stdlib's os package via the indirection
// in helpers_test.go.

func TestLoadDefaults(t *testing.T) {
	withEnv(t, nil, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load with no env: %v", err)
		}
		if cfg.Port != 8080 {
			t.Errorf("Port=%d, want 8080", cfg.Port)
		}
		if cfg.SiteBaseURL != "http://localhost:8080" {
			t.Errorf("SiteBaseURL=%q, want http://localhost:8080", cfg.SiteBaseURL)
		}
		if cfg.LogLevel != slog.LevelInfo {
			t.Errorf("LogLevel=%v, want Info", cfg.LogLevel)
		}
		if cfg.Env != EnvDevelopment {
			t.Errorf("Env=%q, want development", cfg.Env)
		}
	})
}

func TestLoadValidOverrides(t *testing.T) {
	withEnv(t, map[string]string{
		"PORT":          "9090",
		"SITE_BASE_URL": "https://muxmaster.dev/", // trailing slash trimmed
		"LOG_LEVEL":     "debug",
		"ENV":           "production",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Port != 9090 {
			t.Errorf("Port=%d, want 9090", cfg.Port)
		}
		if cfg.SiteBaseURL != "https://muxmaster.dev" {
			t.Errorf("SiteBaseURL=%q, want trailing slash trimmed", cfg.SiteBaseURL)
		}
		if cfg.LogLevel != slog.LevelDebug {
			t.Errorf("LogLevel=%v, want Debug", cfg.LogLevel)
		}
		if cfg.Env != EnvProduction {
			t.Errorf("Env=%q, want production", cfg.Env)
		}
	})
}

func TestLoadInvalidPort(t *testing.T) {
	cases := []string{"", "0", "-1", "65536", "abc", "70000"}
	// Empty PORT is treated as default by Load, so test only the rejects.
	for _, p := range cases[1:] {
		p := p
		t.Run("PORT="+p, func(t *testing.T) {
			withEnv(t, map[string]string{"PORT": p}, func() {
				_, err := Load()
				if err == nil {
					t.Fatalf("expected error for PORT=%q, got nil", p)
				}
				if !strings.Contains(err.Error(), "invalid PORT") {
					t.Errorf("error %v does not mention 'invalid PORT'", err)
				}
			})
		})
	}
}

func TestLoadInvalidEnv(t *testing.T) {
	withEnv(t, map[string]string{"ENV": "prod"}, func() {
		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for ENV=prod")
		}
		if !strings.Contains(err.Error(), "invalid ENV") {
			t.Errorf("error %v does not mention 'invalid ENV'", err)
		}
	})
}

func TestLoadInvalidLogLevel(t *testing.T) {
	withEnv(t, map[string]string{"LOG_LEVEL": "trace"}, func() {
		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for LOG_LEVEL=trace")
		}
		if !strings.Contains(err.Error(), "invalid LOG_LEVEL") {
			t.Errorf("error %v does not mention 'invalid LOG_LEVEL'", err)
		}
	})
}

func TestLoadEmptySiteBaseURL(t *testing.T) {
	// An empty SITE_BASE_URL must fail the post-check. An empty env
	// variable is treated as "not set" so we need to pass a single space
	// then have it trimmed to "" by TrimRight only-if it is "/" — but
	// TrimRight does not strip whitespace; the easiest reproduction is
	// SITE_BASE_URL set to exactly "/" so TrimRight strips it to "".
	withEnv(t, map[string]string{"SITE_BASE_URL": "/"}, func() {
		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for SITE_BASE_URL=/")
		}
		if !strings.Contains(err.Error(), "SITE_BASE_URL must not be empty") {
			t.Errorf("error %v does not mention SITE_BASE_URL empty", err)
		}
	})
}

func TestParseLevelAllAccepted(t *testing.T) {
	for _, in := range []string{"debug", "DEBUG", "info", "INFO", "warn", "warning", "WARN", "error", "ERROR"} {
		if _, err := parseLevel(in); err != nil {
			t.Errorf("parseLevel(%q): %v", in, err)
		}
	}
}

func TestLoadTrustedProxyCIDRs(t *testing.T) {
	withEnv(t, map[string]string{"TRUSTED_PROXY_CIDRS": "127.0.0.0/8, 10.0.0.0/8 , 192.168.0.0/16,"}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(cfg.TrustedProxyCIDRs) != 3 {
			t.Fatalf("got %d cidrs, want 3", len(cfg.TrustedProxyCIDRs))
		}
		want := []string{"127.0.0.0/8", "10.0.0.0/8", "192.168.0.0/16"}
		for i, p := range cfg.TrustedProxyCIDRs {
			if p.String() != want[i] {
				t.Errorf("cidr[%d]=%q, want %q", i, p.String(), want[i])
			}
		}
	})
}

func TestLoadTrustedProxyCIDRsEmptyByDefault(t *testing.T) {
	withEnv(t, nil, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(cfg.TrustedProxyCIDRs) != 0 {
			t.Errorf("default TrustedProxyCIDRs must be empty (no proxy is trusted), got %v", cfg.TrustedProxyCIDRs)
		}
	})
}

func TestLoadInvalidTrustedProxyCIDR(t *testing.T) {
	withEnv(t, map[string]string{"TRUSTED_PROXY_CIDRS": "not-a-cidr"}, func() {
		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for TRUSTED_PROXY_CIDRS=not-a-cidr")
		}
		if !strings.Contains(err.Error(), "invalid TRUSTED_PROXY_CIDRS entry") {
			t.Errorf("error %v does not mention 'invalid TRUSTED_PROXY_CIDRS entry'", err)
		}
	})
}

func TestLogAttrs(t *testing.T) {
	cfg := &Config{Port: 8080, SiteBaseURL: "https://x.example", LogLevel: slog.LevelInfo, Env: EnvStaging}
	attrs := cfg.LogAttrs()
	if len(attrs) != 4 {
		t.Fatalf("got %d attrs, want 4", len(attrs))
	}
	wantKeys := map[string]bool{"port": true, "site_base_url": true, "log_level": true, "env": true}
	for _, a := range attrs {
		if !wantKeys[a.Key] {
			t.Errorf("unexpected attr key %q", a.Key)
		}
	}
}
