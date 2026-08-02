---
name: example-skill
description: A fixture that exercises every construct a real SKILL.md uses. Not a real skill.
---

# Example Skill

This fixture exists so the renderer is tested against the shapes that actually appear
in a SKILL.md: headings, **bold**, *italic*, `inline code`, fences, lists, tables and
[links](https://example.com).

## When to use

<!--mc:a:trnv3v-->Use this when you need a document that is long enough to scroll and varied enough to
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
of which shift byte offsets if handled carelessly.<!--mc:/a:trnv3v-->

<!--mc:threads:begin-->
<!--mc:t {"id":"trnv3v","quote":"Use this when you need a document that is long enough to scroll and varied enough to\nbreak a naive parser. The paragraph above deliberately spans two source lines, because\nsoft line breaks are where offset arithmetic usually goes wrong.\n\n## Process\n\n1. Read the input in full.\n2. Decide which of the branches below applies.\n3. Report what you did — never claim a step you skipped.\n\n### Branches\n\n- **Simple case** — one file, one edit. Do it inline.\n- **Wide case** — several files. List them first, then edit.\n- **Blocked** — say so and stop. Do not guess.\n\n## Example\n\n```go\nfunc Handle(w http.ResponseWriter, r *http.Request) {\n    if r.Method != http.MethodPost {\n        http.Error(w, \"method not allowed\", http.StatusMethodNotAllowed)\n        return\n    }\n    fmt.Fprintln(w, \"ok\")\n}\n```\n\nThe snippet above is deliberately inside a fence, so a highlight landing in it has to\nexpand to the whole block rather than corrupt the code.\n\n## Reference\n\n| Field      | Values                        | Default        |\n| ---------- | ----------------------------- | -------------- |\n| `priority` | high, medium, low             | medium         |\n| `category` | instructions, tools, examples | instructions   |\n\n\u003e A blockquote, because SKILL.md files use them for warnings.\n\nFinally, a line with an escaped asterisk (\\*), an ampersand (\u0026), and an em dash — all\nof which shift byte offsets if handled carelessly.","status":"open","comments":[{"id":"c1","author":"ollymarsters","ts":"2026-08-02T13:41:33Z","body":"does not include an information section it should"}],"priority":"medium","category":"instructions"}-->
<!--mc:threads:end-->
