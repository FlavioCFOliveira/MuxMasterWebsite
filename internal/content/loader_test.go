package content

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// completeFS returns a fixture filesystem containing every required file
// (per content-sources.md "Required files"). Test cases that want a
// partial corpus mutate the map before calling NewLoader.
func completeFS() fstest.MapFS {
	now := time.Now()
	body := []byte("# placeholder\n\nbody.\n")
	fs := fstest.MapFS{}
	for _, p := range requiredFiles {
		fs[p] = &fstest.MapFile{Data: body, ModTime: now}
	}
	// changelog.md must contain a parseable version heading for the
	// Version() tests; overwrite the placeholder body for it.
	fs["changelog.md"] = &fstest.MapFile{
		Data:    []byte("# Changelog\n\n## [1.0.1] - 2026-05-08\n\nAll the things.\n"),
		ModTime: now,
	}
	return fs
}

func TestNewLoaderRejectsNilRoot(t *testing.T) {
	if _, err := NewLoader(nil); err == nil {
		t.Fatal("NewLoader(nil) returned no error")
	}
}

func TestVerifyHappyPath(t *testing.T) {
	loader, err := NewLoader(completeFS())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	if err := loader.Verify(); err != nil {
		t.Fatalf("Verify on complete corpus: %v", err)
	}
}

func TestVerifyReportsMissingFiles(t *testing.T) {
	fs := completeFS()
	delete(fs, "docs/routing.md")
	delete(fs, "examples/jwt.md")

	loader, _ := NewLoader(fs)
	err := loader.Verify()
	if err == nil {
		t.Fatal("Verify on incomplete corpus returned nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "docs/routing.md") {
		t.Errorf("Verify error %q does not mention docs/routing.md", msg)
	}
	if !strings.Contains(msg, "examples/jwt.md") {
		t.Errorf("Verify error %q does not mention examples/jwt.md", msg)
	}
}

func TestLoadReadsAFile(t *testing.T) {
	loader, _ := NewLoader(completeFS())
	body, err := loader.Load("docs/routing.md")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(string(body), "placeholder") {
		t.Errorf("Load returned %q, expected placeholder body", body)
	}
}

func TestLoadOnMissingFile(t *testing.T) {
	loader, _ := NewLoader(completeFS())
	if _, err := loader.Load("docs/does-not-exist.md"); err == nil {
		t.Fatal("Load on missing file returned no error")
	}
}

func TestExistsTrueAndFalse(t *testing.T) {
	loader, _ := NewLoader(completeFS())
	if !loader.Exists("api.md") {
		t.Error("Exists(api.md) = false, want true")
	}
	if loader.Exists("docs/missing.md") {
		t.Error("Exists(missing) = true, want false")
	}
}

func TestMtimeReturnsModTime(t *testing.T) {
	ts := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	fs := completeFS()
	fs["api.md"] = &fstest.MapFile{Data: []byte("x"), ModTime: ts}
	loader, _ := NewLoader(fs)
	got, err := loader.Mtime("api.md")
	if err != nil {
		t.Fatalf("Mtime: %v", err)
	}
	if !got.Equal(ts) {
		t.Errorf("Mtime=%v, want %v", got, ts)
	}
}

func TestMtimeOnMissingFile(t *testing.T) {
	loader, _ := NewLoader(completeFS())
	if _, err := loader.Mtime("docs/missing.md"); err == nil {
		t.Fatal("Mtime on missing file returned no error")
	}
}

func TestVersionParsesLatestStable(t *testing.T) {
	loader, _ := NewLoader(completeFS())
	v, err := loader.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "v1.0.1" {
		t.Errorf("Version=%q, want v1.0.1", v)
	}
}

func TestVersionRejectsPreReleaseHeading(t *testing.T) {
	// First H2 heading is a pre-release; the regex requires the
	// canonical X.Y.Z form, so the parser must skip and find the next
	// stable heading underneath.
	fs := completeFS()
	fs["changelog.md"] = &fstest.MapFile{
		Data: []byte("# Changelog\n\n## [1.1.0-rc1] - 2026-06-01\n\nrc.\n\n## [1.0.1] - 2026-05-08\n\nstable.\n"),
	}
	loader, _ := NewLoader(fs)
	v, err := loader.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "v1.0.1" {
		t.Errorf("Version=%q, want v1.0.1 (pre-release heading must be skipped)", v)
	}
}

func TestVersionEmptyChangelog(t *testing.T) {
	fs := completeFS()
	fs["changelog.md"] = &fstest.MapFile{Data: []byte("# Changelog\n\nNothing released yet.\n")}
	loader, _ := NewLoader(fs)
	if _, err := loader.Version(); err == nil {
		t.Fatal("Version on changelog with no version heading returned no error")
	}
}

func TestFilesListsCorpus(t *testing.T) {
	loader, _ := NewLoader(completeFS())
	files, err := loader.Files()
	if err != nil {
		t.Fatalf("Files: %v", err)
	}
	if len(files) < len(requiredFiles) {
		t.Errorf("Files returned %d entries, want >= %d", len(files), len(requiredFiles))
	}
	// Verify a known path is present.
	want := "docs/routing.md"
	found := false
	for _, p := range files {
		if p == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Files did not list %q", want)
	}
}
