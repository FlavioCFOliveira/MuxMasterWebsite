// Package muxmasterwebsite holds the content corpus embedded into the
// binary at build time. The package contains no executable code; it exists
// only to anchor the //go:embed directive at the repository root, which
// places /content/ inside the binary.
//
// The runtime binary is self-contained (specification/deployment.md): no
// path outside this embed is consulted at request time, at startup, or at
// any point in the process lifetime. The content-curator agent populates
// /content/ at development time; a redeploy is the unit of update.
package muxmasterwebsite

import (
	"embed"
	"io/fs"
)

// contentFS is the embedded /content/ tree. The `all:` prefix preserves
// dotfiles (none today, but the curator may add a `.editorconfig` or
// similar without breaking the embed).
//
//go:embed all:content
var contentFS embed.FS

// ContentFS returns an fs.FS rooted at the embedded /content/ directory.
// It is the only legitimate way the runtime accesses curated bodies.
func ContentFS() (fs.FS, error) {
	return fs.Sub(contentFS, "content")
}
