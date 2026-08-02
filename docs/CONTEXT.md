# Context

`skill-review` renders a SKILL.md as a page you can highlight and comment on, then hands
the comments to the `skill-updater` skill, which edits the skill in its source repo and
opens a draft PR.

```
skill-review path/to/SKILL.md      # serves http://localhost:8420
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
The `/skill-comments` slash command reads the threads straight out of a SKILL.md and calls
`skill-updater` with the same payload — a second, independent path, so a broken button is
never a dead end.

Two non-goals, both deliberate. There is no editor — you review, Claude writes, so a
rendered view you can highlight is the whole requirement. And there is no eval harness;
evals are run separately, by choice, after a skill changes.

## Vocabulary

Terms mean exactly this throughout the code, the API and the UI.

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

**Suggestion** and **payload** — `skill-updater`'s vocabulary, not ours. A thread becomes one
suggestion with a `priority`, a `category`, the suggestion text and an `expected_impact`;
the payload is the set of them plus the skill's name and path. Priority and category are
picked in the sidebar while commenting, so the payload is complete before Submit is clicked.

## Shape

| Package | Holds |
| --- | --- |
| `internal/comments` | the mc format — parse, anchor, upsert, remove |
| `internal/render` | Markdown → HTML with byte offsets stamped on every run |
| `internal/handoff` | threads → `skill-updater` payload |
| `internal/server` | the routes, the file lock, the embedded page |

The first three are pure and table-tested; that is the reason this is Go rather than another
TypeScript extension. Anchoring is the part that goes wrong in every tool of this kind, so it
carries a fuzz test asserting one property: every rendered run's text is exactly the source
bytes at the offset it advertises.
