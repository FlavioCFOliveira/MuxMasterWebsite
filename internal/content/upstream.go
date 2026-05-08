package content

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// requiredUpstreamFiles is the day-one minimum set defined in
// specification/content-sources.md. The server refuses to start if any of
// these is absent.
var requiredUpstreamFiles = []string{
	"README.md",
	"api.md",
	"CHANGELOG.md",
	"COMPATIBILITY.md",
	"SECURITY.md",
	"CONTRIBUTING.md",
	"docs/getting-started.md",
	"docs/routing.md",
	"docs/groups.md",
	"docs/middleware.md",
	"docs/error-handling.md",
	"docs/configuration.md",
	"docs/response-helpers.md",
	"docs/performance.md",
	"docs/observability.md",
	"docs/migration.md",
	"docs/cookbook.md",
	"examples/rest-api/main.go",
	"examples/authn/main.go",
	"examples/jwt/main.go",
	"examples/oauth2/main.go",
	"examples/cache/main.go",
	"examples/graceful-shutdown/main.go",
	"examples/server-side-render/main.go",
	"examples/static-site/main.go",
	"release-notes/v1.0.0-20260508.md",
	"assets/logo-muxmaster.png",
}

// VerifyUpstream checks that every required upstream file is present and
// readable. Returns a joined error listing every missing path.
func VerifyUpstream(sourceDir string) error {
	if sourceDir == "" {
		return errors.New("content: MUXMASTER_SOURCE_DIR is empty")
	}
	info, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("content: stat MUXMASTER_SOURCE_DIR %q: %w", sourceDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("content: MUXMASTER_SOURCE_DIR %q is not a directory", sourceDir)
	}

	var missing []error
	for _, rel := range requiredUpstreamFiles {
		p := filepath.Join(sourceDir, rel)
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, fmt.Errorf("missing %s", rel))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("content: upstream contract not satisfied: %w", errors.Join(missing...))
	}
	return nil
}
