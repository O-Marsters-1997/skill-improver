import { toast } from "@/components/ui/toast";

// Errors stay visible longer (12s) than confirmations (5s) — there's more to read, and
// nothing else on the page repeats the failure.
export function say(message: string, isError = false) {
  toast.add({
    title: message,
    type: isError ? "error" : "success",
    timeout: isError ? 12000 : 5000,
    priority: isError ? "high" : "low",
  });
}

// Every mutation in this app follows the same shape: try the request, and if it fails,
// say why instead of leaving the user looking at nothing.
export async function run(work: () => Promise<void>) {
  try {
    await work();
  } catch (err) {
    say(err instanceof Error ? err.message : String(err), true);
  }
}
