package meta

import "testing"

func TestFullTitle(t *testing.T) {
	cases := []struct {
		name  string
		title string
		want  string
	}{
		{"landing-suppresses-suffix", "", "MuxMaster — High-performance HTTP router for Go"},
		{"regular-page-appends-suffix", "Routing", "Routing — MuxMaster"},
		{"explicit-title-with-spaces", "Getting started", "Getting started — MuxMaster"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			p := Page{Title: tc.title}
			if got := p.FullTitle(); got != tc.want {
				t.Errorf("FullTitle()=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsHome(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"/", true},
		{"/docs/", false},
		{"/api", false},
		{"", false},
	} {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			p := Page{Path: tc.path}
			if got := p.IsHome(); got != tc.want {
				t.Errorf("IsHome(%q)=%v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestSectionLabel(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/", ""},
		{"/docs/", "Documentation"},
		{"/docs/routing", "Documentation"},
		{"/api", "API"},
		{"/examples/", "Examples"},
		{"/examples/jwt", "Examples"},
		{"/benchmarks", "Benchmarks"},
		{"/changelog", "Changelog"},
		{"/releases/v1.0.0", "Releases"},
		{"/security", "Security"},
		{"/contributing", "Contributing"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			p := Page{Path: tc.path}
			if got := p.SectionLabel(); got != tc.want {
				t.Errorf("SectionLabel(%q)=%q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestJSONLDBlockShape(t *testing.T) {
	// JSONLDBlock has no methods today, but the struct is part of the
	// renderer/template contract. A trivial round-trip confirms the
	// field names compile against the rest of the project.
	b := JSONLDBlock{
		Comment: "omitted: datePublished on TechArticle — front-matter date not yet authored",
		JSON:    `{"@type":"TechArticle"}`,
	}
	if b.Comment == "" || b.JSON == "" {
		t.Fatalf("JSONLDBlock fields not retained: %+v", b)
	}
}
