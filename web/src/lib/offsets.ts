// Ported verbatim from the previous app.js. This is the client-side half of the property
// internal/render's FuzzOffsets asserts on the server: every rendered span's text is
// exactly the source bytes at the offset it advertises (data-o). Get this wrong and
// comments silently anchor to the wrong passage in the user's SKILL.md.

const encoder = new TextEncoder();
const decoder = new TextDecoder();

export function byteLength(text: string): number {
  return encoder.encode(text).length;
}

// Offsets are UTF-8 byte positions and a JS string index is UTF-16, so the slice has to be
// taken in the encoded form.
export function sliceBytes(src: string, start: number, end: number): string {
  return decoder.decode(encoder.encode(src).subarray(start, end));
}

function elementOf(node: Node): Element | null {
  return node.nodeType === Node.TEXT_NODE ? node.parentElement : (node as Element);
}

// The byte range an in-place edit may replace. Null where the renderer stamped no bounds,
// such as a table cell, which is where editing is not offered.
export function blockAt(node: Node): { start: number; end: number } | null {
  const block = elementOf(node)?.closest<HTMLElement>("[data-os]");
  if (!block) return null;
  return { start: Number(block.dataset.os), end: Number(block.dataset.oe) };
}

// Every text run the server rendered carries the byte offset it started at, so an offset
// is that number plus the bytes of rendered text preceding the caret.
export function offsetAt(node: Node, offset: number): number | null {
  const span = elementOf(node)?.closest("[data-o]");
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
