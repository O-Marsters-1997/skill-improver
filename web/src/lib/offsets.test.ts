import { GlobalRegistrator } from "@happy-dom/global-registrator";
import { beforeAll, describe, expect, test } from "bun:test";
import { byteLength, edgeOffset, offsetAt } from "./offsets";

beforeAll(() => {
  GlobalRegistrator.register();
});

// Mirrors what internal/render actually emits: every run of source text wrapped in
// <span data-o="N">, N being the byte offset that run starts at (render.go writeSpan).
function renderDoc(html: string) {
  document.body.innerHTML = `<div id="doc">${html}</div>`;
  return document.getElementById("doc")!;
}

describe("byteLength", () => {
  test("counts UTF-8 bytes, not UTF-16 code units", () => {
    // é is one UTF-16 code unit but two UTF-8 bytes — a regression that swaps in
    // .length here would silently mis-anchor every non-ASCII comment.
    expect(byteLength("café")).toBe(5);
    expect("café".length).toBe(4);
  });
});

describe("offsetAt", () => {
  test("start of a span is exactly its data-o", () => {
    const doc = renderDoc('<p><span data-o="0">Hello </span><span data-o="6">world</span></p>');
    const span = doc.querySelector('[data-o="6"]')!;
    const text = span.firstChild!;
    expect(offsetAt(text, 0)).toBe(6);
  });

  test("mid-span offset adds the bytes preceding the caret", () => {
    const doc = renderDoc('<p><span data-o="0">Hello </span><span data-o="6">world</span></p>');
    const span = doc.querySelector('[data-o="6"]')!;
    const text = span.firstChild!;
    expect(offsetAt(text, 3)).toBe(9); // 6 + byteLength("wor")
  });

  test("multi-byte text preceding the caret is counted in bytes", () => {
    const doc = renderDoc('<p><span data-o="0">café </span><span data-o="6">bar</span></p>');
    const span = doc.querySelector('[data-o="0"]')!;
    const text = span.firstChild!;
    // "café " is 5 bytes of "café" (c,a,f,é=2 bytes) + 1 space = 6 bytes total; offset 4
    // JS chars in = "café" (4 UTF-16 units) -> byteLength("café") = 5.
    expect(offsetAt(text, 4)).toBe(5);
  });

  test("returns null when the node has no ancestor span", () => {
    const doc = renderDoc("<p>no spans here</p>");
    const text = doc.querySelector("p")!.firstChild!;
    expect(offsetAt(text, 2)).toBeNull();
  });
});

describe("edgeOffset", () => {
  test("wantEnd=false falls back to the first span's start", () => {
    const doc = renderDoc('<p><span data-o="0">Hello </span><span data-o="6">world</span></p>');
    const range = document.createRange();
    range.selectNodeContents(doc.querySelector("p")!);
    expect(edgeOffset(range, false)).toBe(0);
  });

  test("wantEnd=true falls back to the last span's end", () => {
    const doc = renderDoc('<p><span data-o="0">Hello </span><span data-o="6">world</span></p>');
    const range = document.createRange();
    range.selectNodeContents(doc.querySelector("p")!);
    expect(edgeOffset(range, true)).toBe(11); // 6 + byteLength("world")
  });

  test("returns null when the range contains no spans", () => {
    const doc = renderDoc("<p>no spans here</p>");
    const range = document.createRange();
    range.selectNodeContents(doc.querySelector("p")!);
    expect(edgeOffset(range, false)).toBeNull();
  });
});
