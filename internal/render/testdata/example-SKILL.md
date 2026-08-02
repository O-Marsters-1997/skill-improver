---
name: example-skill
description: A fixture that exercises every construct a real SKILL.md uses. Not a real skill.
---

# Example Skill

This fixture exists so the renderer is tested against the shapes that actually appear
in a SKILL.md: headings, **bold**, *italic*, `inline code`, fences, lists, tables and
[links](https://example.com).

## When to use

Use this when you need a document that is long enough to scroll and varied enough to
break a naive parser. The paragraph above deliberately spans two source lines, because
soft line breaks are where offset arithmetic usually goes wrong.

## Process

1. Read the input in full.
2. Decide which of the branches below applies.
3. Report what you did — never claim a step you skipped.

### Branches

- **Simple case** — one file, one edit. Do it inline.
- **Wide case** — several files. List them first, then edit.
- **Blocked** — say so and stop. Do not guess.

## Example

```go
func Handle(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    fmt.Fprintln(w, "ok")
}
```

The snippet above is deliberately inside a fence, so a highlight landing in it has to
expand to the whole block rather than corrupt the code.

## Reference

| Field      | Values                        | Default        |
| ---------- | ----------------------------- | -------------- |
| `priority` | high, medium, low             | medium         |
| `category` | instructions, tools, examples | instructions   |

> A blockquote, because SKILL.md files use them for warnings.

Finally, a line with an escaped asterisk (\*), an ampersand (&), and an em dash — all
of which shift byte offsets if handled carelessly.
