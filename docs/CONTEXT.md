# Context

`skill-review` renders a **target** — a file or a directory — as a page you can highlight
and comment on, then hands the comments to a skill that applies them. For the author that
is `skill-updater`, which edits the skill in its source repo and opens a draft PR. The
target is not necessarily the skill: you can review a document a skill *produced* and have
the comments land on the skill that produced it.

What the review asks for and who receives it are configured, not compiled in: the triage
fields and the updater skill come from a TOML file, and the built-in defaults are what the
tool hardcoded before it had one.

```
skill-review [--skill <path>] <target>     # serves http://127.0.0.1:8420
skill-review config init                   # write the defaults out to edit
go test ./...
```

## Why it exists

Reviewing a skill used to mean reading a long file, keeping notes by hand, and writing one
big prose comment back into chat. The VS Code extensions that do this were tried and found
janky in three specific ways: the editing feel, the handoff to Claude, and — worst —
submitting sometimes silently did nothing.

The third is the one the architecture answers. Those extensions **push** text into a
terminal or the clipboard, where it can drop with no trace. Here, every comment is written
into the reviewed file the moment it is made, and Submit is a pure function of what is
already on disk. A failed submit loses nothing; click it again.

Because the comments live in the file, they can also be collected without this tool at all.
`skill-review handoff <target>` prints the payload and prompt with no server involved, and
the `/skill-comments` slash command is a thin wrapper over it. That path runs the same
`handoff.Submit` the button does rather than reimplementing it, so a broken button is never
a dead end and the two can never disagree about the schema.

Two non-goals, both deliberate, and the second is narrower than it first sounds. There is
no editor — you review, Claude writes, so a rendered view you can highlight is the whole
requirement. And there is no eval **harness**: no test sets, no judges, no pass rates, no
scores. That is not the same as no *evaluation*. Reviewing a document a skill produced is
evaluation, and it is squarely in scope — what stays out is the machinery, not the
judgement.

One smaller choice is worth recording here rather than in an ADR. Listing the skills on
this machine is a native Go glob over the skill directories, not a shell-out to
`npx skills`. That was tried and rejected on evidence: it was not installed, it reported no
plugin skills at all, and it carries no descriptions to show. Reversing the decision is a
few lines if it ever grows them, which is why this is a note.

## Vocabulary

Terms mean exactly this throughout the code, the API and the UI.

**Target** — the file or directory being read and commented on. A directory target is the
set of reviewable files under it; a file target is a set of one, so there is only ever one
case to reason about.

**Skill** — the skill the payload is aimed at, and so the thing the updater ends up editing.
It defaults to the target, and the two are the same whenever you are reviewing a skill's own
instructions. They part company when the target is something the skill produced.

**Mode** — `instructions` when the reviewed file **resolves** inside the skill directory,
`output` when it does not. **Derived from the two paths, never declared.** Resolved, not
merely prefixed: the skill directories are largely a symlink farm, so a plain path comparison
would call every real skill `output`. There is no flag for it either, because a flag is a
second source of truth and the two would eventually disagree.

**Anchor** — a passage of the document a thread is attached to. Written as a **marker**
pair in the source: `<!--mc:a:ID-->the passage<!--mc:/a:ID-->`. Because the markers live in
the text they move with it when the text is edited, which is why nothing here stores line
numbers.

**Thread** — one review conversation: an id, the anchored **quote**, a status of `open` or
`resolved`, and a list of comments. All threads live on one `<!--mc:t {JSON}-->` line each,
inside a block at the end of the file. The format is `mc`, borrowed from the Markdown
Collab extension so other tools that understand it can read our files; the implementation
here is our own. A thread never records which file it belongs to: it lives in that file, so
file identity is implicit in where the markers are and cannot drift away from them.

**Quote** — the source text the markers wrap. Not the same as what the browser reported the
reviewer selected: a selection covering `the **lazy** fix` reads as "the lazy fix" in the
DOM. The browser's version is a hint used to correct small offset drift; the source slice is
the truth.

**Run** — one stretch of source text as the renderer emitted it, carrying its starting byte
offset in `data-o`. Offsets are **bytes**, not characters — the browser converts through
`TextEncoder` before sending them. Two renderers emit runs, one for Markdown targets and one
for HTML, and the word means the same in both: they are held to a single invariant, that a
run's text is exactly the source bytes at the offset it advertises.

**Format** — Markdown or HTML, decided once from the target's extension. It picks the
renderer *and* the anchoring rules, because a code fence is Markdown's idea and backticks in
an HTML file are ordinary text. An HTML target is rendered by tokenising and sanitised
against an allowlist; see [ADR-0004](adr/0004-html-rendering.md).

**Rev** — a stamp of the file's modification time and size. The page sends back the rev it
last read; a mismatch means the file changed underneath it, so the write is refused with
409 and the page reloads. This is what stops an edit made in the editor from being clobbered.
A rev is **per file**, not per review: a review spanning several files carries one for each,
so an outside edit to one of them invalidates that file alone and the rest keep working.

**Field** — one unit of triage: a name, a label, a list of values and a default, declared in
the config. Each one is a control on every thread card and a key on every suggestion, so the
config is the single description of both. Fields are stored **flat** on the thread rather
than nested under a key of their own, which is what lets a file written under a different
schema still parse. A value whose field has since been removed is kept in the file and
simply not offered. The defaults are `priority` (high/medium/low), `category`
(instructions, tools, examples, error_handling, structure, references) and `cause`
(instructions/execution) — the last of which separates "the instruction is wrong" from "the
model slipped this once", so a fluke is not baked into a permanent edit.

**Suggestion** and **payload** — the updater's vocabulary, not ours. A thread becomes one
suggestion carrying the thread's `id`, the **absolute** `file` it was anchored in, one key
per field, the suggestion text and an `expected_impact`; the payload is the set of them plus
the skill's name and path and the **mode**, ordered by the first field in the order its
values are listed. The `file` is per suggestion because one payload can span several files;
the `mode` is on the payload because it describes the whole review. The composer asks for
none of it: fields are set on the thread cards, where every other thread is in view, because
ranking one comment against nothing is not a judgement anyone can make.

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
| `internal/render` | Markdown or HTML → a page, with byte offsets on every run |
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
