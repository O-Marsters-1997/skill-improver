import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import type { Field, Thread } from "@/lib/types";

interface ThreadCardProps {
  thread: Thread;
  fields: Field[];
  selected: boolean;
  onReply: (body: string) => void;
  onToggleStatus: () => void;
  onDelete: () => void;
  onFieldChange: (field: string, value: string) => void;
}

export function ThreadCard({ thread, fields, selected, onReply, onToggleStatus, onDelete, onFieldChange }: ThreadCardProps) {
  const [reply, setReply] = useState("");
  const resolved = thread.status === "resolved";

  function handleReply() {
    const trimmed = reply.trim();
    if (!trimmed) return;
    onReply(trimmed);
    setReply("");
  }

  return (
    <Card
      data-id={thread.id}
      className={cn("gap-2 p-4", resolved && "opacity-60", selected && "ring-2 ring-primary")}
    >
      <blockquote className="rounded-md border-l-3 border-highlight-open bg-muted px-2.5 py-1.5 text-sm text-muted-foreground">
        {thread.quote}
      </blockquote>

      {thread.comments.filter((c) => !c.deleted).map((comment) => (
        <p key={comment.id} className="text-sm whitespace-pre-wrap">
          <span className="block text-xs text-muted-foreground">
            {comment.author} · {comment.ts}
          </span>
          {comment.body}
        </p>
      ))}

      {fields.length > 0 && (
        <div className="flex items-end gap-2">
          {fields.map((field) => (
            <div key={field.name} className="flex-1">
              <Label htmlFor={`${thread.id}-${field.name}`} className="text-xs text-muted-foreground">
                {field.label}
              </Label>
              <Select
                value={String(thread[field.name] ?? field.default)}
                onValueChange={(value) => onFieldChange(field.name, value as string)}
              >
                <SelectTrigger id={`${thread.id}-${field.name}`} className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {field.values.map((value) => (
                    <SelectItem key={value} value={value}>
                      {value}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          ))}
        </div>
      )}

      <Textarea
        rows={2}
        placeholder="Reply…"
        value={reply}
        onChange={(event) => setReply(event.target.value)}
      />

      <div className="flex gap-2">
        <Button type="button" size="sm" onClick={handleReply}>
          Reply
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onToggleStatus}>
          {resolved ? "Reopen" : "Resolve"}
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onDelete}>
          Delete
        </Button>
      </div>
    </Card>
  );
}
