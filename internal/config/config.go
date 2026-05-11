// Package config loads runtime configuration from environment variables.
//
// The runtime binary is self-contained: every public route is rendered at
// startup from /content/ embedded at build time. Configuration is therefore
// limited to network and observability knobs (PORT, SITE_BASE_URL, ENV,
// LOG_LEVEL).
//
// MUXMASTER_SOURCE_DIR is intentionally NOT consumed here. It is a
// development-time and agent-time variable only, used by the content-curator
// agent to locate the upstream working tree (specification/deployment.md).
// Setting it in the runtime environment has no effect.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// Env is the deployment environment.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvStaging     Env = "staging"
	EnvProduction  Env = "production"
)

// Config holds all runtime configuration.
type Config struct {
	Port        int
	SiteBaseURL string
	LogLevel    slog.Level
	Env         Env
	// TrustedProxyCIDRs is the list of CIDR prefixes from which
	// X-Forwarded-* headers will be honoured. Empty means no proxy is
	// trusted and r.RemoteAddr is used verbatim — the safe default for
	// development. Production deployments set TRUSTED_PROXY_CIDRS to
	// the edge-proxy network(s); a forged X-Forwarded-For from a
	// non-trusted peer is then ignored.
	TrustedProxyCIDRs []netip.Prefix
}

// Load reads configuration from the environment, applying defaults.
// It validates required values and returns an error on misconfiguration.
func Load() (*Config, error) {
	cfg := &Config{
		Port:        8080,
		SiteBaseURL: "http://localhost:8080",
		LogLevel:    slog.LevelInfo,
		Env:         EnvDevelopment,
	}

	if v := os.Getenv("PORT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 65535 {
			return nil, fmt.Errorf("config: invalid PORT %q", v)
		}
		cfg.Port = n
	}

	if v := os.Getenv("SITE_BASE_URL"); v != "" {
		cfg.SiteBaseURL = strings.TrimRight(v, "/")
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		lvl, err := parseLevel(v)
		if err != nil {
			return nil, err
		}
		cfg.LogLevel = lvl
	}

	if v := os.Getenv("ENV"); v != "" {
		switch Env(v) {
		case EnvDevelopment, EnvStaging, EnvProduction:
			cfg.Env = Env(v)
		default:
			return nil, fmt.Errorf("config: invalid ENV %q (want development|staging|production)", v)
		}
	}

	if v := os.Getenv("TRUSTED_PROXY_CIDRS"); v != "" {
		cidrs, err := parseCIDRs(v)
		if err != nil {
			return nil, err
		}
		cfg.TrustedProxyCIDRs = cidrs
	}

	if cfg.SiteBaseURL == "" {
		return nil, errors.New("config: SITE_BASE_URL must not be empty")
	}
	return cfg, nil
}

// parseCIDRs parses a comma-separated list of CIDR prefixes; empty
// entries are skipped so trailing commas and surrounding whitespace do
// not break the input. Returns an error naming the first bad entry.
func parseCIDRs(s string) ([]netip.Prefix, error) {
	parts := strings.Split(s, ",")
	out := make([]netip.Prefix, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		px, err := netip.ParsePrefix(p)
		if err != nil {
			return nil, fmt.Errorf("config: invalid TRUSTED_PROXY_CIDRS entry %q: %w", p, err)
		}
		out = append(out, px)
	}
	return out, nil
}

// LogAttrs returns the configuration as slog attributes for the startup log.
func (c *Config) LogAttrs() []slog.Attr {
	return []slog.Attr{
		slog.Int("port", c.Port),
		slog.String("site_base_url", c.SiteBaseURL),
		slog.String("log_level", c.LogLevel.String()),
		slog.String("env", string(c.Env)),
	}
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("config: invalid LOG_LEVEL %q", s)
	}
}
