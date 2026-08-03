import { useEffect, useRef } from "react";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

interface BlockEditorProps {
  text: string;
  onChange: (text: string) => void;
  onCancel: () => void;
  onSave: () => void;
}

// The textarea holds Markdown source, markers and all: the server applies the edit as a
// byte splice, so anything prettier would misrepresent what is being changed.
export function BlockEditor({ text, onChange, onCancel, onSave }: BlockEditorProps) {
  const formRef = useRef<HTMLFormElement>(null);

  // The comment composer's shortcut. Escape is handled here rather than in useSelection
  // because this editor outlives the selection that opened it.
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if ((event.metaKey || event.ctrlKey) && event.key === "Enter") {
        formRef.current?.requestSubmit();
      }
      if (event.key === "Escape") onCancel();
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onCancel]);

  return (
    <form
      ref={formRef}
      onSubmit={(event) => {
        event.preventDefault();
        onSave();
      }}
      aria-label="Edit block source"
      className="not-prose my-2 flex flex-col gap-2 rounded-lg border border-primary bg-card p-3 shadow-xs"
    >
      <Textarea
        autoFocus
        required
        spellCheck
        rows={Math.min(text.split("\n").length + 1, 20)}
        className="font-mono text-sm"
        value={text}
        onChange={(event) => onChange(event.target.value)}
      />
      <div className="flex items-center gap-2">
        <Button type="submit" size="sm">
          Save
        </Button>
        <Button type="button" size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
        <span className="ml-auto text-xs text-muted-foreground">⌘↵ to save · esc to cancel</span>
      </div>
    </form>
  );
}
