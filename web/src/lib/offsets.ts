// Ported verbatim from the previous app.js. This is the client-side half of the property
// internal/render's FuzzOffsets asserts on the server: every rendered span's text is
// exactly the source bytes at the offset it advertises (data-o). Get this wrong and
// comments silently anchor to the wrong passage in the user's SKILL.md.

const encoder = new TextEncoder();

export function byteLength(text: string): number {
  return encoder.encode(text).length;
}

// Every text run the server rendered carries the byte offset it started at, so an offset
// is that number plus the bytes of rendered text preceding the caret.
export function offsetAt(node: Node, offset: number): number | null {
  const element = node.nodeType === Node.TEXT_NODE ? node.parentElement : (node as Element);
  const span = element?.closest("[data-o]");
  if (!span) return null;
  const range = document.createRange();
  range.setStart(span, 0);
  range.setEnd(node, offset);
  return Number((span as HTMLElement).dataset.o) + byteLength(range.toString());
}

// Endpoints can land outside any run — on a highlight boundary, or in whitespace between
// blocks. Fall back to the edge of the nearest run inside the selection; the server
// re-checks the quote and corrects small drifts.
export function edgeOffset(range: Range, wantEnd: boolean): number | null {
  const container = range.commonAncestorContainer as Element;
  const spans = container.querySelectorAll?.("[data-o]") ?? [];
  const span = (wantEnd ? spans[spans.length - 1] : spans[0]) as HTMLElement | undefined;
  if (!span) return null;
  return Number(span.dataset.o) + (wantEnd ? byteLength(span.textContent ?? "") : 0);
}
