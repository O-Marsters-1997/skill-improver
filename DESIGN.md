---
name: skill-review
description: A manuscript desk for reading a skill closely and marking what should change.
colors:
  paper: "oklch(0.985 0.003 84.6)"
  paper-dark: "oklch(0.203 0.008 297.1)"
  page: "oklch(1 0 0)"
  page-dark: "oklch(0.238 0.01 294.8)"
  ink: "oklch(0.223 0.002 67.7)"
  ink-dark: "oklch(0.926 0.007 80.7)"
  ink-faded: "oklch(0.516 0.008 67.6)"
  ink-faded-dark: "oklch(0.67 0.012 71.8)"
  sienna-ink: "oklch(0.534 0.103 54.9)"
  sienna-ink-dark: "oklch(0.759 0.106 59.1)"
  rule: "oklch(0.903 0.01 72.7)"
  rule-dark: "oklch(0.319 0.016 295.7)"
  highlighter: "oklch(0.939 0.088 92.7)"
  highlighter-dark: "oklch(0.418 0.065 86.6)"
  highlighter-open: "oklch(0.927 0.065 83.6)"
  highlighter-open-dark: "oklch(0.459 0.07 82.7)"
  correction: "oklch(0.543 0.174 29.7)"
typography:
  display:
    fontFamily: "ui-sans-serif, system-ui, sans-serif"
    fontSize: "2.25rem"
    fontWeight: 800
    lineHeight: 1.1
  body:
    fontFamily: "ui-sans-serif, system-ui, sans-serif"
    fontSize: "1rem"
    fontWeight: 400
    lineHeight: 1.75
  title:
    fontFamily: "ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 500
    lineHeight: 1.4
  label:
    fontFamily: "ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.6875rem"
    fontWeight: 500
    lineHeight: 1
    letterSpacing: "0.1em"
  chrome:
    fontFamily: "ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.75rem"
    fontWeight: 400
    lineHeight: 1
rounded:
  sm: "0.3rem"
  md: "0.4rem"
  lg: "0.5rem"
  xl: "0.7rem"
spacing:
  row: "1.75rem"
  gutter: "0.75rem"
  pane: "0.75rem"
  measure: "48rem"
  rail: "16rem"
  aside: "24rem"
  header: "3.25rem"
components:
  button-primary:
    backgroundColor: "{colors.sienna-ink}"
    textColor: "{colors.page}"
    rounded: "{rounded.lg}"
    padding: "0 0.75rem"
    height: "2rem"
  tree-row:
    textColor: "{colors.ink}"
    typography: "{typography.chrome}"
    height: "{spacing.row}"
    padding: "0 0.5rem"
  tree-row-selected:
    backgroundColor: "{colors.sienna-ink}"
    textColor: "{colors.page}"
    height: "{spacing.row}"
  tree-row-hover:
    backgroundColor: "{colors.rule}"
    height: "{spacing.row}"
  panel-header:
    textColor: "{colors.ink-faded}"
    typography: "{typography.label}"
    padding: "0.5rem 0.75rem"
  thread-card:
    backgroundColor: "{colors.page}"
    textColor: "{colors.ink}"
    rounded: "{rounded.lg}"
    padding: "1rem"
---

# Design System: skill-review

## Overview

**Creative North Star: The Manuscript Desk.**

A warm paper surface with a document on it and editor's marks in the margin. Every colour in the
system earns its place against that metaphor: off-white paper, sienna ink, an amber highlighter
over passages under discussion. The interface is the desk, not the work — chrome is drained of
colour and held to thin rules so the prose is the only thing with presence.

Mood: quiet, warm, close. Considered rather than styled. The reviewer is doing sustained close
reading, often of their own writing, and the surface should feel like somewhere you can stay for
twenty minutes.

Anti-reference: the SaaS dashboard. No card grids, no stat tiles, no gradient headers, no
coloured section accents. If a screen starts to look like it is reporting on something rather
than presenting a document to be read, it has gone wrong.

Everything is defined in `web/src/index.css` — a Tailwind v4 `@theme inline` block over CSS
custom properties. There is no `tailwind.config`. Every token has a light and a dark value; the
`-dark` keys in the frontmatter above are the `prefers-color-scheme: dark` half of the same
token, not separate tokens.

## Colors

One accent. That is the whole colour strategy, and it is load-bearing: because nothing else in
the interface is saturated, **Sienna Ink** reliably means "this is the thing you are on".

| Role | Token | Character |
| --- | --- | --- |
| Desk | `--background` / `--sidebar` | **Warm Paper** — off-white with a trace of yellow. The chrome tone. |
| Page | `--card` | **Page White** — pure white in light, a lifted slate in dark. The reading surface. |
| Text | `--foreground` | **Ink** — near-black, warmed off neutral. |
| Secondary text | `--muted-foreground` | **Faded Ink** — the same hue at half strength, never a cool grey. |
| Accent | `--primary` / `--sidebar-primary` | **Sienna Ink** — muted mid-brown with a red-orange lean. |
| Anchors | `--highlight` | **Highlighter** — pale amber over commented passages. |
| Open anchors | `--highlight-open` | **Highlighter, Open** — a half-step warmer, for unresolved threads. |
| Rules | `--border` / `--input` | **Rule** — the 1px lines that do all the dividing. |
| Errors | `--destructive` | **Correction Red** — the only other saturated colour, and only for loss. |

Rules that hold:

- Never a raw Tailwind palette value (`text-blue-500`) and never an inline hex or `oklch()`.
  Semantic tokens only.
- Never a manual `dark:` colour override. Dark mode is a media query redefining the same custom
  properties, so a token is correct in both themes by construction. A `dark:` in a colour class
  means the token is wrong.
- The rail and the document surface must stay tonally distinct in **both** themes. In light,
  paper against white; in dark, the rail drops to the background value so the same relationship
  survives. This is why `--sidebar` is not simply `--card`.
- Anything new that needs a colour gets a token, not an arbitrary value.

## Typography

The system font stack, unstyled and deliberately so — this is a tool for reading Markdown, and
Markdown's own voice should come through rather than a typeface's. There are no font tokens and
no webfonts to load.

The document is `@tailwindcss/typography`'s `prose prose-neutral dark:prose-invert`, applied to
`#doc` and held to a `max-w-3xl` container so the measure lands near 70ch however wide the window
gets. That container is the only place the measure is set; `prose`'s own `max-w` is disabled.

Chrome typography is a tight three-step scale that never competes with the document: `title`
(14px medium) for the app name, `chrome` (12px) for paths and tree rows, and `label` (11px, wide
tracking, uppercase) for the two panel headers.

**Panel headers are a two-item system, not a decoration.** EXPLORER and COMMENTS name the two
chrome panes. Do not add a third over the document — its content is its title — and do not use
the treatment as a generic section eyebrow.

## Layout

A title bar over three independently scrolling panes, divided by 1px rules rather than gaps:

```
header                       3.25rem, --card, sticky
├── rail        16rem        --sidebar        the file tree
├── document    1fr          --card           max-w-3xl, centred
└── comments    24rem        --background     threads and composer
```

The header height is the `--header-h` token because all three panes size against it; it must not
be re-derived as a magic number in any pane.

Panes butt against each other. There is no page padding and no outer container — the shell fills
the viewport (`lg:h-dvh`) and each pane owns its own overflow. Below `lg` they stack and the page
scrolls as one, with the rail capped at 16rem so it cannot push the document off-screen.

Flex for single-axis stacks with `gap-*`; never `space-y-*`. Grid only for the genuinely
two-dimensional shell.

## Elevation & Depth

**Flat by rule.** Depth is tone and 1px rules — surfaces are distinguished by which token they
carry, not by shadow.

A shadow means **detached from the layout**, and nothing else. That is currently two things: the
selection toolbar that floats beside a highlighted passage, and the handoff panel. Both are
genuinely outside the document flow, and both use a shadow with real offset and blur.

A card that sits in a pane gets a plain 1px border. Not a shadow, not a glow, and not a thick
coloured left edge — the side-tab accent is the most recognisable tell of generated UI.

The one exception is a **quotation**, where a left rule is typographic convention rather than
decoration: the composer's and thread cards' quoted passages carry `border-l-3` in the
highlighter tone, matching what `prose` does to a blockquote. Quotes may; cards, callouts, list
items and alerts may not.

## Shapes

One radius, `--radius: 0.5rem`, with everything else derived from it by multiplication
(`--radius-sm` is `0.6×`, `--radius-xl` is `1.4×`, and so on). Add a step to the scale rather
than an arbitrary corner value.

Tree rows are **square**. The rail is a panel, not a stack of cards, and rounded rows in a dense
28px list read as buttons — which is exactly the wrong affordance for a treeview.

## Components

Primitives are **Base UI**, not Radix. Composition uses the `render` prop rather than `asChild`,
and `Select` requires `items` on the Root or `Select.Value` shows the raw value instead of the
label. Only six primitives are installed (button, card, label, select, textarea, toast) — add
more with `npx shadcn@latest add`, don't hand-roll them.

**File tree.** A WAI-ARIA treeview: `role="tree"` / `treeitem` / `group`, `aria-level`,
`aria-expanded` on folders, `aria-selected` on files. Roving tabindex — the tree is one tab stop
and the arrow keys move within it. The `<li>` is the focusable element; there is no nested
button, and adding one would be an a11y regression as well as a tab-order one. Rows are 28px,
indent is 0.75rem per level, folders sort before files. The selected row is the single saturated
element on screen.

**Document.** No chrome of its own — no border, no card, no shadow. The pane is the surface. It
is server-rendered HTML injected with `dangerouslySetInnerHTML`, so its styles live in the
`@layer components` block of `index.css` where Tailwind's scanner cannot miss them.

**Thread cards.** Bordered, on `--card`, in the comments pane. The one place stacked containers
are legitimate — and they are still not nested.

**Buttons.** Built-in variants first (`outline`, `ghost`, `destructive`, `size="sm"`). Add a
`cva` variant only when a treatment repeats in more than one place.

## Do's and Don'ts

**Do**

- Reach for a semantic token; add one to `index.css` if none fits.
- Let the document be the only thing with visual weight.
- Use `cn()` for conditional classes, `size-*` when width equals height, `truncate` for overflow.
- Keep custom widgets on their ARIA pattern, and check the keyboard path before the pixels.

**Don't**

- Add a second accent colour. It would compete with the selected row, which is the one signal
  that has to be unmissable.
- Put a shadow on anything that sits in the layout.
- Wrap the document in a card. It was, and it read as a sheet floating in a void.
- Use monospace as a costume for "technical". It is for paths, code, and offsets.
- Add a `dark:` colour override, a raw palette value, or an arbitrary hex.
- Turn tree rows into buttons for convenience.
