import { useCallback, useState } from "react";
import { api } from "@/lib/api";
import type { FileEntry } from "@/lib/types";

export function useFiles() {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [filter, setFilter] = useState<"markdown" | "all">("markdown");

  // Returns the fetched array too, not just via setFiles — a caller that awaits refresh()
  // and immediately needs the result (the handoff summary) would otherwise read the stale
  // pre-update `files` from its own render closure.
  const refresh = useCallback(async () => {
    const fresh = await api<FileEntry[]>("/api/files");
    setFiles(fresh);
    return fresh;
  }, []);

  return { files, filter, setFilter, refresh };
}
