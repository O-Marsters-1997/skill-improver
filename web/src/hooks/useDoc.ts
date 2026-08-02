import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, api } from "@/lib/api";
import type { Doc } from "@/lib/types";

function docPath(rel: string) {
  return rel ? `/api/doc?file=${rel.split("/").map(encodeURIComponent).join("/")}` : "/api/doc";
}

// Owns the one server resource this app has. Every mutation POSTs {file, rev, ...body} and
// redraws straight from the response, so there is nothing to cache or invalidate — the
// server is always asked to render the next state, never the client.
export function useDoc(rel: string) {
  const [doc, setDoc] = useState<Doc | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Clicking through the tree faster than the server answers would otherwise let an earlier
  // response land last and show the wrong file.
  const latest = useRef(0);

  const refresh = useCallback(async (path: string) => {
    const request = ++latest.current;
    try {
      const next = await api<Doc>(docPath(path));
      if (request !== latest.current) return;
      setDoc(next);
      setError(null);
    } catch (err) {
      if (request !== latest.current) return;
      setDoc(null);
      // A 400 here is only ever one thing — a URL naming a file the review set does not
      // hold. The server's own wording is for the API, not for the person reading it.
      const notInSet = err instanceof ApiError && err.status === 400;
      setError(
        notInSet
          ? `“${path}” is not one of the files under review.`
          : err instanceof Error
            ? err.message
            : "Could not load that file.",
      );
    }
  }, []);

  useEffect(() => {
    void refresh(rel);
  }, [rel, refresh]);

  // A 409 means the file changed on disk since `rev` was read. Reloading and surfacing one
  // friendly message is the server's own contract (see server.go's `mutate`) — every
  // caller gets it for free instead of re-implementing the conflict check.
  async function mutate(path: string, body: Record<string, unknown>) {
    // doc.rel, not the URL: it is the file the server actually resolved, so a mutation from
    // "/" edits the file on screen rather than defaulting a second time.
    const file = doc?.rel ?? "";
    try {
      setDoc(await api<Doc>(path, { file, rev: doc?.rev ?? "", ...body }));
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        await refresh(rel);
        throw new Error("The file changed on disk, so the page reloaded. Nothing was lost — try again.");
      }
      throw err;
    }
  }

  return { doc, error, mutate };
}
