import { type RefObject, useLayoutEffect } from "react";
import { Button } from "@/components/ui/button";

interface SelectionMenuProps {
  menuRef: RefObject<HTMLDivElement | null>;
  anchorRect: DOMRect | null;
  onComment: () => void;
}

function clamp(value: number, low: number, high: number) {
  return Math.min(Math.max(value, low), Math.max(low, high));
}

// Positioned imperatively, same as the original: the anchor is a text Range, not an
// element, so there's no declarative "attach to this node" API to reach for.
export function SelectionMenu({ menuRef, anchorRect, onComment }: SelectionMenuProps) {
  useLayoutEffect(() => {
    const node = menuRef.current;
    if (!node || !anchorRect) return;

    const gap = 8;
    const width = node.offsetWidth;
    const height = node.offsetHeight;
    const left = clamp(anchorRect.left + anchorRect.width / 2 - width / 2, gap, window.innerWidth - width - gap);
    const above = anchorRect.top - height - gap;
    const top = above < gap ? anchorRect.bottom + gap : above;

    node.style.left = `${left + window.scrollX}px`;
    node.style.top = `${top + window.scrollY}px`;
  }, [menuRef, anchorRect]);

  if (!anchorRect) return null;

  return (
    <div
      ref={menuRef}
      role="toolbar"
      aria-label="Selection actions"
      className="absolute z-40 rounded-lg border bg-popover p-1 shadow-lg"
    >
      <Button type="button" size="sm" onClick={onComment} aria-label="Comment on selection">
        💬 Comment
      </Button>
    </div>
  );
}
