import { type MouseEvent, type RefObject, useEffect } from "react";
import type { Thread } from "@/lib/types";

interface DocumentProps {
  containerRef: RefObject<HTMLDivElement | null>;
  html: string | null;
  error: string | null;
  threads: Thread[];
  selectedId: string | null;
  onSelectThread: (id: string) => void;
}

// The doc container is always mounted (even before the first fetch resolves) so
// useSelection's event listeners have something to attach to from the very first render —
// see that hook's comment. Rendered markdown comes straight from internal/render via
// dangerouslySetInnerHTML; it's safe because render.writeMarkup escapes every raw HTML
// tag that isn't its own mc marker (render_test.go asserts <script> never survives).
export function Document({ containerRef, html, error, threads, selectedId, onSelectThread }: DocumentProps) {
  // React only re-renders this subtree when `html` changes (dangerouslySetInnerHTML skips
  // the DOM write when the string is unchanged), so resolved/selected classes have to be
  // reapplied here rather than baked into the server's markup.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    for (const node of container.querySelectorAll<HTMLElement>(".mc")) {
      const id = node.dataset.id;
      node.classList.toggle("resolved", threads.some((t) => t.id === id && t.status === "resolved"));
      node.classList.toggle("selected", id === selectedId);
    }
  }, [containerRef, html, threads, selectedId]);

  function handleClick(event: MouseEvent<HTMLDivElement>) {
    const mark = (event.target as HTMLElement).closest<HTMLElement>(".mc");
    if (mark?.dataset.id) onSelectThread(mark.dataset.id);
  }

  if (html === null) {
    return (
      <div id="doc" ref={containerRef}>
        {error ? (
          <>
            <p className="m-0 font-medium">{error}</p>
            <p className="mt-2 mb-0 text-sm text-muted-foreground">
              Pick a file from the explorer, or <a href="/">go back to the first one</a>.
            </p>
          </>
        ) : (
          <p className="m-0 text-muted-foreground">Loading…</p>
        )}
      </div>
    );
  }

  return (
    <div
      id="doc"
      ref={containerRef}
      aria-label="Skill under review"
      onClick={handleClick}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
