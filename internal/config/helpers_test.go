package config

import (
	"os"
	"testing"
)

// lookupEnv proxies os.LookupEnv. Defined here so config_test.go can
// reference it without importing "os" directly — keeps the env-handling
// helpers in one place.
func lookupEnv(key string) (string, bool) { return os.LookupEnv(key) }

// unsetEnv proxies t.Setenv to clear a variable. Go's testing package
// reverts environment changes at cleanup; using t.Setenv("") would set
// the variable to the empty string rather than unset it, so we use
// os.Unsetenv with a Cleanup hook that restores the previous value.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if had {
		t.Cleanup(func() { _ = os.Setenv(key, prev) })
	}
	_ = os.Unsetenv(key)
}
