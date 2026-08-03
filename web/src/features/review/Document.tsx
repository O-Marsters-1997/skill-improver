import {
  memo,
  type MouseEvent,
  type ReactNode,
  type RefObject,
  useEffect,
  useLayoutEffect,
  useState,
} from "react";
import { createPortal } from "react-dom";
import type { Thread } from "@/lib/types";

interface DocumentProps {
  containerRef: RefObject<HTMLDivElement | null>;
  html: string | null;
  error: string | null;
  threads: Thread[];
  selectedId: string | null;
  onSelectThread: (id: string) => void;
  // The start offset of the block the editor stands in for; the parent owns the draft.
  editingAt: number | null;
  editor: ReactNode;
}

// Memoised on `html` alone, and it has to be: React re-sets innerHTML on every render of this
// element, not only when the string changes, and each reset destroys the subtree — including
// the editor's host node. The click handler lives on the wrapper outside so the default
// shallow compare is enough; events reach it by bubbling.
//
// Rendered markdown comes straight from internal/render; it's safe because render.writeMarkup
// escapes every raw HTML tag that isn't its own mc marker (render_test.go asserts <script>
// never survives).
const RenderedDoc = memo(function RenderedDoc({
  containerRef,
  html,
}: {
  containerRef: RefObject<HTMLDivElement | null>;
  html: string;
}) {
  return (
    <div
      id="doc"
      ref={containerRef}
      aria-label="Skill under review"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
});

// The doc container is always mounted (even before the first fetch resolves) so
// useSelection's event listeners have something to attach to from the very first render —
// see that hook's comment.
export function Document({
  containerRef,
  html,
  error,
  threads,
  selectedId,
  onSelectThread,
  editingAt,
  editor,
}: DocumentProps) {
  const [host, setHost] = useState<HTMLElement | null>(null);

  // The block is server markup React does not own: hiding it and portalling into a sibling
  // saves stashing and restoring innerHTML. Re-runs on `html` to re-host after a redraw.
  useLayoutEffect(() => {
    const container = containerRef.current;
    if (!container || editingAt === null) return;

    const block = container.querySelector<HTMLElement>(`[data-os="${editingAt}"]`);
    if (!block) return;

    const node = document.createElement("div");
    block.style.display = "none";
    block.after(node);
    setHost(node);
    return () => {
      block.style.display = "";
      node.remove();
      setHost(null);
    };
  }, [containerRef, editingAt, html]);

  // The markup is the server's and RenderedDoc rebuilds it whenever `html` changes, so
  // resolved/selected classes have to be reapplied here rather than baked into it.
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
    <div onClick={handleClick}>
      <RenderedDoc containerRef={containerRef} html={html} />
      {/* The fallback matters after a rejected save: the redraw took the host away, and
          dropping the editor would drop the typing with it. */}
      {editor && (host ? createPortal(editor, host) : <div className="mt-4">{editor}</div>)}
    </div>
  );
}
