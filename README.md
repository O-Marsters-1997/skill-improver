# skill-review

A local web UI for reviewing a `SKILL.md`: highlight a passage, leave a comment, triage it,
and hand the whole set off to the skill that applies them in one click. Comments are written
straight into the `SKILL.md` as you make them, so there is no in-memory review state to
lose — the file on disk is the only state.

What the triage asks for and which skill receives the payload are
[configurable](#configuration); out of the box it is a priority, a category and a cause, and
the prompt spells the work out rather than naming a skill.

See [`docs/CONTEXT.md`](docs/CONTEXT.md) for the vocabulary (anchor, thread, quote, field,
rev) and the reasoning behind the design;
[`docs/adr/0001-goldmark.md`](docs/adr/0001-goldmark.md) for why rendering happens
server-side, and [`docs/adr/0002-toml-config.md`](docs/adr/0002-toml-config.md) for why the
config is TOML and why fields are stored flat.

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

1. **Select text** in the rendered document. A small **💬 Comment** action appears next to
   the selection — selecting is never hijacked, so you can read, copy and re-select freely.
   Click the action (or press Escape / click away to dismiss it) to open a composer anchored
   to that passage.
2. **Comment** — write the body and save. That's the whole form: triage is a comparative
   judgement, so it happens later, in the sidebar, once there is something to compare
   against. Save writes the comment into the file immediately.
3. Each comment becomes a **thread** in the sidebar, anchored to its highlighted passage in
   the document. From a thread card you can:
   - **Reply** — add another comment to the same thread.
   - **Resolve / Reopen** — toggle status; resolved threads are excluded from the handoff.
   - **Delete** — remove the whole thread and its anchor markers.
   - Set the **fields** — the triage step, with every other thread in view. One control per
     configured field; by default that is Priority and Category, defaulting to
     `medium`/`instructions`. See [Configuration](#configuration).
4. **Submit all** (top right, named after your updater skill when one is configured) collects
   every open thread that hasn't already
   been handed off into `.skill-review/pending.json`, and shows the prompt in a panel that
   stays until you close it — with a **Copy prompt** button, in case the automatic clipboard
   write was refused. Nothing is pushed anywhere, so clicking it twice is harmless: the
   second click reports that nothing is new. Paste the prompt to Claude, and **run the `mv`
   it gives you** once the suggestions are applied — see [The handoff step](#the-handoff-step).

If the file changes on disk while you're reviewing (e.g. someone edits it in another editor),
the next write is refused with a conflict and the page reloads — no changes are lost, you
just retry.

## Commands and flags

```
usage: skill-review [flags] <path-to-SKILL.md>
```

`serve` is the default command, so the bare form above is the same as
`skill-review serve <path>`. Flags may go before or after the command name; pass them
through `just run`, or straight to the binary from `just build`.

| Command | What it does |
| ------- | ------------ |
| `serve <path>` | Serve a `SKILL.md` for review. The default — the name is optional |
| `handoff <path>` | Print the payload and prompt for a `SKILL.md` without serving anything. The backstop when the browser is not an option |
| `config init` | Write a config file with the built-in defaults spelled out |

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `--addr` | `:8420` | address to serve on |
| `--out` | `.skill-review` | directory for handoff payloads |
| `--author` | `$USER` env var, or `reviewer` if unset | name recorded against comments |
| `--config` | *(see [Configuration](#configuration))* | config file to use |

`config init` takes three of its own:

| Flag | Description |
| ---- | ----------- |
| `--updater PATH` | absolute path to the skill that applies the suggestions. Checked before anything is written |
| `--local` | write `skill-review.toml` here instead of the user config file |
| `--force` | overwrite an existing config file |

## Configuration

Everything is optional. With no config file at all the tool behaves exactly as it did before
there was one, so `config init` writes those defaults out rather than inventing anything:

```
skill-review config init            # -> ~/.config/skill-review/config.toml
skill-review config init --local    # -> ./skill-review.toml
```

The first file found wins:

1. `--config <path>`
2. `./skill-review.toml`
3. `$XDG_CONFIG_HOME/skill-review/config.toml` (usually `~/.config/...`)
4. the built-in defaults

```toml
# Each [[field]] is one control on a thread card and one key on every suggestion in
# the handoff payload. Delete a block to drop the field; add one to introduce your
# own. Suggestions are sorted by the FIRST field, in the order its values are listed.

[[field]]
name    = "priority"   # the JSON key: lower-case letters, digits and underscores
label   = "Priority"   # what the sidebar shows; defaults to the name
values  = ["high", "medium", "low"]
default = "medium"     # must be one of values

[[field]]
name    = "category"
label   = "Category"
values  = ["instructions", "tools", "examples", "error_handling", "structure", "references"]
default = "instructions"

[[field]]
name    = "cause"
label   = "Cause"
values  = ["instructions", "execution"]
default = "instructions"

# Optional. Its name is read from the frontmatter and used in the handoff prompt and
# on the Submit button. Leave it out to get the built-in prompt.
[updater]
skill = "/absolute/path/to/skill-updater"
```

The fields the config declares are the only ones the sidebar offers and the only ones a
suggestion carries — the browser builds its controls from what the server serves, so the two
cannot drift. `id`, `quote`, `status`, `comments` and `impact` are reserved, because fields
are stored flat on the thread.

A config that will not load is fatal rather than ignored: falling back silently would hand a
reviewer the wrong controls without saying so. Errors name the file and the key, and a syntax
error points at the line.

Changing the schema does not rewrite existing files. A value for a field you have since
removed stays in the `SKILL.md` and is simply not offered or handed off; put the field back
and it reappears.

## Where things are stored

Comments live **inline in the `SKILL.md` itself**, so they survive edits and travel with the
file in version control:

- An anchored passage is wrapped in a marker pair: `<!--mc:a:ID-->the passage<!--mc:/a:ID-->`.
- Every thread (id, quote, status, comments, then its fields and impact) is one
  `<!--mc:t {JSON}-->` line, inside a `<!--mc:threads:begin-->` / `<!--mc:threads:end-->`
  block appended at the end of the file. Fields are written flat alongside the fixed keys,
  which is why a file written under a different schema still reads.

This is the `mc` marker format (see `internal/comments/comments.go`); other tooling that
understands it can read the same file.

Handoff payloads are written to `-out`. The default `.skill-review` is **relative**, so they
land in the directory you ran the binary from — next to the work, not buried in `$HOME`. Add
`.skill-review/` to your `.gitignore` if you'd rather not commit them.

Two kinds of file live there:

- **`pending.json`** — everything open and not yet handed off. There is only ever one, and
  Submit rewrites it in place.
- **`handoff-<skill-name>-<UTC timestamp>.json`** — an archive of one handoff that has been
  applied. Submit reads these to know which threads are done.

## The handoff step

Clicking **Submit all** writes `pending.json`: `skill_name`, `skill_path`, and an
`improvement_suggestions` list, each with the thread's `id`, one key per
[configured field](#configuration), `suggestion` (the comment thread text, with the anchored
quote appended) and `expected_impact`, sorted by the first field. `expected_impact` is
synthesised from the anchored quote — the UI never asks for it, because an updater derives
that kind of framing better than a text box collects it.

The prompt you get back has two parts. With an updater configured it names it:

```
Use skill-updater with the payload in /proj/.skill-review/pending.json

Once applied, archive it so these suggestions are not handed off again:
mv /proj/.skill-review/pending.json /proj/.skill-review/handoff-<skill>-<timestamp>.json
```

With none configured, the first half spells the same work out instead — apply the
suggestions in order, smallest edit that satisfies each, keep the skill's voice — and the
`mv` half is identical.

**The `mv` is the important half.** Until you run it, those threads stay pending; once you
do, their ids are archived and Submit will never hand them over again. Without that
boundary, a second review session re-proposes everything the first one already applied.

Everything else follows from it:

- Pending is **regenerated** on every Submit, not appended to, so retriaging a thread,
  replying to it, resolving it or deleting it is reflected next time you click.
- A Submit that changes nothing doesn't rewrite the file, and the panel says so.
- Replying to a thread that has already been archived keeps the reply as a local record —
  it is not handed off a second time. Start a new thread if you want it acted on.
- With every open thread archived, Submit reports nothing to hand off and removes
  `pending.json`.

The prompt is kept in three places so it can't be lost: the panel in the UI (which waits to
be dismissed), a `prompt` field in `pending.json` itself, and a line printed to the terminal
running the server. That file is therefore a superset of what an updater reads — `skill_name`,
`skill_path` and `improvement_suggestions` are still at the top level.

`skill-review handoff <path>` does all of the above without a browser, printing the payload
and the prompt to stdout. It runs the same code the button does, so a broken button is never
a dead end.

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

`internal/comments`, `internal/render` and `handoff.Build` are pure and table-tested;
`internal/render` additionally has `FuzzOffsets`, which asserts the property the whole tool
rests on — every rendered span's text is exactly the source bytes at the offset it
advertises. `internal/server` wires the three together over HTTP, read-modify-writing the
file on every mutation.
