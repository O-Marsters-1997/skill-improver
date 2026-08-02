# 4. Render HTML targets by tokenising, and sanitise with an allowlist

Status: accepted · 2026-08-02

## Context

A skill's output is as often HTML as Markdown — a coverage report, a generated page, a
`references/` file. Until now `render.HTML` was the only renderer and it took Markdown, so
an `.html` target was fed to goldmark and came out as escaped source text.

The Markdown renderer's rule is *escape everything*, pinned by the test at
`render_test.go:83`: raw HTML in a Markdown file is shown, never executed. That rule
cannot survive contact with an HTML target, where the markup **is** the document. Reversing
it means the tool now has a genuine untrusted-input problem it did not have before.

Two constraints do not move. Anchoring is the property the whole tool rests on: every
rendered run's text must be exactly the source bytes at the offset it advertises, or a
highlight lands in the wrong place. And Markdown output must stay byte-identical, because
its golden file is the only guard on a renderer nobody is otherwise touching.

## Decision

### `html.Tokenizer`, not `html.Parse`

`html.Parse` builds a tree whose nodes have forgotten where they came from — `html.Node`
carries no offset. `Tokenizer.Raw()` returns the source bytes of the current token, so
accumulating `len(Raw())` across `Next()` tracks absolute byte position. A spike over
nested elements, attributes, comments, entities, `<pre>`, rawtext elements, CRLF, BOM,
malformed entities and unterminated comments confirmed the accumulation consumes the source
exactly, and the fuzz target now holds it.

One hard rule follows. `Text()` unescapes and normalises newlines **in place**, in the
buffer `Raw()` points into, and `Token()` calls `Text()`. Neither is ever called here. The
renderer reads `len(Raw())` and then slices `src` itself, so no tokenizer buffer is trusted
to survive.

### An allowlist, and the escape-everything rule reversed

Elements not on the list are dropped; attributes not on the list are dropped; `href` and
`src` values whose scheme is not `http`, `https` or `mailto` are dropped. A denylist was
rejected for the usual reason — it is a list of the attacks someone thought of.

Tags are **rebuilt** from the tokenizer rather than copied from the source, so there is no
path by which an attribute reaches the page without being named on the allowlist. `on*`
handlers and `srcdoc` need no special case; they are simply absent from it. Scheme checking
strips ASCII whitespace and control characters first, because browsers ignore them inside a
scheme and `java&#9;script:` is a live `javascript:` URL — the tokenizer has already decoded
the entity by then, so obfuscation through entities is closed off too.

Some elements lose their bodies as well as their tags, and the line is drawn by what the
tokenizer does with them. `script`, `style`, `iframe`, `noscript`, `noembed`, `noframes`,
`xmp` and `plaintext` are *rawtext*: their contents arrive as one unparsed token, so
dropping only the tags would print a stylesheet, a program, or a slab of raw markup into
the middle of the document as visible text — escaped and inert, but nonsense.

`object`, `embed` and `form` keep their children, which the tokenizer does parse. A form's
labels and an object's fallback are the author's words, and with the tag gone and no
allowlisted attribute left there is nothing to embed or submit. `html`, `head` and `body`
are the same call for a different reason — meaningless in a fragment spliced into an
existing page, but a `<title>` is still the author's words.

### Source bytes are emitted, escaped — not `Text()`

`assertOffsets` unescapes a span's text and compares it against `src[offset:offset+n]`.
`Text()` returns *unescaped* text, so a source `&lt;` would arrive as one byte where the
source has five and the property would break. The source bytes are written through
goldmark's `RawWrite` instead, which escapes rather than passes through: a source `&lt;`
becomes `&amp;lt;`, and unescaping it recovers `&lt;` — the five source bytes, exactly.

The cost is that a source `&nbsp;` displays literally rather than as a space. Anchoring is
the property the whole tool rests on and this is what it costs; it wins. The upgrade path,
should it grate, is marked with a `ponytail:` comment: split text tokens at entity
boundaries.

### A CR ends a run

This one is invisible to every assertion on the rendered bytes. An HTML parser folds CR and
CRLF to LF *before* `textContent` exists, and `app.js` measures offsets with
`byteLength(span.textContent)`. A CR inside a span is therefore a byte `data-o` counts and
the browser cannot, and every anchor after it drifts by one per line — while the offset
assertions and the fuzz target, which read the output as bytes rather than as a DOM, both
still pass.

Each CR ends its run and is written outside any span, where nothing measures it. The fix
lives in `writeOffsetSpan`, which both renderers call, because **Markdown had the same bug**.
Goldmark does not keep CR out of its segments: a fenced code block in a CRLF file rendered
as one span containing `code\r\n`, and every anchor inside it was already drifting a byte
per line. Fixing only the path this ADR is about would have left the older one broken.

This is the one place Markdown output is not byte-identical to before, and the exception is
deliberate: it changes only sources containing CR, and only by correcting them.

### Fence expansion is gated on the host format

`comments.expandFences` grows a span to contain a whole ``` fence so markers are never
inserted into code. That is a Markdown rule. In an HTML file backticks are ordinary text,
and growing the span would move the anchor off what the reviewer selected. `comments.Anchor`
therefore takes a `Format`, and `internal/server` derives it from the file extension in one
place — the same place that picks the renderer, so the two can never disagree.

The threads block needs no such gate. `Upsert` appends it to the end of the file, which for
an HTML target puts it after `</html>`, and `Anchor` already refuses any span at or past
`threadsBegin` — so everything reviewable stays anchorable.

## Consequences

`golang.org/x/net` joins the dependency list. It is the standard tokenizer, maintained
alongside the toolchain, and writing one is not a thing to do by hand at a security
boundary.

A comment an author wrote in the HTML source renders as nothing, where the Markdown path
escapes and shows unknown markup. This departs from routing everything through
`writeMarkup`, deliberately: a comment is invisible in the browser the file was built for,
and showing `<!-- TODO -->` as visible text mid-paragraph is a rendering bug, not a feature.
Only mc's own markers are acted on.

Sanitising is not defence in depth for a tool that serves one reviewer their own files on
loopback — the page inserts the output with `innerHTML`, which does not run `<script>`
anyway. It is here because the next thing this renders is a file some skill generated from
a source nobody read.

NUL is exempt from the offset property, on both paths and for the same reason: the HTML
spec and goldmark's escape table both turn it into U+FFFD, so one source byte becomes
three. Both fuzz targets skip it.
