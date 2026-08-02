// Mirrors the JSON the Go server actually emits — see internal/server/server.go (doc,
// fileEntry), internal/comments/comments.go (Thread, Comment), internal/config/config.go
// (Field), and internal/handoff/{handoff,pending}.go (Suggestion, Payload, Result).

export interface Comment {
  id: string;
  parent?: string;
  author: string;
  ts: string;
  body: string;
  editedTs?: string;
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
  impact?: string;
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
  path: string;
  rev: string;
  html: string;
  threads: Thread[];
  fields: Field[];
  updater: string;
}

export interface FileEntry {
  rel: string;
  ext: string;
  threads: number;
}

export interface Suggestion {
  id: string;
  file: string;
  suggestion: string;
  expected_impact: string;
  [field: string]: unknown;
}

export interface Payload {
  skill_name: string;
  skill_path: string;
  mode: string;
  improvement_suggestions: Suggestion[];
}

export interface HandoffResult {
  file: string;
  prompt: string;
  changed: boolean;
  payload: Payload;
}
