# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

The author of an agent skill, reviewing their own work. They run `skill-review` from a
terminal against a skill directory, read it in a browser on localhost, and leave comments on
the passages they want changed. The session is short and single-user: one person, one
machine, no accounts, no collaborators. The repository is public and installable with
`go install`, so other skill authors are a real secondary audience — but they arrive as
individuals running the same local workflow, not as a team sharing a deployment.

## Product Purpose

Reviewing a skill is a writing problem, not a code problem, and the tools either side of it
are wrong for it: a diff view has nothing to diff, and a chat window loses which sentence you
meant. `skill-review` renders the skill's Markdown as a document, lets the reviewer select any
passage and attach a comment to it, and then hands the whole set of comments to an agent that
edits the skill. Success is that a reviewer can read a skill end to end, mark everything that
should change while reading, and hand it off in one action.

## Positioning

Comments live **inside the reviewed Markdown file**, as HTML comments anchored around the
quoted text. There is no database, no sidecar file, and no server-side state — the document is
the store. That is what makes the review survive the tool: the file can be committed, diffed,
handed to an agent, or read by a person with no `skill-review` running. The handoff is the
other half: the accumulated threads become a structured payload for a named updater agent
(`skill-updater` by default), so a review ends in an edit rather than in a list of notes.

## Operating Context

- Invoked as `skill-review [flags] <path>`, where the path is a skill directory or a single
  file. A directory is walked for reviewable files (`.md`, `.html`, `.htm`), skipping dotfiles,
  `node_modules`, and the output directory.
- Serves a local HTTP server (default `127.0.0.1:8420`) with the frontend embedded in the Go
  binary. Nothing leaves the machine.
- The reviewed file is read from and written to disk on every mutation. Each response carries a
  `rev`; a mismatch means the file changed underneath the reviewer and the UI reloads rather
  than overwriting.
- The file under review and the skill the handoff edits are deliberately two different paths
  (see `docs/adr/0003-target-is-not-the-skill.md`).
- Configuration is TOML; comment threads can carry configured fields such as an impact rating.

## Capabilities and Constraints

- **Terminology.** *Target* — what is being reviewed. *Skill* — what the handoff edits.
  *Thread* — one anchored comment plus its replies. *Handoff* — the payload written for the
  updater agent. *Review set* — the files discovered at startup; nothing outside it is
  addressable.
- Rendering is server-side (goldmark), with byte offsets emitted per text run so a browser
  selection maps back to exact source bytes. Raw HTML in the source is escaped, never executed.
- The frontend is a Vite/React SPA built by `bun` into `internal/server/web` and embedded with
  `go:embed`. A checkout that has not run `just web` serves an explicit "not built" page.
- Single-user by construction: there is no auth, no multi-writer coordination beyond the `rev`
  check, and no persistence outside the reviewed files.
- **Known defect, undecided:** a comment anchored across an entire top-level paragraph makes
  that paragraph disappear from the rendered view. `internal/render.writeMarkup` discards any
  HTML-block line beginning `<!--mc:`, and a whole-paragraph anchor produces exactly that.
  Mid-paragraph anchors are unaffected.

## Brand Commitments

Name: `skill-review`. No logo, wordmark, or brand palette exists, and none is claimed. The
warm off-white/brown palette in `web/src/index.css` is an inherited implementation choice
rather than a stated brand commitment.

## Evidence on Hand

- `testdata/example-skill/` — the fixture review set (a SKILL.md plus a `references/` folder),
  used by the Go tests and by `just run`.
- `docs/adr/` — four accepted decisions: goldmark, TOML config, target-is-not-the-skill, and
  the HTML rendering pipeline.
- `docs/CONTEXT.md` — the Go package map.
- No users, testimonials, benchmarks, adoption numbers, or deployment claims exist. Future work
  must not invent them.

## Product Principles

1. **The file is the database.** Anything the tool knows lives in the reviewed Markdown. If a
   feature needs state the file cannot hold, question the feature first.
2. **The server renders; the client displays.** Every mutation redraws from the server's
   response, so there is no client cache to invalidate and no second source of truth.
3. **A review ends in an edit.** Features are judged by whether they get a suggestion into the
   updater agent's hands, not by how well they organise notes.
4. **Reading comes first.** This is a document being read closely. Chrome, controls, and
   affordances yield to the prose.
5. **Local and disposable.** Short-lived process, one user, no network. Do not add
   accounts, sync, or storage that outlives the session.

## Accessibility & Inclusion

WCAG 2.1 AA as a floor, not a compliance programme: full keyboard operability with visible
focus, AA text contrast, and correct semantics on custom widgets. The file explorer follows the
WAI-ARIA treeview pattern (one tab stop, arrow-key navigation) specifically so keyboard users
reach the document without traversing every file.
