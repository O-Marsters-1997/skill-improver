// The subset of the Go server's JSON this UI reads — see internal/server/server.go (doc,
// fileEntry), internal/comments/comments.go (Thread, Comment), internal/config/config.go
// (Field), and internal/handoff/handoff.go (Result) for the full shapes.

export interface Comment {
  id: string;
  author: string;
  ts: string;
  body: string;
  deleted?: boolean;
}

// Fields and impact ride flat on the thread (comments.go Thread.MarshalJSON splices them
// in alongside id/quote/status/comments), so a config field named "priority" shows up as
// `thread.priority`, not `thread.fields.priority`. The index signature models that.
export interface Thread {
  id: string;
  quote: string;
  status: "open" | "resolved";
  comments: Comment[];
  [field: string]: unknown;
}

export interface Field {
  name: string;
  label: string;
  values: string[];
  default: string;
}

export interface Doc {
  name: string;
  // The file the server resolved, which is not always the one the URL asked for: "/" means
  // the first file in the review set. Mutations address this, never the URL.
  rel: string;
  path: string;
  rev: string;
  html: string;
  // The source html was rendered from; an in-place edit replaces a byte range of it.
  src: string;
  // Decided by the server, so the page and the write path cannot disagree.
  editable: boolean;
  threads: Thread[];
  fields: Field[];
  updater: string;
}

export interface FileEntry {
  rel: string;
  ext: string;
  threads: number;
}

export type Filter = "markdown" | "all";

// Only the count of suggestions is ever read; the rest of the payload rides straight
// through to the handoff file the server writes.
export interface HandoffResult {
  file: string;
  prompt: string;
  changed: boolean;
  payload: { improvement_suggestions: unknown[] };
}
