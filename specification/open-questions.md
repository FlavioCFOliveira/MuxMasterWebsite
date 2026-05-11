---
title: Open questions
purpose: Register every TBD item that the specification carries today, with the reason it is open, the impact it has, and the owner responsible for closing it.
owners: specification-manager (registry); the named owner per question (resolution).
last-updated: 2026-05-08
status: live (re-read every session)
---

# Open questions

This register lists every TBD the specification has carried. Items still open are marked as such; items that have been ratified are kept visible with a `RESOLVED` status, the resolution date, and the ratified value, so the resolution history is preserved. The specification's other files refer back to this register where they encounter a `TBD`. Implementation MUST NOT silently invent values for any item still marked open; resolution requires explicit ratification.

**Status summary as of 2026-05-11:** seven items registered; three resolved (items 2, 3, 4); one superseded (item 6, by the 2026-05-08 content-pivot ratification); three still open (items 1, 5, 7).

## 1. Canonical production domain

- **Question.** Which domain will host the public production site?
- **Why open.** A domain has not been selected and registered.
- **Blocks.**
  - Public production launch (gate stated in `deployment.md`).
  - `<link rel="canonical">` absolute URLs.
  - Open Graph `og:url` and absolute `og:image` URLs.
  - Twitter Card image URL.
  - `sitemap.xml` (`<loc>` entries).
  - `/llms.txt` and `/llms-full.txt` (link prefixes).
  - JSON-LD `@id` URIs and `WebSite.url`.
  - HSTS preload submission (the domain must exist first).
- **Owner.** Project owner (Flavio CF Oliveira).
- **Resolution path.** Owner selects a domain, registers it, configures DNS to point at the production deployment, and ratifies the value as the `SITE_BASE_URL` for `ENV=production`. The `specification-manager` then records the chosen domain in `deployment.md` and removes this entry.

## 2. Exact accent colour hexes

- **Status.** RESOLVED on 2026-05-11.
- **Ratified values.** Tailwind's stock cyan and yellow scales:
  - Cyan on light surfaces (`#0e7490`, `cyan-700`) — 5.36 : 1 on `#ffffff`, WCAG AA.
  - Cyan on dark surfaces (`#67e8f9`, `cyan-300`) — 13.7 : 1 on `#09090b`, WCAG AAA.
  - Yellow badge background (`#fef08a`, `yellow-200`) — 15.1 : 1 with `#18181b` text, WCAG AAA.
  - Yellow as text on light surfaces (`#854d0e`, `yellow-800`) — 5.84 : 1 on `#ffffff`, WCAG AA.
- **Question.** What were the exact hex values of the cyan and yellow accents to lock as design tokens?
- **Why was open.** The hexes had not been measured from the source PNG; the briefing described them only approximately.
- **Blocks (now cleared).** Tailwind v4 design tokens, accessibility verification (all four pairs measured and recorded above), and the Open Graph image (which uses Tailwind's stock palette consistently).
- **Owner.** `tailwind-specialist` proposed the ratification during the 2026-05-11 production-readiness audit; the project owner ratified the proposal.
- **Resolution path.** The four ratified pairs are recorded in `brand-and-visual.md § Accents` as the authoritative reference. Templates reference them via Tailwind utilities; the unused `--color-accent-*` tokens previously in `static/css/app.css @theme` are removed.

## 3. Go module path of this repository

- **Status.** RESOLVED on 2026-05-08.
- **Ratified value.** `github.com/FlavioCFOliveira/MuxMasterWebsite` (matches the GitHub repository path; capitalisation preserved).
- **Question.** What is the Go module path declared in this repository's `go.mod`?
- **Why was open.** The repository does not yet contain a `go.mod`. The natural choice was `github.com/FlavioCFOliveira/MuxMasterWebsite`, and the project owner ratified that value on 2026-05-08.
- **Blocks (now cleared).**
  - Repository bootstrap (`go mod init`).
  - Internal import paths.
- **Owner.** Project owner.
- **Resolution path.** Owner ratified the module path. `specification-manager` recorded it in this entry; the module path itself is repository configuration (declared in `go.mod`), not a specification artefact carried in another spec file.

## 4. Binary name

- **Status.** RESOLVED on 2026-05-08.
- **Ratified value.** `muxmaster-website` (kebab-case, all lower-case).
- **Question.** What is the name of the compiled site server binary?
- **Why was open.** No name had been chosen. Options considered included `muxmaster-site`, `muxmaster-website`, `mmsite`, and `site`. The project owner ratified `muxmaster-website` on 2026-05-08.
- **Blocks (now cleared).**
  - The Dockerfile (target binary name).
  - Operational documentation.
- **Owner.** Project owner.
- **Resolution path.** Owner ratified the name. `specification-manager` recorded it in `deployment.md` and in this entry.

## 5. Landing page copy

- **Question.** What is the exact wording of:
  1. The landing page headline (value proposition).
  2. The landing page subhead.
  3. The primary CTA label (current placeholder concept: "Get started").
  4. The secondary CTA label (current placeholder concept: "Read the API").
  5. The three-to-five highlight blocks (titles and one-liners).
- **Why open.** The briefing names the structure but not the words.
- **Blocks.**
  - Implementation of `/`.
  - The Open Graph image (which embeds a tagline derived from the landing copy).
  - The blurb in `/llms.txt`.
- **Owner.** Project owner, with `ux-specialist` and `geo-specialist` review for tone and citability.
- **Resolution path.** Owner drafts the copy. `ux-specialist` and `geo-specialist` review. After ratification, the copy is stored at `/content/site/landing.md` and the Open Graph image is regenerated.

## 6. Category vs. source mapping for `/docs/`, `/examples/`, `/benchmarks`

- **Status.** SUPERSEDED on 2026-05-08 by the content-pivot ratification (recorded the same day; see the change log of `overview.md`, `content-sources.md`, and `rendering-and-caching.md`).
- **Supersession note.** The Category A vs Category B distinction this question depended on no longer exists. After the 2026-05-08 pivot, every public route is pre-rendered at startup from `/content/`; there is no lazy-cache live templating, and no route reads `${MUXMASTER_SOURCE_DIR}` at runtime. The earlier resolution of this item is therefore obsolete; the current contract is recorded in `content-sources.md` (route → local-file mapping) and `rendering-and-caching.md` (single rendering category).
- **Original ratified value (now obsolete, kept for history).**
  - `/docs/` was Category A, generated at startup from the registered route table.
  - `/examples/` was Category A, generated at startup from the registered route table.
  - `/benchmarks` was Category B, derived live from `${MUXMASTER_SOURCE_DIR}/README.md` under an extraction contract.
- **Owner.** Project owner.
- **Resolution path (closed).** No further action required; the pivot replaces the entire framing of the question.

## 7. Format and structure of the `content-curator` agent's diff output

- **Question.** When the `content-curator` agent runs a sync (see `content-sources.md` "Sync workflow"), in what shape MUST it present its proposed changes for the project owner and the gatekeepers to review? Specifically:
  1. Is the diff a unified-diff patch, a git working-tree state, a per-file before/after listing, or a structured summary that pairs each change with the upstream commit / file / section it derives from?
  2. Does the agent include a per-file rationale (for example, "renamed because upstream renamed")?
  3. How are inconsistencies the curator cannot resolve surfaced — inline in the diff, in a separate block, or as a refusal to produce the diff at all?
  4. Does the curator stage the changes in the working tree (so the project owner reviews via `git diff`), or does it present the diff in chat without writing to disk first?
- **Why open.** The pivot ratified on 2026-05-08 introduced the curator and described its role, but did not fix the review surface the curator presents to the human reviewer. Without a fixed review shape, two consecutive syncs may be reviewed differently, undermining repeatability.
- **Blocks.**
  - Authoring of the `.claude/agents/content-curator.md` agent definition (a separate task; outside the scope of `/specification/`).
  - First sync run.
- **Owner.** Project owner, with input from the four gatekeeper agents on which review shape best fits each agent's working method.
- **Resolution path.** Project owner proposes the review shape (or ratifies a recommendation from the curator-definition author). `specification-manager` records the ratified shape in `content-sources.md` "Sync workflow" and removes this entry.

## Process for closing an open question

1. The named owner produces a proposal with rationale.
2. The relevant gatekeeper agents review.
3. The project owner ratifies (or rejects with revisions).
4. `specification-manager` updates the affected specification file(s) and removes the entry from this register, in the same change.
5. Implementation MAY proceed on the affected area only after step 4 is complete.
