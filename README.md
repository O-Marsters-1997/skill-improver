# skill-review

A local web UI for reviewing a `SKILL.md`: highlight a passage, leave a comment with a
priority/category/impact, and hand the whole set of comments off to the `skill-updater`
skill in one click. Comments are written straight into the `SKILL.md` as you make them, so
there is no in-memory review state to lose — the file on disk is the only state.

See [`docs/CONTEXT.md`](docs/CONTEXT.md) for the vocabulary (anchor, thread, quote, rev) and
the reasoning behind the design; [`docs/adr/0001-goldmark.md`](docs/adr/0001-goldmark.md) for
why rendering happens server-side.

## Install

```
git clone https://github.com/O-Marsters-1997/improve-skills
cd improve-skills
just build          # -> bin/skill-review
just install        # or, onto your $GOPATH/bin
```

Requires Go 1.26.5+ (per `go.mod`) and [`just`](https://just.systems) (`brew install just`).
Once pushed to GitHub, `go install
github.com/O-Marsters-1997/improve-skills/cmd/skill-review@latest` works without cloning.

## Quickstart

```
just run
```

```
reviewing /path/to/testdata/example-SKILL.md
serving   http://localhost:8420
```

That serves the bundled fixture. To review a real skill — flags and the path both go through
`just run`:

```
just run ~/.claude/skills/my-skill/SKILL.md
just run --addr :9000 ~/.claude/skills/my-skill/SKILL.md
```

Open `http://localhost:8420` and try the workflow below against the fixture — it's a
`SKILL.md` with headings, a fenced code block, a table and a blockquote, built specifically
to exercise the anchoring.

`just --list` shows every recipe.

## Using the UI

1. **Select text** in the rendered document. Releasing the mouse (or Shift+arrow keys) opens
   a comment composer anchored to that passage.
2. **Comment** — write the body, pick a **priority** (`high`/`medium`/`low`) and **category**
   (`instructions`/`tools`/`examples`/`error_handling`/`structure`/`references`), and
   optionally an **expected impact**. Save writes the comment into the file immediately.
3. Each comment becomes a **thread** in the sidebar, anchored to its highlighted passage in
   the document. From a thread card you can:
   - **Reply** — add another comment to the same thread.
   - **Resolve / Reopen** — toggle status; resolved threads are excluded from the handoff.
   - **Delete** — remove the whole thread and its anchor markers.
   - Change **Priority**/**Category** inline at any time.
4. **Submit all to skill-updater** (top right) writes every open thread with at least one
   live comment to a handoff JSON file and copies a prompt to the clipboard. Nothing is
   pushed anywhere else, so clicking it twice is harmless.

If the file changes on disk while you're reviewing (e.g. someone edits it in another editor),
the next write is refused with a conflict and the page reloads — no changes are lost, you
just retry.

## Flags

<!-- AUTO-GENERATED: from cmd/skill-review/main.go flag definitions -->

```
usage: skill-review [flags] <path-to-SKILL.md>
```

Pass them through `just run`, or straight to the binary from `just build`.

| Flag       | Default                                  | Description                        |
| ---------- | ----------------------------------------- | ----------------------------------- |
| `-addr`    | `:8420`                                   | address to serve on                 |
| `-out`     | `~/.claude/skill-review`                  | directory for handoff payloads      |
| `-author`  | `$USER` env var, or `reviewer` if unset   | name recorded against comments      |

<!-- END AUTO-GENERATED -->

## Where things are stored

Comments live **inline in the `SKILL.md` itself**, so they survive edits and travel with the
file in version control:

- An anchored passage is wrapped in a marker pair: `<!--mc:a:ID-->the passage<!--mc:/a:ID-->`.
- Every thread (id, quote, status, comments, priority, category, impact) is one
  `<!--mc:t {JSON}-->` line, inside a `<!--mc:threads:begin-->` / `<!--mc:threads:end-->`
  block appended at the end of the file.

This is the `mc` marker format (see `internal/comments/comments.go`); other tooling that
understands it can read the same file.

Handoff payloads are written to `-out` (`~/.claude/skill-review` by default) as
`handoff-<skill-name>-<UTC timestamp>.json`, one file per Submit click.

## The handoff step

Clicking **Submit all to skill-updater** builds a payload from every open thread that still
has at least one comment: `skill_name`, `skill_path`, and an `improvement_suggestions` list,
each with `priority`, `category`, `suggestion` (the comment thread text, with the anchored
quote appended) and `expected_impact`, sorted high → medium → low. It's written to a file in
`-out` and the response also gives you the exact line to paste to Claude:

```
Use skill-updater with the payload in /path/to/handoff-<skill>-<timestamp>.json
```

That line is copied to your clipboard automatically; paste it to Claude to run
`skill-updater` against the reviewed suggestions.

## Development

| Recipe | What it does |
| ------ | ------------ |
| `just run [args]` | Serve a `SKILL.md` (defaults to the fixture) |
| `just build` | Build to `bin/skill-review` |
| `just install` | `go install` the command |
| `just test [args]` | Run all tests |
| `just fuzz [time]` | Run `FuzzOffsets` (default 30s) |
| `just fmt` | `gofmt -w` over `cmd` and `internal` |
| `just vet` | `go vet ./...` |
| `just check` | vet + test + fail on unformatted files |
| `just clean` | Remove `bin/` |

`internal/comments`, `internal/render` and `internal/handoff` are pure and table-tested;
`internal/render` additionally has `FuzzOffsets`, which asserts the property the whole tool
rests on — every rendered span's text is exactly the source bytes at the offset it
advertises. `internal/server` wires the three together over HTTP, read-modify-writing the
file on every mutation.
