import { Code2, PanelLeft, PanelRight } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Toaster } from "@/components/ui/toast";
import { api } from "@/lib/api";
import { hiddenFileNames } from "@/lib/files";
import { run, say } from "@/lib/notify";
import type { FileEntry, Filter, HandoffResult } from "@/lib/types";
import { cn } from "@/lib/utils";
import { useDoc } from "@/hooks/useDoc";
import { useFilePath } from "@/hooks/useFilePath";
import { useSelection } from "@/hooks/useSelection";
import { Composer } from "./Composer";
import { Document } from "./Document";
import { FileExplorer } from "./FileExplorer";
import { HandoffPanel } from "./HandoffPanel";
import { QuickOpen } from "./QuickOpen";
import { SelectionMenu } from "./SelectionMenu";
import { ThreadList } from "./ThreadList";

// Written out in full rather than composed, because Tailwind's scanner only sees literals.
function gridCols(explorer: boolean, comments: boolean): string {
  if (explorer && comments) return "lg:grid-cols-[16rem_minmax(0,1fr)_24rem]";
  if (explorer) return "lg:grid-cols-[16rem_minmax(0,1fr)]";
  if (comments) return "lg:grid-cols-[minmax(0,1fr)_24rem]";
  return "lg:grid-cols-[minmax(0,1fr)]";
}

export default function App() {
  const [rel, navigate] = useFilePath();
  const { doc, error, mutate } = useDoc(rel);
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filter, setFilter] = useState<Filter>("markdown");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [handoff, setHandoff] = useState<{ summary: string; prompt: string | null } | null>(null);
  const [explorerOpen, setExplorerOpen] = useState(true);
  const [commentsOpen, setCommentsOpen] = useState(true);
  const [showFrontmatter, setShowFrontmatter] = useState(false);
  const [quickOpen, setQuickOpen] = useState(false);

  const docContainerRef = useRef<HTMLDivElement>(null);
  const { pending, anchorRect, composerOpen, menuRef, openComposer, closeComposer } =
    useSelection(docContainerRef);

  // Returns the fetched array too, not just via setFiles — a caller that awaits it and
  // immediately needs the result (the handoff summary) would otherwise read the stale
  // pre-update `files` from its own render closure.
  const refreshFiles = useCallback(async () => {
    const fresh = await api<FileEntry[]>("/api/files");
    setFiles(fresh);
    return fresh;
  }, []);

  useEffect(() => {
    const file = doc?.rel?.slice(doc.rel.lastIndexOf("/") + 1);
    document.title = doc?.name ? `${file} — ${doc.name}` : "skill-review";
  }, [doc?.name, doc?.rel]);

  // A thread id belongs to one file, so carrying the selection across a file switch would
  // highlight nothing and leave the composer pointing at a range that no longer exists.
  useEffect(() => {
    setSelectedId(null);
    closeComposer();
  }, [doc?.rel, closeComposer]);

  // Every redraw (initial load, and every mutation) refetches the file list — its thread
  // counts are what Submit would actually ship, and only the server applies those rules.
  useEffect(() => {
    if (doc) void refreshFiles();
  }, [doc, refreshFiles]);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setQuickOpen(true);
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  async function handleSubmitComment(body: string) {
    if (!pending) return;
    await run(async () => {
      await mutate("/api/anchor", { ...pending, body });
      closeComposer();
      window.getSelection()?.removeAllRanges();
      say("Comment saved to the file.");
    });
  }

  function handleReply(id: string, body: string) {
    void run(async () => {
      await mutate("/api/thread", { id, body });
      say("Reply saved to the file.");
    });
  }

  function handleToggleStatus(id: string) {
    const thread = doc?.threads.find((t) => t.id === id);
    const status = thread?.status === "resolved" ? "open" : "resolved";
    void run(() => mutate("/api/thread", { id, status }));
  }

  function handleDelete(id: string) {
    void run(async () => {
      await mutate("/api/thread/delete", { id });
      say("Thread removed.");
    });
  }

  function handleFieldChange(id: string, field: string, value: string) {
    void run(() => mutate("/api/thread", { id, fields: { [field]: value } }));
  }

  async function copyPrompt(prompt: string) {
    try {
      await navigator.clipboard.writeText(prompt);
      say("Prompt copied to the clipboard.");
    } catch {
      say("Could not reach the clipboard — copy the prompt from the panel or the terminal.", true);
    }
  }

  function handleSubmitAll() {
    void run(async () => {
      const result = await api<HandoffResult>("/api/handoff", {});
      const freshFiles = await refreshFiles();
      const count = result.payload.improvement_suggestions.length;

      if (count === 0) {
        setHandoff({
          summary: "Nothing to hand off — every open thread has already been archived.",
          prompt: null,
        });
        return;
      }

      const noun = `${count} suggestion${count === 1 ? "" : "s"}`;
      const hidden = hiddenFileNames(freshFiles, filter);
      const from = hidden ? ` — including ${hidden}, which the filter is hiding` : "";
      const summary = result.changed
        ? `${noun} pending in ${result.file}${from}`
        : `Nothing new — ${noun} still pending in ${result.file}${from}`;

      setHandoff({ summary, prompt: result.prompt });
      await copyPrompt(result.prompt);
    });
  }

  return (
    <div className="flex min-h-dvh flex-col bg-background text-foreground lg:h-dvh">
      <header className="z-20 flex h-(--header-h) shrink-0 items-center gap-2 border-b bg-card px-4">
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-pressed={explorerOpen}
          aria-label="Toggle the file explorer"
          title="Toggle the file explorer"
          onClick={() => setExplorerOpen((open) => !open)}
        >
          <PanelLeft className={cn(!explorerOpen && "text-muted-foreground")} />
        </Button>
        <h1 className="m-0 shrink-0 text-sm font-medium">{doc?.name || "skill-review"}</h1>
        <code className="min-w-0 truncate text-xs text-muted-foreground">{doc?.path}</code>

        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="ml-auto"
          aria-pressed={showFrontmatter}
          title="Show or hide the YAML frontmatter"
          onClick={() => setShowFrontmatter((shown) => !shown)}
        >
          <Code2 className={cn(!showFrontmatter && "text-muted-foreground")} />
          Frontmatter
        </Button>
        {/* The file list is the authority on what Submit would ship, but it refetches a beat
            behind a mutation — so the open doc's own threads count too, and the button doesn't
            blink out of existence right after the first comment lands. */}
        {(files.some((file) => file.threads > 0) || (doc?.threads.length ?? 0) > 0) && (
          <Button type="button" size="sm" onClick={handleSubmitAll}>
            {doc?.updater ? `Submit all to ${doc.updater}` : "Submit all"}
          </Button>
        )}
        <Button
          type="button"
          size="icon-sm"
          variant="ghost"
          aria-pressed={commentsOpen}
          aria-label="Toggle the comments pane"
          title="Toggle the comments pane"
          onClick={() => setCommentsOpen((open) => !open)}
        >
          <PanelRight className={cn(!commentsOpen && "text-muted-foreground")} />
        </Button>
      </header>

      {/* Three panes that scroll independently, as an editor does. Below lg they stack and
          the page scrolls as one, with the explorer capped so it can't push the doc away.
          A collapsed pane is hidden rather than unmounted, so it keeps its expanded folders
          and its scroll position — and display:none also drops it out of the grid, which is
          why the column template has to change with it. */}
      <main className={cn("grid min-h-0 flex-1 grid-cols-1", gridCols(explorerOpen, commentsOpen))}>
        <div
          className={cn(
            "order-first max-h-64 lg:order-none lg:max-h-none lg:min-h-0",
            !explorerOpen && "hidden",
          )}
        >
          <FileExplorer
            files={files}
            filter={filter}
            selected={doc?.rel ?? null}
            onFilterChange={setFilter}
            onSelect={navigate}
          />
        </div>

        {/* The pane is the reading surface, not a card floating on one. max-w-3xl holds
            the prose to a ~70ch measure however wide the window gets. */}
        <div className={cn("bg-card lg:min-h-0 lg:overflow-y-auto", !showFrontmatter && "hide-frontmatter")}>
          <div className="mx-auto max-w-3xl px-6 py-8 md:px-10 md:py-12">
            <Document
              containerRef={docContainerRef}
              html={doc?.html ?? null}
              error={error}
              threads={doc?.threads ?? []}
              selectedId={selectedId}
              onSelectThread={(id) => {
                setCommentsOpen(true);
                setSelectedId(id);
              }}
            />
          </div>
        </div>

        <aside
          className={cn(
            "flex flex-col border-t lg:min-h-0 lg:border-t-0 lg:border-l",
            !commentsOpen && "hidden",
          )}
        >
          <h2 className="shrink-0 px-3 py-2 text-[0.6875rem] font-medium tracking-widest text-muted-foreground uppercase">
            Comments
          </h2>
          <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-3 pb-3">
            {composerOpen && pending ? (
              <Composer quote={pending.quote} onCancel={closeComposer} onSubmit={handleSubmitComment} />
            ) : null}

            <ThreadList
              threads={doc?.threads ?? []}
              fields={doc?.fields ?? []}
              selectedId={selectedId}
              onReply={handleReply}
              onToggleStatus={handleToggleStatus}
              onDelete={handleDelete}
              onFieldChange={handleFieldChange}
            />
          </div>
        </aside>
      </main>

      {/* Both entry points into the comments pane have to reveal it — a composer or a
          selected thread rendered inside a collapsed aside is a click that does nothing. */}
      <SelectionMenu
        menuRef={menuRef}
        anchorRect={anchorRect}
        onComment={() => {
          setCommentsOpen(true);
          openComposer();
        }}
      />

      {quickOpen && (
        <QuickOpen files={files} onClose={() => setQuickOpen(false)} onSelect={navigate} />
      )}

      {handoff && (
        <HandoffPanel
          summary={handoff.summary}
          prompt={handoff.prompt}
          onCopy={() => void copyPrompt(handoff.prompt ?? "")}
          onClose={() => setHandoff(null)}
        />
      )}

      <Toaster />
    </div>
  );
}
