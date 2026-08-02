import { useEffect, useRef } from "react";
import { ThreadCard } from "./ThreadCard";
import type { Field, Thread } from "@/lib/types";

interface ThreadListProps {
  threads: Thread[];
  fields: Field[];
  selectedId: string | null;
  onReply: (id: string, body: string) => void;
  onToggleStatus: (id: string) => void;
  onDelete: (id: string) => void;
  onFieldChange: (id: string, field: string, value: string) => void;
}

export function ThreadList({ threads, fields, selectedId, onReply, onToggleStatus, onDelete, onFieldChange }: ThreadListProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!selectedId) return;
    containerRef.current?.querySelector(`[data-id="${selectedId}"]`)?.scrollIntoView({ block: "nearest" });
  }, [selectedId]);

  if (threads.length === 0) {
    return <p className="text-sm text-muted-foreground">Highlight a passage in the document, then choose Comment.</p>;
  }

  return (
    <div ref={containerRef} className="flex flex-col gap-3">
      {threads.map((thread) => (
        <ThreadCard
          key={thread.id}
          thread={thread}
          fields={fields}
          selected={thread.id === selectedId}
          onReply={(body) => onReply(thread.id, body)}
          onToggleStatus={() => onToggleStatus(thread.id)}
          onDelete={() => onDelete(thread.id)}
          onFieldChange={(field, value) => onFieldChange(thread.id, field, value)}
        />
      ))}
    </div>
  );
}
