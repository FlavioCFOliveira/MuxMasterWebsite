// Package content reads facts and bodies from the upstream MuxMaster source tree.
package content

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// versionRE matches "## v1.2.3" or "## [1.2.3]" — the changelog uses both
// shapes across MuxMaster history. Pre-release suffixes (-rc1, -beta) are
// rejected per specification/url-and-versioning.md "Version label rule".
var versionRE = regexp.MustCompile(`^##\s+\[?v?(\d+\.\d+\.\d+)\]?(\s|$)`)

// ReadLatestVersion parses the upstream CHANGELOG.md and returns the first
// stable semantic version it finds. Returns an error if no stable version
// heading is present.
func ReadLatestVersion(sourceDir string) (string, error) {
	path := filepath.Join(sourceDir, "CHANGELOG.md")
	f, err := os.Open(path) //nolint:gosec // path is built from a trusted config value.
	if err != nil {
		return "", fmt.Errorf("content: open changelog: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		m := versionRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		return "v" + m[1], nil
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("content: scan changelog: %w", err)
	}
	return "", fmt.Errorf("content: no stable version heading in %s", path)
}
