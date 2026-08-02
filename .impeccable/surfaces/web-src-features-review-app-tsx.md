# Surface brief: the review screen

**Mode: Operate.** The visitor is completing a task — reading a skill closely and marking what
should change. Scanability, consistency, and native expectations outrank expression. Brand
lives in precise details, not in statements.

## Job and audience

A skill author, at their own machine, mid-session, reading their own writing critically. They
already know the content; what they need is to move through it and register objections without
losing their place. They are keyboard-first and impatient with chrome.

## Outcome and proof

Primary task: read a file, select a passage, attach a comment, move to the next file, submit
everything to the updater agent. Success is that the reviewer never has to think about the
tool — where they are, which file they're editing, or whether a comment landed.

The proof this surface owes the reviewer is *location*: which file is open, which files still
hold unaddressed threads, and where in the set they are. Before this change none of that was
visible; the sidebar listed paths that could not be clicked.

## Selected direction

**Editor chrome, three panes.** The incumbent world (warm off-white/brown OKLCH, OS-driven dark
mode, Base UI primitives, Tailwind v4 CSS-first tokens) is preserved; only the structure changes.

- Structural thesis: a title bar over three independently scrolling panes — explorer, document,
  comments — divided by 1px borders rather than gaps. The panes are surfaces, not cards. Nothing
  floats.
- The document pane is the reading surface itself (`--card`), held to a ~70ch measure and given
  generous vertical rhythm. Both side panes are chrome (`--sidebar` rail, `--background`
  comments), separated from the document by tone in both themes.
- Focal moment: the open file's row in the explorer, the single saturated element on the screen.
  Everything else is neutral, so the eye finds "where am I" instantly.
- The panel headers (EXPLORER / COMMENTS) are a two-item system, not decoration. Do not add a
  third label to the document pane — its content is its title.

## Scope and boundaries

In scope: the explorer tree, the three-pane shell, and the URL contract. Untouched: the anchoring
model, the server-rendered `#doc` prose layer, the thread cards, the handoff panel, and the
palette. Anti-goals: a visual redesign of the document typography, a theme toggle, a command
palette, and any chrome that competes with the prose.

## States and ranges

- Review sets run from **1 file** (single-file target, no folders) to a few dozen across two or
  three folder levels. Not thousands — no virtualization.
- Loading: the document pane shows a plain "Loading…"; the panes and explorer stay put.
- Error: a URL naming a file outside the review set states so in the reviewer's terms and offers
  the way back. Never the server's wording.
- Filter: Markdown-only by default. Files it hides that still carry threads are named at the foot
  of the rail, because Submit would ship them.
- Deep link: arrives with its folders already expanded and its row selected.

## Interaction and layout

- URL is the selection: `/references/theming.md` opens that file. Back and forward walk the
  review. `/` means the first file.
- Keyboard: the tree is one tab stop (WAI-ARIA treeview). ↑↓ move, →← expand/collapse and climb,
  Home/End jump, Enter opens. Tab leaves for the document in one press — this is the whole reason
  for the roving tabindex, and it must not regress into a per-row tab order.
- Rows are 28px and dense; folders sort before files. Thread counts ride at the row's right edge.
- Below `lg` the panes stack, the rail capped at 16rem so it cannot push the document off-screen.

## Constraints and open decisions

- No router dependency: routing is `history.pushState` plus `useSyncExternalStore`
  (`web/src/hooks/useFilePath.ts`). Deep links depend on the Go asset handler's index.html
  fallback — see `docs/adr/0005-spa-routing.md`.
- Reuse before adding: `--sidebar-*` tokens, `isShown`/`hiddenFileNames`, the existing Base UI
  Select. Base UI's `Select` needs `items` on the Root or `Select.Value` renders the raw value.
- Do not invent status colours. There is one accent; a second would compete with the selected row.
