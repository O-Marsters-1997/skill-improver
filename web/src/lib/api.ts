export class ApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

// GET when body is omitted, POST otherwise — the server tells the two apart the same way
// (see server.go's mux registrations, one GET and one POST per path).
export async function api<T>(path: string, body?: unknown): Promise<T> {
  const init: RequestInit = body
    ? { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) }
    : { method: "GET" };
  const response = await fetch(path, init);
  const payload = await response.json();
  if (!response.ok) {
    throw new ApiError(response.status, payload.error ?? `Request failed (${response.status})`);
  }
  return payload as T;
}
