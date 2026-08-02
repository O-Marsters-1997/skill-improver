---
name: example-skill
description: A fixture that exercises every construct a real SKILL.md uses, including links out to reference files. Not a real skill.
---

# Example Skill

This fixture exists so the reviewer is tested against the shapes that actually appear in
a real skill: headings, **bold**, *italic*, `inline code`, lists, a blockquote,
[links](https://example.com) — and a couple of `references/` files a real SKILL.md would
lean on for detail rather than inlining everything itself.

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

See [references/handler-example.md](references/handler-example.md) for the request
handler this fixture is built around, fenced code and all.

## Reference

See [references/field-table.md](references/field-table.md) for the field table, the
blockquote, and a line of characters — an escaped asterisk, an ampersand, an em dash —
that shift byte offsets if handled carelessly.<!--mc:/a:trnv3v-->

<!--mc:threads:begin-->
<!--mc:t {"id":"trnv3v","quote":"Use this when you need a document that is long enough to scroll and varied enough to\nbreak a naive parser. The paragraph above deliberately spans two source lines, because\nsoft line breaks are where offset arithmetic usually goes wrong.\n\n## Process\n\n1. Read the input in full.\n2. Decide which of the branches below applies.\n3. Report what you did — never claim a step you skipped.\n\n### Branches\n\n- **Simple case** — one file, one edit. Do it inline.\n- **Wide case** — several files. List them first, then edit.\n- **Blocked** — say so and stop. Do not guess.\n\n## Example\n\nSee [references/handler-example.md](references/handler-example.md) for the request\nhandler this fixture is built around, fenced code and all.\n\n## Reference\n\nSee [references/field-table.md](references/field-table.md) for the field table, the\nblockquote, and a line of characters — an escaped asterisk, an ampersand, an em dash —\nthat shift byte offsets if handled carelessly.","status":"open","comments":[{"id":"c1","author":"ollymarsters","ts":"2026-08-02T13:41:33Z","body":"does not include an information section it should"}],"priority":"medium","category":"instructions"}-->
<!--mc:threads:end-->
