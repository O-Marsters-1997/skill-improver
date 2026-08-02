import { type FormEvent, useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

interface ComposerProps {
  quote: string;
  onCancel: () => void;
  onSubmit: (body: string) => Promise<void>;
}

export function Composer({ quote, onCancel, onSubmit }: ComposerProps) {
  const [body, setBody] = useState("");
  const formRef = useRef<HTMLFormElement>(null);

  // Escape is handled by useSelection (it owns menu + composer visibility); this only
  // owns the form's own submit shortcut.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
        formRef.current?.requestSubmit();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  // Left unset on failure so the user's draft survives a retry — closeComposer (called by
  // the parent only on success) is what resets this component by unmounting it.
  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const trimmed = body.trim();
    if (!trimmed) return;
    await onSubmit(trimmed);
  }

  return (
    <form
      ref={formRef}
      onSubmit={handleSubmit}
      className="flex flex-col gap-2 rounded-lg border border-primary bg-card p-4 shadow-xs"
    >
      <h2 className="text-sm font-medium">New comment</h2>
      <blockquote className="max-h-24 overflow-hidden rounded-md border-l-3 border-highlight-open bg-muted px-2.5 py-1.5 text-sm text-muted-foreground">
        {quote}
      </blockquote>

      <Label htmlFor="composer-body">Comment</Label>
      <Textarea
        id="composer-body"
        rows={4}
        autoFocus
        required
        placeholder="What should change, and why?"
        value={body}
        onChange={(event) => setBody(event.target.value)}
      />

      <div className="flex gap-2">
        <Button type="submit">Save comment</Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
    </form>
  );
}
