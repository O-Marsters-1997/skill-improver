import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Toaster } from "@/components/ui/toast";
import { api } from "@/lib/api";
import { hiddenFileNames } from "@/lib/files";
import { run, say } from "@/lib/notify";
import type { FileEntry, Filter, HandoffResult } from "@/lib/types";
import { useDoc } from "@/hooks/useDoc";
import { useSelection } from "@/hooks/useSelection";
import { Composer } from "./Composer";
import { Document } from "./Document";
import { FileExplorer } from "./FileExplorer";
import { HandoffPanel } from "./HandoffPanel";
import { SelectionMenu } from "./SelectionMenu";
import { ThreadList } from "./ThreadList";

export default function App() {
  const { doc, mutate } = useDoc();
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filter, setFilter] = useState<Filter>("markdown");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [handoff, setHandoff] = useState<{ summary: string; prompt: string | null } | null>(null);

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
    document.title = doc?.name ? `${doc.name} — skill-review` : "skill-review";
  }, [doc?.name]);

  // Every redraw (initial load, and every mutation) refetches the file list — its thread
  // counts are what Submit would actually ship, and only the server applies those rules.
  useEffect(() => {
    if (doc) void refreshFiles();
  }, [doc, refreshFiles]);

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
    <div className="min-h-screen bg-background text-foreground">
      <header className="sticky top-0 z-20 flex items-center gap-3 border-b bg-card px-5 py-3">
        <h1 className="m-0 shrink-0 text-base font-medium">{doc?.name || "skill-review"}</h1>
        <code className="min-w-0 truncate text-xs text-muted-foreground">{doc?.path}</code>
        <Button type="button" className="ml-auto shrink-0" onClick={handleSubmitAll}>
          {doc?.updater ? `Submit all to ${doc.updater}` : "Submit all"}
        </Button>
      </header>

      <main className="mx-auto grid max-w-[90rem] grid-cols-1 gap-6 p-6 lg:grid-cols-[minmax(0,1fr)_24rem] lg:items-start">
        <Document
          containerRef={docContainerRef}
          html={doc?.html ?? null}
          threads={doc?.threads ?? []}
          selectedId={selectedId}
          onSelectThread={setSelectedId}
        />

        <aside className="flex flex-col gap-4 lg:sticky lg:top-17 lg:max-h-[calc(100vh-5.5rem)] lg:overflow-y-auto">
          <FileExplorer files={files} filter={filter} onFilterChange={setFilter} />

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
        </aside>
      </main>

      <SelectionMenu menuRef={menuRef} anchorRect={anchorRect} onComment={openComposer} />

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
