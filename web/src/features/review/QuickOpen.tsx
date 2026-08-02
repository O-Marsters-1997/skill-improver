import { FileText } from "lucide-react";
import { type KeyboardEvent, type MouseEvent, useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";
import type { FileEntry } from "@/lib/types";

interface QuickOpenProps {
  files: FileEntry[];
  onClose: () => void;
  onSelect: (rel: string) => void;
}

const LIMIT = 50;

// A native <dialog> rather than a Dialog component: showModal() already gives the focus
// trap, the inert background, the backdrop and Esc-to-dismiss that a quick opener needs.
export function QuickOpen({ files, onClose, onSelect }: QuickOpenProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const [query, setQuery] = useState("");
  const [cursor, setCursor] = useState(0);

  useEffect(() => ref.current?.showModal(), []);

  const needle = query.toLowerCase();
  const matches = files.filter((file) => file.rel.toLowerCase().includes(needle)).slice(0, LIMIT);
  const active = Math.min(cursor, matches.length - 1);

  function choose(rel: string) {
    onSelect(rel);
    onClose();
  }

  function handleKeyDown(event: KeyboardEvent<HTMLDivElement>) {
    const move = (to: number) => setCursor(Math.max(0, Math.min(to, matches.length - 1)));
    switch (event.key) {
      case "ArrowDown":
        move(active + 1);
        break;
      case "ArrowUp":
        move(active - 1);
        break;
      case "Enter": {
        const match = matches[active];
        if (match) choose(match.rel);
        break;
      }
      default:
        return;
    }
    event.preventDefault();
  }

  // showModal() stretches the backdrop over the whole viewport, so a click that lands on the
  // dialog element itself (and not on its content) landed on the backdrop.
  function handleClick(event: MouseEvent<HTMLDialogElement>) {
    if (event.target === ref.current) onClose();
  }

  return (
    <dialog
      ref={ref}
      onClose={onClose}
      onClick={handleClick}
      aria-label="Open file"
      className="fixed top-[15vh] m-0 w-[min(34rem,calc(100vw-2rem))] -translate-x-1/2 left-1/2 rounded-lg border bg-card p-0 text-foreground shadow-lg backdrop:bg-black/40"
    >
      <div onKeyDown={handleKeyDown}>
        <input
          autoFocus
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setCursor(0);
          }}
          placeholder="Search files…"
          aria-label="Search files"
          className="w-full border-b bg-transparent px-3.5 py-3 text-sm outline-none placeholder:text-muted-foreground"
        />

        {matches.length === 0 ? (
          <p className="px-3.5 py-3 text-sm text-muted-foreground">No files match.</p>
        ) : (
          <ul className="max-h-80 list-none overflow-y-auto p-1">
            {matches.map((file, index) => (
              <li key={file.rel}>
                <button
                  type="button"
                  onClick={() => choose(file.rel)}
                  onMouseMove={() => setCursor(index)}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-xs",
                    index === active && "bg-sidebar-accent",
                  )}
                >
                  <FileText aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
                  <span className="truncate">{file.rel}</span>
                  {file.threads > 0 && (
                    <span className="ml-auto shrink-0 rounded-full bg-sidebar-accent px-1.5 text-[0.6875rem] tabular-nums text-muted-foreground">
                      {file.threads}
                    </span>
                  )}
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </dialog>
  );
}
