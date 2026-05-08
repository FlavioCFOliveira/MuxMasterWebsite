---
title: URLs and versioning
purpose: Define URL conventions, redirects, reserved paths, and the version-label rule.
owners: ux-specialist (URL shape); seo-specialist (canonical and redirect alignment).
last-updated: 2026-05-08
status: ratified
---

# URLs and versioning

## URL conventions

- Path segments are lowercase, kebab-case (e.g. `/docs/error-handling`, not `/docs/errorHandling` or `/docs/error_handling`).
- No file extensions on HTML routes.
- The `.md` suffix is reserved for Markdown companions.
- No query strings for content. Query strings MUST NOT change which page or canonical URL is shown.
- No fragments are required for navigation; fragments (`#section-id`) are used only for in-page anchors and MUST be safe to share.
- Index URLs end with a trailing slash: `/`, `/docs/`, `/examples/`.
- Leaf URLs do not end with a trailing slash: `/docs/routing`, `/api`, `/benchmarks`.

## Redirects

The server MUST issue HTTP `301 Moved Permanently` redirects for the following normalisations:

| Request | Redirect target |
| --- | --- |
| `/index.html` | `/` |
| `/docs` (no trailing slash) | `/docs/` |
| `/docs/index.html` | `/docs/` |
| `/examples` (no trailing slash) | `/examples/` |
| `/examples/index.html` | `/examples/` |
| `/docs/<section>/` (trailing slash on a leaf) | `/docs/<section>` |
| `/examples/<name>/` (trailing slash on a leaf) | `/examples/<name>` |
| `/<path>.html` (any HTML route written with extension) | `/<path>` |
| Mixed case in the path (`/Docs/Routing`) | the lowercased equivalent (`/docs/routing`) |

Redirect targets MUST be absolute paths on the same origin. The body MAY be empty. `Cache-Control: public, max-age=300` is acceptable.

## Reserved paths

The following paths are reserved by the site and MUST NOT collide with documentation routes:

- `/healthz` — health endpoint (operational).
- `/robots.txt`, `/sitemap.xml`.
- `/llms.txt`, `/llms-full.txt`.
- `/assets/...` — versioned static assets (CSS, favicons, OG image, logo).
- `/favicon.ico` — served from a redirect to a hashed PNG/ICO under `/assets/`, or directly from a fixed location; either way the path is reserved.

## Version label rule

- The site reads the latest released version from `/content/changelog.md` at server startup. The `content-curator` agent commits this file mirrored from `../MuxMaster/CHANGELOG.md` during a sync (see `content-sources.md`).
- Detection rule: the **first** Markdown heading of the form `## v<MAJOR>.<MINOR>.<PATCH>` (no pre-release suffix such as `-rc1`, `-beta`, `-alpha`) at the top of the changelog file.
- The version label is rendered as plain text in the header next to the navigation and in the footer.
- A restart is required for the label to roll forward.
- The current value as of 2026-05-08 is **v1.0.1**.

## URL versioning policy

- The site does **not** prefix URLs with a version (no `/v1/...`).
- This policy is revisited when MuxMaster v2 ships. The decision MUST be re-ratified at that point. Possible options to be considered then: archive subdomain, version prefix, content-negotiated versions. No option is selected today.
- Until v2 ships, every documentation URL describes the latest released version of MuxMaster.

## External links

- Links to the upstream repository, GitHub releases, or third-party sites MUST open in a new browsing context (`target="_blank"`) and MUST set `rel="noopener"`. They MUST NOT use `rel="noreferrer"` unless privacy considerations require it on a specific link.
- Links to upstream files (for example, "view this example on GitHub") MUST point to the `main` branch on `github.com/FlavioCFOliveira/MuxMaster`.

## Trailing-slash and case enforcement

- The MuxMaster fields `RedirectTrailingSlash` and `RedirectFixedPath` MAY be enabled to handle these normalisations natively, provided the redirect codes match the table above (`301`).
- `CaseInsensitive` MUST be **off** on the public router; URL case is part of the canonical URL contract.
