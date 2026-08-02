import { X } from "lucide-react";
import { useEffect } from "react";
import { Button } from "@/components/ui/button";

interface HandoffPanelProps {
  summary: string;
  prompt: string | null;
  onCopy: () => void;
  onClose: () => void;
}

// Non-modal by design — this reports on a background action, it doesn't gate one. A
// Dialog would trap focus and block the page, which the prompt (worth keeping visible
// while you go do something else) is deliberately not asking for. Esc is wired by hand
// for the same reason: no dialog element, so nothing hands it to us.
export function HandoffPanel({ summary, prompt, onCopy, onClose }: HandoffPanelProps) {
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  return (
    <div
      role="status"
      aria-live="polite"
      className="fixed right-5 bottom-20 z-30 max-w-md rounded-lg border border-primary bg-card p-3.5 text-sm shadow-lg"
    >
      <div className="flex items-start gap-3">
        <p className="min-w-0 flex-1">{summary}</p>
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          aria-label="Close (Esc)"
          title="Close (Esc)"
          onClick={onClose}
        >
          <X />
        </Button>
      </div>
      {prompt !== null && (
        <>
          <pre className="mt-2 [overflow-wrap:anywhere] rounded-md border bg-background p-2 text-xs whitespace-pre-wrap select-all">
            {prompt}
          </pre>
          <Button type="button" size="sm" className="mt-2" onClick={onCopy}>
            Copy prompt
          </Button>
        </>
      )}
    </div>
  );
}
