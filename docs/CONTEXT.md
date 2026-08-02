# Context

`skill-review` renders a Markdown document as a page you can highlight and comment on,
then hands the comments to a skill that applies them — for the author, `skill-updater`,
which edits the skill in its source repo and opens a draft PR. The document is usually the
skill's own SKILL.md, but it does not have to be: point `--skill` somewhere else and the
comments become suggestions about the skill that produced what you are reading.

What the review asks for and who receives it are configured, not compiled in: the triage
fields and the updater skill come from a TOML file, and the built-in defaults are what the
tool hardcoded before it had one.

```
skill-review path/to/SKILL.md          # serves http://localhost:8420
skill-review --skill ~/.claude/skills/ideate report.md
                                       # review the output, edit the skill
skill-review config init               # write the defaults out to edit
go test ./...
```

## Why it exists

Reviewing a skill used to mean reading a long file, keeping notes by hand, and writing one
big prose comment back into chat. The VS Code extensions that do this were tried and found
janky in three specific ways: the editing feel, the handoff to Claude, and — worst —
submitting sometimes silently did nothing.

The third is the one the architecture answers. Those extensions **push** text into a
terminal or the clipboard, where it can drop with no trace. Here, every comment is written
into the SKILL.md the moment it is made, and Submit is a pure function of what is already
on disk. A failed submit loses nothing; click it again.

Because the comments live in the file, they can also be collected without this tool at all.
`skill-review handoff path/to/SKILL.md` prints the payload and prompt with no server
involved, and the `/skill-comments` slash command is a thin wrapper over it. That path runs
the same `handoff.Submit` the button does rather than reimplementing it, so a broken button
is never a dead end and the two can never disagree about the schema.

Two non-goals, both deliberate. There is no editor — you review, Claude writes, so a
rendered view you can highlight is the whole requirement. And there is no eval harness;
evals are run separately, by choice, after a skill changes.

## Vocabulary

Terms mean exactly this throughout the code, the API and the UI.

**Target** — the file being read and commented on. The one positional argument.

**Skill** — the skill the payload edits. `--skill`, defaulting to the target. The two are
different whenever the thing under review is something the skill *produced* rather than
the skill itself, which is the review with the most to say about whether a skill works.

**Mode** — `instructions` when the target resolves inside the skill directory, `output`
when it does not. **Derived from the two paths, never declared**, because a flag would let
them disagree. Symlinks are resolved on both sides first, or every skill installed into
`~/.claude/skills` as a link would read as `output`. The mode is what the prompt changes:
in `output` mode the updater is told to infer which instruction caused each observation
and edit the `SKILL.md`, not the reviewed file.

**Anchor** — a passage of the document a thread is attached to. Written as a **marker**
pair in the source: `<!--mc:a:ID-->the passage<!--mc:/a:ID-->`. Because the markers live in
the text they move with it when the text is edited, which is why nothing here stores line
numbers.

**Thread** — one review conversation: an id, the anchored **quote**, a status of `open` or
`resolved`, and a list of comments. All threads live on one `<!--mc:t {JSON}-->` line each,
inside a block at the end of the file. The format is `mc`, borrowed from the Markdown
Collab extension so other tools that understand it can read our files; the implementation
here is our own.

**Quote** — the source text the markers wrap. Not the same as what the browser reported the
reviewer selected: a selection covering `the **lazy** fix` reads as "the lazy fix" in the
DOM. The browser's version is a hint used to correct small offset drift; the source slice is
the truth.

**Run** — one stretch of source text as the renderer emitted it, carrying its starting byte
offset in `data-o`. Offsets are **bytes**, not characters — the browser converts through
`TextEncoder` before sending them.

**Rev** — a stamp of the file's modification time and size. The page sends back the rev it
last read; a mismatch means the file changed underneath it, so the write is refused with
409 and the page reloads. This is what stops an edit made in the editor from being clobbered.

**Field** — one unit of triage: a name, a label, a list of values and a default, declared in
the config. Each one is a control on every thread card and a key on every suggestion, so the
config is the single description of both. Fields are stored **flat** on the thread rather
than nested under a key of their own, which is what lets a file written under a different
schema still parse. A value whose field has since been removed is kept in the file and
simply not offered. The defaults are `priority` (high/medium/low) and `category`
(instructions, tools, examples, error_handling, structure, references).

**Suggestion** and **payload** — the updater's vocabulary, not ours. A thread becomes one
suggestion carrying the thread's `id`, the absolute `file` it was anchored in, one key per
field, the suggestion text and an `expected_impact`; the payload is the set of them plus
the skill's name, path and mode, ordered by the first field in the order its values are
listed. The name comes from `<skill>/SKILL.md`, never from the reviewed bytes. The
composer asks for none of it: fields are set on the thread cards, where every other thread
is in view, because ranking one comment against nothing is not a judgement anyone can
make.

**Pending** and **archive** — `.skill-review/pending.json` holds every open thread that has
not yet been handed off. It is regenerated on each Submit, so a retriage, a reply or a
deletion lands in it. The prompt carries a `mv` that moves it to
`handoff-<skill>-<timestamp>.json`; from then on those thread ids are **archived**, and
Submit excludes them for good. That boundary is the whole point — without it a second Submit
re-proposes work `skill-updater` has already applied. A reply to an archived thread stays a
local record and is never handed off again.

## Shape

| Package | Holds |
| --- | --- |
| `internal/comments` | the mc format — parse, anchor, upsert, remove |
| `internal/render` | Markdown → HTML with byte offsets stamped on every run |
| `internal/config` | the TOML file — fields, updater, defaults, validation |
| `internal/skill` | the `name:` in a SKILL.md's frontmatter, and directory → SKILL.md |
| `internal/handoff` | threads → payload (`Build`), and payload → disk (`Submit`) |
| `internal/server` | the routes, the file lock, the embedded page |

`comments`, `render`, `config` and `handoff.Build` are pure and table-tested; that is the
reason this is Go rather than another TypeScript extension. `handoff.Submit` is the one
exception, and it is deliberate: the browser and the `handoff` subcommand share it so that
neither can produce a payload the other would not. Anchoring is the part that goes wrong in every tool of this kind, so it
carries a fuzz test asserting one property: every rendered run's text is exactly the source
bytes at the offset it advertises.
