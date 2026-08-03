# 6. Prose blocks are editable in place, as a byte splice on the source

Status: accepted · 2026-08-03

## Context

Reviewing had exactly one verb. Every observation, however small, became a comment, and a
comment is a round trip through another agent: it is written into the file, submitted to
`pending.json`, handed to `skill-updater`, applied, and a draft PR opened. That is the right
weight for "this instruction is wrong". It is absurd for a typo.

`docs/CONTEXT.md` recorded the opposite as a deliberate non-goal — *"There is no editor — you
review, Claude writes, so a rendered view you can highlight is the whole requirement."* That
held while the only thing a reviewer could express was a judgement. It stops holding the first
time someone spots a misspelling and has to choose between filing a suggestion about it and
leaving it there.

The machinery an editor needs already existed, built for comments: `mutate` takes the write
lock, refuses a stale `rev` with a 409, splices, saves atomically through a temp file and
rename, and re-renders. Nothing about it is specific to comments — it is a
`func(path, src) ([]byte, error)` pipeline.

## Decision

### The editable unit is a block, and an edit is a byte splice

The renderer already stamps `data-o` on every run of source text, which is what makes
anchoring exact. It now also stamps `data-os`/`data-oe` on the blocks that hold prose:
paragraphs, headings, tight list items, code-fence bodies and the frontmatter. An edit sends
`{start, end, text}` and the server replaces those bytes. There is no HTML-to-Markdown step
anywhere.

The bounds come from goldmark's line segments, which begin *after* the block's own syntax. A
heading's range covers `A heading` and not `## A heading`; a list item's covers the text and
not the `- `; a fence's covers the body and not the fences. An edit can therefore change what
a block says but never what kind of block it is.

Two alternatives were real:

**A whole-file source textarea** is far less code — no renderer change at all, one handler,
about twenty lines. It was rejected for what it is rather than what it costs: it is not
editing in place, it puts the whole document one careless keystroke from being replaced, and
it abandons the rendered view that is the entire point of the tool.

**`contentEditable` on the rendered HTML** looks like the obvious in-place editor and cannot
work here. Reading it back means converting HTML to Markdown, which cannot reproduce the
source byte for byte — the choice of `*` or `_`, the exact line wrapping, an escaped
character. The `data-o` contract and every stored anchor depend on those bytes being
preserved, so a round trip through the DOM would silently move every comment in the file.

### Editing is refused unless the file is Markdown and in instructions mode

Markdown, because the HTML renderer is a sanitising allowlist and so cannot promise a
faithful round trip.

Instructions mode is the sharper half. In output mode the reviewed document is something the
skill *produced*: editing it improves an artifact that gets thrown away while the skill that
wrote it stays exactly as wrong as it was. There, a comment is the only thing that travels,
so the edit affordance is not offered and the route refuses it. The rule reuses
`handoff.ModeOf` rather than adding a second one — `deriveMode` is now a loop over it, so the
per-file answer the gate needs and the whole-review answer the payload carries cannot drift.

The gate is enforced server-side and *also* served to the page as `editable`. The page needs
it to decide whether to show the button; the server re-checks it because this is the one
route that accepts arbitrary bytes into a file whose markers hold the review together.

### An edit may not change the document's markers

The source a reviewer sees includes any anchor markers the block carries —
`<!--mc:a:k3f-->the passage<!--mc:/a:k3f-->` — and they have to come back unchanged.
`comments.Replace` compares the marker sequence of the whole document before and after and
refuses the write otherwise.

This is a hard refusal rather than an attempt to re-anchor. `snap`'s ±64-byte window exists
to correct *drift*, where the quote is still present and only the offsets moved. After a
rewrite there is nothing to snap to: the honest answers are "guess" or "drop the thread", and
both lose a comment silently. Refusing costs the reviewer one extra step — resolve or delete
the comment, then edit — and cannot lose anything.

Comparing the whole document rather than the replaced slice is what catches an offset that
landed inside a marker, where half of it would be in the old bytes and half left behind.
Comparing the *sequence* rather than the set is what catches an anchor turned inside out by
swapping its open and close markers. The same check is the input validation for the route: a
`<!--mc:t {…}-->` line typed into the prose would otherwise forge a thread.

### Saving is explicit

`Cmd/Ctrl+Enter` or the Save button writes; Escape discards; clicking away does nothing. The
keybindings are the comment composer's, and Escape already means dismiss elsewhere in the UI.

Debounced autosave was rejected on mechanics, not taste. Every write returns the whole
re-rendered document and replaces `doc.html`, so a write mid-sentence would rebuild the
document under the cursor. The rev would churn constantly for the same reason.

## Consequences

**The rendered document is now memoised.** It had to be. React re-set `#doc`'s `innerHTML` on
every render of that element, not only when the html string changed, which destroyed the
editor's host node the moment it was inserted — the comment in `Document.tsx` asserting the
opposite was simply wrong. `RenderedDoc` is `memo`ised on `html` alone, with the click handler
moved to a wrapper outside it so events still arrive by bubbling. A side effect is that a
keystroke in the reply box no longer rebuilds the entire document.

**The draft lives in `App`, not in the editor.** A refused save redraws the document and
remounts everything under it, so a draft held inside `BlockEditor` would vanish exactly when
it matters. When the redraw takes the host block away, the editor falls back to rendering
below the document with the typing intact.

**Editing shows source, including its noise.** A multi-line list item or a paragraph inside a
blockquote carries its continuation indentation or `> ` prefixes between lines, and those are
in the slice. That is honest — the thing being spliced is source — but it is not a prose
editor and should not be mistaken for one.

**Frontmatter is editable and unvalidated.** A skill's `description` decides whether the
skill triggers at all, which makes it the highest-value thing to tune during a review. There
is no YAML dependency in the module — `skill.go` scans for `name:` by hand — so a broken
indent saves cleanly and surfaces later as a skill whose name stops resolving. The bounds
exclude the `---` delimiters, so the block cannot stop being frontmatter, but nothing checks
what is between them. Adding `gopkg.in/yaml.v3` and a parse-before-write is the upgrade if
that ever bites.

**Tables and blockquote wrappers are not editable.** Table cells need the Table extension's
node kinds plus pipe and alignment handling; a blockquote's own paragraphs are editable, which
covers the useful case.
