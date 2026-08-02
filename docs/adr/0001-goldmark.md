# 1. Use goldmark to render Markdown

Status: accepted · 2026-08-02

## Context

This tool exists because the editor extensions that do the same job anchor comments badly.
They hand Markdown to a renderer that discards provenance, get HTML back with no link to the
source, and then reverse-engineer where a selection came from — storing a copy of the
selected text plus a line number and hoping neither moves.

Rendering server-side deletes that problem only if the renderer can say where each piece of
output came from. So the requirement is not "render Markdown", it is "render Markdown and
report byte offsets".

The standing preference on this project is to own the code rather than glue tools together,
which put a hand-rolled parser genuinely in play: roughly eight constructs appear in a
SKILL.md, and a first cut is a couple of hundred lines with no dependencies.

## Decision

Use `github.com/yuin/goldmark`, with a custom node renderer that stamps `data-o` on every
text run.

`goldmark`'s AST carries `text.Segment{Start, Stop}` on its nodes — the offset map, for
free, from a parser that is CommonMark-compliant and maintained. Overriding five node kinds
gets the offsets out; everything else renders as it already does.

Owning the code means owning the review tool, not reimplementing CommonMark. A hand-rolled
parser would put nested lists, table alignment, fence edge cases, escapes and entities into
our maintenance burden — and every one of those rendering wrong is exactly the jank this
project set out to escape. That is the wrong thing to own.

Linkify is left off (GFM minus autolinking): it splits text into one run per word, which
bloats the HTML in a document whose links are already explicit.

## Consequences

One dependency, no transitive ones.

Offsets hold byte for byte, which the fuzz test in `internal/render` asserts as a property
over arbitrary input. The one documented divergence is NUL, which CommonMark requires be
rendered as U+FFFD; it cannot occur in a file anyone is reviewing.

YAML frontmatter is handled here rather than by an extension. `goldmark-meta` would pull in a
YAML parser to serve one field, so instead the frontmatter is rendered as its own block and
blanked — not sliced — out of the source handed to goldmark, which keeps every later offset
absolute.
