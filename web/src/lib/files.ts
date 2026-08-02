import type { FileEntry } from "./types";

const MARKDOWN_EXTS = new Set([".md", ".markdown"]);

// filter is client-only and never rides in a request — the count Submit reports has to
// stay the server's, not a count of whatever the explorer currently happens to show.
export function isShown(file: FileEntry, filter: "markdown" | "all"): boolean {
  return filter === "all" || MARKDOWN_EXTS.has(file.ext);
}

// The names of files the current filter hides but that still carry threads Submit would
// ship — so a reviewer filtering to Markdown isn't surprised by what Submit hands off.
export function hiddenFileNames(files: FileEntry[], filter: "markdown" | "all"): string {
  return files
    .filter((file) => !isShown(file, filter) && file.threads > 0)
    .map((file) => file.rel)
    .join(", ");
}
