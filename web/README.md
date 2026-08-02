# web/

The `skill-review` frontend: one screen, built by Vite into `../internal/server/web` and
embedded into the Go binary with `go:embed`.

```bash
bun install
bun run build      # tsc -b && vite build → ../internal/server/web
bun run dev        # port 5173, proxies /api → 127.0.0.1:8420
bun test           # src/lib/*.test.ts
bun run lint       # oxlint
```

Dev needs both halves running: `go run ./cmd/skill-review <target>` for the API, and
`bun run dev` for the page. From the repo root, `just web` builds and `just run` serves the
built binary.

**`../internal/server/web/` is build output, not source.** Only `not-built.html` is committed
there — hence `emptyOutDir: false` in `vite.config.ts`, which would otherwise delete the page a
fresh checkout sees before `just web` has run. Output is a single `app.js` / `app.css` pair,
referenced absolutely so a deep link at any depth still resolves them.

## Stack

React 19 · Vite 8 · TypeScript · Tailwind v4 (CSS-first, no config file) · shadcn/ui over
**Base UI** (not Radix) · lucide-react · oxlint · bun.

## Layout

```
src/
  features/review/   the screen: App, FileExplorer, FileTree, Document,
                     ThreadList, ThreadCard, Composer, SelectionMenu, HandoffPanel
  hooks/             useDoc (the one server resource), useFilePath (routing),
                     useSelection (text selection → a pending comment)
  lib/               api, types, files, tree, offsets, notify, utils
  components/ui/     shadcn primitives — generated, edit via the CLI
  index.css          every design token; there is no tailwind.config
```

## Things that will bite you

- **The URL is the file.** `/references/theming.md` reviews that path. Routing is
  `history.pushState` plus `useSyncExternalStore` in `hooks/useFilePath.ts` — no router. Deep
  links depend on the Go asset handler serving `index.html` for non-assets; see
  `docs/adr/0005-spa-routing.md`.
- **Mutations must carry `doc.rel`**, not the URL path. The server defaults an empty `file` to
  the first file in the set, so omitting it writes comments into the wrong document.
- **`#doc` is server-rendered HTML** injected with `dangerouslySetInnerHTML`. Tailwind's scanner
  never sees those classes, so they are hand-written in the `@layer components` block of
  `index.css`. `internal/render` escapes every raw tag that is not an mc marker.
- **Dark mode follows the OS**, via `@media (prefers-color-scheme: dark)`. There is no toggle,
  which means the built-in media `dark:` variant rather than shadcn's class-based one. Adding a
  toggle means changing the variant, not just adding a button.
- **Base UI's `Select` needs `items` on the Root** or `Select.Value` renders the raw value
  instead of the item's label.
- **The explorer is a WAI-ARIA treeview**, not a list of buttons: one tab stop, roving tabindex,
  arrow keys inside. Turning rows into `<button>`s would put every file in the tab order and
  strand keyboard users before the document.
