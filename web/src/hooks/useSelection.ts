import { type RefObject, useCallback, useEffect, useRef, useState } from "react";
import { edgeOffset, offsetAt } from "@/lib/offsets";

export interface PendingComment {
  start: number;
  end: number;
  quote: string;
}

// A selection only offers to become a comment — nothing opens until the floating menu's
// action is chosen, so the document stays readable and selectable. `docRef` must point at
// an always-mounted container (see Document.tsx): if it mounted only once a doc loaded,
// these listeners would attach to nothing.
export function useSelection(docRef: RefObject<HTMLElement | null>, onError: (message: string) => void) {
  const [pending, setPending] = useState<PendingComment | null>(null);
  const [anchorRect, setAnchorRect] = useState<DOMRect | null>(null);
  const [composerOpen, setComposerOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  const hideMenu = useCallback(() => setAnchorRect(null), []);

  const capture = useCallback(() => {
    hideMenu();

    const selection = window.getSelection();
    if (!selection || selection.isCollapsed || selection.rangeCount === 0) return;

    const range = selection.getRangeAt(0);
    if (!docRef.current?.contains(range.commonAncestorContainer)) return;

    const quote = selection.toString().trim();
    if (!quote) return;

    const start = offsetAt(range.startContainer, range.startOffset) ?? edgeOffset(range, false);
    const end = offsetAt(range.endContainer, range.endOffset) ?? edgeOffset(range, true);
    if (start === null || end === null || start >= end) {
      onError("Could not locate that selection in the source. Try selecting whole words.");
      return;
    }

    setPending({ start, end, quote });
    setAnchorRect(range.getBoundingClientRect());
  }, [docRef, hideMenu, onError]);

  const openComposer = useCallback(() => {
    if (!pending) return;
    hideMenu();
    setComposerOpen(true);
  }, [pending, hideMenu]);

  const closeComposer = useCallback(() => {
    setComposerOpen(false);
    setPending(null);
    hideMenu();
  }, [hideMenu]);

  useEffect(() => {
    const container = docRef.current;
    if (!container) return;

    const onMouseUp = () => setTimeout(capture, 0);
    const onKeyUp = (event: KeyboardEvent) => {
      if (event.shiftKey) capture();
    };
    // The menu sits outside #doc, so its own mousedown must not count as clicking away.
    const onMouseDown = (event: MouseEvent) => {
      if (event.target instanceof Node && menuRef.current?.contains(event.target)) return;
      hideMenu();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      hideMenu();
      if (composerOpen) closeComposer();
    };

    container.addEventListener("mouseup", onMouseUp);
    container.addEventListener("keyup", onKeyUp);
    document.addEventListener("mousedown", onMouseDown);
    document.addEventListener("keydown", onKeyDown);
    return () => {
      container.removeEventListener("mouseup", onMouseUp);
      container.removeEventListener("keyup", onKeyUp);
      document.removeEventListener("mousedown", onMouseDown);
      document.removeEventListener("keydown", onKeyDown);
    };
  }, [docRef, capture, hideMenu, composerOpen, closeComposer]);

  return { pending, anchorRect, composerOpen, menuRef, openComposer, closeComposer };
}
