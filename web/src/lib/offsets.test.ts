import { GlobalRegistrator } from "@happy-dom/global-registrator";
import { beforeAll, describe, expect, test } from "bun:test";
import { blockAt, byteLength, edgeOffset, offsetAt, sliceBytes } from "./offsets";

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

describe("sliceBytes", () => {
  test("slices by byte offset, not by string index", () => {
    // The bounds the server stamps are byte offsets: "café " is 6 bytes, so the word after
    // it starts at 6. A String.slice(6) here would start one character late.
    const src = "café bar";
    expect(sliceBytes(src, 6, 9)).toBe("bar");
    expect(src.slice(6, 9)).toBe("ar");
  });

  test("round-trips a multi-byte slice whole", () => {
    // é is 2 bytes and the em dash 3, so "café —" spans bytes 2..11 of a 13-character string.
    expect(sliceBytes("a café — here", 2, 11)).toBe("café —");
  });

  test("an empty range is an empty string", () => {
    expect(sliceBytes("some prose", 4, 4)).toBe("");
  });
});

describe("blockAt", () => {
  test("reads the bounds off the nearest stamped ancestor", () => {
    const doc = renderDoc('<p data-os="0" data-oe="11"><span data-o="0">Hello world</span></p>');
    const text = doc.querySelector('[data-o="0"]')!.firstChild!;
    expect(blockAt(text)).toEqual({ start: 0, end: 11 });
  });

  test("the innermost stamped block wins, so a loose list item edits its paragraph", () => {
    const doc = renderDoc(
      '<li data-os="0" data-oe="20"><p data-os="2" data-oe="7"><span data-o="2">inner</span></p></li>',
    );
    const text = doc.querySelector('[data-o="2"]')!.firstChild!;
    expect(blockAt(text)).toEqual({ start: 2, end: 7 });
  });

  test("returns null inside a block the renderer left unstamped", () => {
    const doc = renderDoc('<td><span data-o="2">a</span></td>');
    const text = doc.querySelector('[data-o="2"]')!.firstChild!;
    expect(blockAt(text)).toBeNull();
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
