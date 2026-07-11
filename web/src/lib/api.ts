let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(fn: (() => void) | null) {
  onUnauthorized = fn;
}

async function parseResponse<T>(res: Response): Promise<T> {
  const text = await res.text();
  if (!text) {
    return {} as T;
  }
  const contentType = res.headers.get("Content-Type") || "";
  if (contentType.includes("application/json") || text.startsWith("{") || text.startsWith("[")) {
    try {
      return JSON.parse(text) as T;
    } catch {
      throw new Error(res.statusText || "invalid JSON response");
    }
  }
  throw new Error(text || res.statusText || "request failed");
}

export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    ...init,
  });
  const data = await parseResponse<{ error?: string } & T>(res);
  if (res.status === 401) {
    onUnauthorized?.();
  }
  if (!res.ok) {
    throw new Error(data.error || res.statusText);
  }
  return data as T;
}
