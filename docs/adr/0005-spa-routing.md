# 5. The URL is the file, routed by the History API and a server-side index.html fallback

Status: accepted · 2026-08-02

## Context

The explorer listed the review set as inert text. Selecting a file was impossible, and the
frontend permanently displayed `s.files[0]` — `useDoc` called bare `/api/doc` and the server's
`at("")` fell back to the first file. Everything below the browser was already multi-file:
`at()` validates a `file` against the set discovered at startup, `rev` is per file by design,
and every mutating endpoint takes a `file` in its body. Only the client never sent one.

Making files selectable raises one question the code cannot answer on its own: where does the
selection live? React state would work and would be smaller. It would also make a review
unshareable, unbookmarkable, and unreachable by the back button — for a tool whose whole subject
is a set of documents you move between while reading, that is the wrong trade.

So the selection is the URL, and the review set's own paths are the routes:
`/references/theming.md` reviews `references/theming.md`. `/` keeps its existing meaning of
"whatever the server picks first".

## Decision

### No router

The app has exactly one route shape: everything after `/` is a file path. A router's matchers,
loaders, typed params and route tree have nothing to match on, and TanStack Router in
particular would have added a dependency, a `RouterProvider`, and a route-tree module to express
`/$`. `web/src/hooks/useFilePath.ts` is `useSyncExternalStore` over `popstate` plus a custom
`app:navigated` event, because `pushState` fires nothing of its own. Twenty lines, no build step.

This is a decision to revisit, not a principle. A second route — a settings screen, a diff view,
anything with its own params — is the signal to bring a router in rather than grow this hook.

Path segments are encoded individually, so a filename with a space round-trips and the slashes
stay slashes.

### The Go asset handler falls back to index.html

`assetHandler` was `http.FileServerFS`, which is correct for assets and wrong for routes:
`/references/theming.md` is not a built asset, so it 404'd before the SPA ever loaded. Deep
links — the entire reason for putting the file in the URL — did not work.

Any path that is not a built asset now serves `index.html`. The check is `fs.Stat` against the
embedded FS, which rejects `..` with an error, so a traversal attempt lands on the page rather
than anywhere else. `/api/*` is unaffected: those are separate `mux` patterns and outrank
`GET /`. The not-built branch is untouched — a checkout that has not run `just web` still says
so, on every path.

This also means an unknown path returns 200 and an HTML page, not a 404. The SPA then asks
`/api/doc?file=…`, gets the 400 that `at()` already produced, and renders it as "not one of the
files under review". The status code lives at the API, where a client can act on it; the page is
a shell either way.

### Mutations address `doc.rel`, not the URL

`mutate` now sends `file`. It sends the file the **server** resolved — `doc.rel`, newly added to
the TS `Doc` (the server already served it) — rather than the path in the address bar. At `/`
those differ: the URL says nothing and `doc.rel` says `SKILL.md`. Using the URL would make the
mutation default a second time, independently, and the two defaults would eventually disagree.

Without this the change would have shipped a data-loss bug rather than a feature: with the
selection working, every comment, reply, resolve and delete would have been written into
`files[0]` no matter which file was on screen.

## Consequences

`useDoc` gained a stale-response guard. Clicking through the tree faster than the server answers
would otherwise let an earlier response land last and display the wrong file — a race that could
not exist while there was only one document to fetch.

The Vite dev server needs no help: its SPA fallback already rewrites `.md`-suffixed paths to
`index.html`, verified against `/references/deep/deeper/nested.md`. If that ever changes, the fix
is a `configureServer` middleware, not a change to the URL contract.

The explorer expands the open file's ancestors on arrival, which is a consequence of deep links
rather than a feature of its own: a link into a nested file that rendered with its folders shut
would leave the reviewer unable to see where they were.
