package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveAssetDirsRespectsExplicitEnv verifies the explicit
// override path: MUXMASTER_SITE_DIR=<root> yields <root>/templates and
// <root>/static regardless of the current working directory.
func TestResolveAssetDirsRespectsExplicitEnv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MUXMASTER_SITE_DIR", tmp)

	gotTpl, gotStatic := resolveAssetDirs()
	wantTpl := filepath.Join(tmp, "templates")
	wantStatic := filepath.Join(tmp, "static")
	if gotTpl != wantTpl {
		t.Errorf("templates dir = %q, want %q", gotTpl, wantTpl)
	}
	if gotStatic != wantStatic {
		t.Errorf("static dir = %q, want %q", gotStatic, wantStatic)
	}
}

// TestResolveAssetDirsFallsBackToCwd verifies that when MUXMASTER_SITE_DIR
// is empty and the current working directory contains a templates/ folder,
// the cwd-based resolution wins.
func TestResolveAssetDirsFallsBackToCwd(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "templates"), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}
	t.Setenv("MUXMASTER_SITE_DIR", "")
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	gotTpl, gotStatic := resolveAssetDirs()
	if !strings.HasSuffix(gotTpl, "templates") {
		t.Errorf("templates dir = %q, expected to end with 'templates'", gotTpl)
	}
	if !strings.HasSuffix(gotStatic, "static") {
		t.Errorf("static dir = %q, expected to end with 'static'", gotStatic)
	}
}

// TestRunFailsOnInvalidConfig verifies the entry-point's fail-fast
// contract: a malformed env causes run() to return an error rather than
// silently start a broken server.
func TestRunFailsOnInvalidConfig(t *testing.T) {
	t.Setenv("PORT", "not-a-number")

	if err := run(); err == nil {
		t.Fatalf("run() with invalid PORT returned no error")
	}
}
