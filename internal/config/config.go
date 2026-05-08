// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
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
	Port              int
	SiteBaseURL       string
	MuxMasterSourceDir string
	LogLevel          slog.Level
	Env               Env
}

// Load reads configuration from the environment, applying defaults.
// It validates required values and returns an error on misconfiguration.
func Load() (*Config, error) {
	cfg := &Config{
		Port:              8080,
		SiteBaseURL:       "http://localhost:8080",
		MuxMasterSourceDir: "../MuxMaster",
		LogLevel:          slog.LevelInfo,
		Env:               EnvDevelopment,
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

	if v := os.Getenv("MUXMASTER_SOURCE_DIR"); v != "" {
		cfg.MuxMasterSourceDir = v
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

	if cfg.SiteBaseURL == "" {
		return nil, errors.New("config: SITE_BASE_URL must not be empty")
	}
	return cfg, nil
}

// LogAttrs returns the configuration as slog attributes for the startup log.
func (c *Config) LogAttrs() []slog.Attr {
	return []slog.Attr{
		slog.Int("port", c.Port),
		slog.String("site_base_url", c.SiteBaseURL),
		slog.String("muxmaster_source_dir", c.MuxMasterSourceDir),
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
