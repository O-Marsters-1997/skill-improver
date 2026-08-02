import { Button } from "@/components/ui/button";

interface HandoffPanelProps {
  open: boolean;
  summary: string;
  prompt: string | null;
  onCopy: () => void;
  onClose: () => void;
}

// Non-modal by design — this reports on a background action, it doesn't gate one. A
// Dialog would trap focus and block the page, which the prompt (worth keeping visible
// while you go do something else) is deliberately not asking for.
export function HandoffPanel({ open, summary, prompt, onCopy, onClose }: HandoffPanelProps) {
  if (!open) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      className="fixed right-5 bottom-20 z-30 max-w-md rounded-lg border border-primary bg-card p-3.5 text-sm shadow-lg"
    >
      <p>{summary}</p>
      {prompt !== null && (
        <pre className="mt-2 [overflow-wrap:anywhere] rounded-md border bg-background p-2 text-xs whitespace-pre-wrap select-all">
          {prompt}
        </pre>
      )}
      <div className="mt-2 flex gap-2">
        {prompt !== null && (
          <Button type="button" size="sm" onClick={onCopy}>
            Copy prompt
          </Button>
        )}
        <Button type="button" size="sm" variant="ghost" onClick={onClose}>
          Close
        </Button>
      </div>
    </div>
  );
}
