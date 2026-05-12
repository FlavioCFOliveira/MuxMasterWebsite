// Package content reads the editorial Markdown corpus committed under
// /content/ in this repository.
//
// The runtime binary never reads the upstream MuxMaster source tree. Every
// content file is mirrored into /content/ at development time by the
// content-curator agent and embedded into the binary at build time
// (specification/content-sources.md, specification/deployment.md). The
// Loader exposes the embedded tree behind a small fs.FS-backed surface so
// that recipes can pull bodies, parse the version label, and enumerate the
// route-to-file mapping deterministically.
package content

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"time"
)

// Loader is the read-only handle the renderer uses to fetch Markdown bodies
// from the embedded /content/ tree. It is safe for concurrent use; the
// underlying fs.FS is read-only.
type Loader struct {
	root fs.FS
}

// NewLoader wraps an fs.FS rooted at /content/ (typically an embed.FS via
// fs.Sub). The supplied FS MUST contain every required file listed in
// specification/content-sources.md "Required files"; Verify checks this.
func NewLoader(root fs.FS) (*Loader, error) {
	if root == nil {
		return nil, errors.New("content: nil fs.FS")
	}
	return &Loader{root: root}, nil
}

// Load reads the Markdown bytes at the supplied repository-relative path
// (e.g. "docs/routing.md", "api.md"). The leading slash is rejected because
// fs.FS uses unrooted paths.
func (l *Loader) Load(path string) ([]byte, error) {
	if path == "" || path[0] == '/' {
		return nil, fmt.Errorf("content: invalid path %q (must be relative, no leading slash)", path)
	}
	b, err := fs.ReadFile(l.root, path)
	if err != nil {
		return nil, fmt.Errorf("content: read %s: %w", path, err)
	}
	return b, nil
}

// Mtime returns the modification time of the file at path. For embed.FS,
// fs.Stat returns a zero time because an embedded file has no on-disk
// timestamp; callers MUST treat a zero time as "use a fallback" (typically
// the process build time). Returns an error if the path does not exist.
func (l *Loader) Mtime(path string) (time.Time, error) {
	if path == "" || path[0] == '/' {
		return time.Time{}, fmt.Errorf("content: invalid path %q", path)
	}
	info, err := fs.Stat(l.root, path)
	if err != nil {
		return time.Time{}, fmt.Errorf("content: stat %s: %w", path, err)
	}
	return info.ModTime(), nil
}

// Exists reports whether the supplied content path is present. Optional
// site-original files (site/landing.md, site/docs-index.md, etc.) use this
// before deciding whether to fold them into a recipe's output.
func (l *Loader) Exists(path string) bool {
	if path == "" || path[0] == '/' {
		return false
	}
	if _, err := fs.Stat(l.root, path); err != nil {
		return false
	}
	return true
}

// requiredFiles is the day-one minimum from specification/content-sources.md.
// Verify exits the process if any of these is absent.
var requiredFiles = []string{
	"api.md",
	"changelog.md",
	"compatibility.md",
	"security.md",
	"contributing.md",
	"benchmarks.md",
	"docs/getting-started.md",
	"docs/routing.md",
	"docs/groups.md",
	"docs/middleware.md",
	"docs/error-handling.md",
	"docs/configuration.md",
	"docs/response-helpers.md",
	"docs/performance.md",
	"docs/max-performance.md",
	"docs/observability.md",
	"docs/migration.md",
	"docs/cookbook.md",
	"examples/rest-api.md",
	"examples/authn.md",
	"examples/jwt.md",
	"examples/oauth2.md",
	"examples/cache.md",
	"examples/graceful-shutdown.md",
	"examples/server-side-render.md",
	"examples/static-site.md",
	"examples/versioning.md",
	"examples/reverse-proxy.md",
	"examples/server-sent-events.md",
	"examples/upload-file.md",
	"examples/max-performance.md",
	"release-notes/v1.0.0.md",
	"release-notes/v1.1.0.md",
}

// Verify checks every required file is present and readable. Returns a
// joined error listing every missing path.
func (l *Loader) Verify() error {
	var missing []error
	for _, rel := range requiredFiles {
		if !l.Exists(rel) {
			missing = append(missing, fmt.Errorf("missing %s", rel))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("content: required files missing under /content/: %w", errors.Join(missing...))
	}
	return nil
}

// versionRE matches `## v1.2.3` or `## [1.2.3]` at the start of a line.
// Pre-release suffixes (`-rc1`, `-beta`) are rejected per
// specification/url-and-versioning.md "Version label rule".
var versionRE = regexp.MustCompile(`^##\s+\[?v?(\d+\.\d+\.\d+)\]?(\s|$)`)

// Version reads /content/changelog.md and returns the latest stable semver
// label (the first matching heading at the top of the file). The label is
// returned with the leading `v`, e.g. `v1.0.1`.
func (l *Loader) Version() (string, error) {
	b, err := l.Load("changelog.md")
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		m := versionRE.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		return "v" + m[1], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("content: scan changelog: %w", err)
	}
	return "", errors.New("content: no stable version heading in changelog.md")
}

// Files returns every Markdown file path under the embedded tree, sorted.
// Useful for tests that assert the curator-provided corpus is intact.
func (l *Loader) Files() ([]string, error) {
	var out []string
	err := fs.WalkDir(l.root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
