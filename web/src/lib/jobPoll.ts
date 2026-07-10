import { api } from "../App";

export type JobPollResult = {
  status: string;
  error?: string;
  lastLines: string[];
};

export async function pollJob(
  jobId: string,
  onProgress: (lines: string[]) => void,
  opts?: { maxAttempts?: number; intervalMs?: number; signal?: AbortSignal },
): Promise<JobPollResult> {
  const maxAttempts = opts?.maxAttempts ?? 120;
  const intervalMs = opts?.intervalMs ?? 500;
  let lastLines: string[] = [];

  for (let i = 0; i < maxAttempts; i++) {
    if (opts?.signal?.aborted) {
      return { status: "cancelled", lastLines };
    }
    await new Promise((r) => setTimeout(r, intervalMs));
    const job = await api<{ status: string; error: string }>(`/api/jobs/${jobId}`);
    const logs = await api<{ lines: string[] }>(`/api/jobs/${jobId}/logs?limit=500`);
    if (logs.lines?.length) {
      lastLines = logs.lines.slice(-5);
      onProgress(lastLines);
    }
    if (
      job.status === "succeeded" ||
      job.status === "failed" ||
      job.status === "cancelled"
    ) {
      return { status: job.status, error: job.error, lastLines };
    }
  }
  return { status: "running", lastLines };
}

export async function cancelJob(jobId: string) {
  await api(`/api/jobs/${jobId}/cancel`, { method: "POST" });
}

export async function downloadDBBackup(databaseId: string, versionId: string) {
  const res = await fetch(`/api/databases/${databaseId}/versions/${versionId}/download`, {
    method: "POST",
    credentials: "include",
  });
  if (!res.ok) {
    const j = await res.json().catch(() => ({}));
    throw new Error((j as { error?: string }).error || "download failed");
  }
  const blob = await res.blob();
  const cd = res.headers.get("Content-Disposition") || "";
  const match = /filename="([^"]+)"/.exec(cd);
  const filename = match?.[1] || `boomerang-db-${versionId.slice(0, 8)}.sql`;
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}
