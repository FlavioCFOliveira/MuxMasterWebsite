---
title: URLs and versioning
purpose: Define URL conventions, redirects, reserved paths, and the version-label rule.
owners: ux-specialist (URL shape); seo-specialist (canonical and redirect alignment).
last-updated: 2026-09-26
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

## Routes added for MuxMaster v1.3.0

- **URL-QUERY-1.** The HTTP QUERY method page MUST be served at `/docs/http-query-method` (HTML) and `/docs/http-query-method.md` (Markdown companion). Its placement and content are defined in `information-architecture.md` § HTTP QUERY method page.
- **URL-REL-1.** Release notes MUST be served at `/releases/v<MAJOR>.<MINOR>.<PATCH>`, each with a `.md` companion. The site MUST serve `/releases/v1.2.0` and `/releases/v1.3.0`, from the sources defined in `content-sources.md` CS-REL-1, in addition to the existing `/releases/v1.0.0` and `/releases/v1.1.0`.

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
- `/static/...` — versioned static assets (CSS, favicons, OG image, logo).
- `/favicon.ico` — served as a `301 Moved Permanently` to `/static/favicon/favicon-32.png` with `Cache-Control: public, max-age=86400`. The legacy `.ico` path is reserved so that browsers, RSS readers, bookmark engines, and aggregators that probe it by reflex receive a single small redirect instead of a 404 body. The redirect is path-only; the host header is never echoed back to the client, mirroring the defensive pattern of `normalisationRedirects` in `redirects.go`.
- `/.well-known/security.txt` — RFC 9116 vulnerability-reporting contact, served as `text/plain; charset=utf-8` with `Cache-Control: public, max-age=86400`. Required fields per RFC 9116 §2.5: `Contact` (URL of the project's security advisory channel), `Expires` (no more than twelve months ahead of the deploy date), `Preferred-Languages: en`, `Canonical: https://muxmaster.net/.well-known/security.txt`. The `Expires` value MUST be refreshed before it lapses; this is part of the release contract and is verified by the release smoke-test.

## Version label rule

- The site reads the latest released version from `/content/changelog.md` at server startup. The `content-curator` agent commits this file mirrored from `../MuxMaster/CHANGELOG.md` during a sync (see `content-sources.md`).
- Detection rule: the **first** Markdown heading of the form `## v<MAJOR>.<MINOR>.<PATCH>` or `## [<MAJOR>.<MINOR>.<PATCH>]` (the Keep a Changelog form upstream uses, for example `## [1.3.0] - 2026-09-26`), with no pre-release suffix such as `-rc1`, `-beta`, `-alpha`, at the top of the changelog file. The label is rendered with a leading `v`.
- The version label is rendered as plain text in the header next to the navigation and in the footer.
- A restart is required for the label to roll forward.
- **URL-VER-1.** After the v1.3.0 content sync, the label MUST read **v1.3.0**. The same value MUST appear wherever the site states the documented MuxMaster version (page chrome, `SoftwareSourceCode.version`, `APIReference.assemblyVersion`, `/llms.txt`, `/llms-full.txt`), and every statement of the minimum Go version MUST read **Go 1.27.1** (from upstream `go.mod` at `v1.3.0`).

## URL versioning policy

- The site does **not** prefix URLs with a version (no `/v1/...`).
- This policy is revisited when MuxMaster v2 ships. The decision MUST be re-ratified at that point. Possible options to be considered then: archive subdomain, version prefix, content-negotiated versions. No option is selected today.
- Until v2 ships, every documentation URL describes the latest released version of MuxMaster.

## External links

- Links to the upstream repository, GitHub releases, or third-party sites MUST open in a new browsing context (`target="_blank"`) and MUST set `rel="noopener"`. They MUST NOT use `rel="noreferrer"` unless privacy considerations require it on a specific link.
- **URL-EXT-1.** A link that cites the source of a fact tied to a release — a benchmark figure, a quoted code excerpt, an example's `## Upstream source`, a `## Sources` entry, release notes, or a `Dataset.distribution` URL — MUST point at the release tag (for example `https://github.com/FlavioCFOliveira/MuxMaster/blob/v1.3.0/README.md`), because a tag does not move and the cited content stays verifiable. A link into this repository's campaign archive MUST point at the full commit SHA that contains it.
- **URL-EXT-2.** Any other link to upstream files (for example, a general "view the repository on GitHub" link) MUST point to the `main` branch on `github.com/FlavioCFOliveira/MuxMaster`.

## Trailing-slash and case enforcement

- The MuxMaster fields `RedirectTrailingSlash` and `RedirectFixedPath` MAY be enabled to handle these normalisations natively, provided the redirect codes match the table above (`301`).
- `CaseInsensitive` MUST be **off** on the public router; URL case is part of the canonical URL contract.
