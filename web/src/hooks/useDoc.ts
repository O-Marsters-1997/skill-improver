import { useCallback, useEffect, useRef, useState } from "react";
import { ApiError, api } from "@/lib/api";
import type { Doc } from "@/lib/types";

// Owns the one server resource this app has. Every mutation POSTs {rev, ...body} and
// redraws straight from the response, so there is nothing to cache or invalidate — the
// server is always asked to render the next state, never the client.
export function useDoc() {
  const [doc, setDoc] = useState<Doc | null>(null);
  const docRef = useRef<Doc | null>(null);
  docRef.current = doc;

  const refresh = useCallback(async () => {
    setDoc(await api<Doc>("/api/doc"));
  }, []);

  // A 409 means the file changed on disk since `rev` was read. Reloading and surfacing one
  // friendly message is the server's own contract (see server.go's `mutate`) — every
  // caller gets it for free instead of re-implementing the conflict check.
  const mutate = useCallback(
    async (path: string, body: Record<string, unknown>) => {
      try {
        setDoc(await api<Doc>(path, { rev: docRef.current?.rev ?? "", ...body }));
      } catch (err) {
        if (err instanceof ApiError && err.status === 409) {
          await refresh();
          throw new Error("The file changed on disk, so the page reloaded. Nothing was lost — try again.");
        }
        throw err;
      }
    },
    [refresh],
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { doc, refresh, mutate };
}
