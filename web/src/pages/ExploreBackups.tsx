import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom";
import { api } from "../lib/api";
import { asArray } from "../lib/arrays";
import { useTimezone } from "../context/Timezone";
import { formatApplianceDateTime } from "../lib/formatTime";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import VersionLogPanel from "../components/VersionLogPanel";

type Version = {
  id: string;
  status: string;
  bytes: number;
  createdAt: string;
  pathOnDisk: string;
};

type Entry = {
  name?: string;
  path: string;
  isDir: boolean;
  size?: number;
};

type ServerMeta = {
  id: string;
  name: string;
  remoteRoot: string;
};

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export default function ExploreBackups() {
  const { timezone } = useTimezone();
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [server, setServer] = useState<ServerMeta | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [vid, setVid] = useState("");
  const [path, setPath] = useState("");
  const [basePath, setBasePath] = useState("");
  const [remotePath, setRemotePath] = useState("");
  const [entries, setEntries] = useState<Entry[]>([]);
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [q, setQ] = useState("");
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [confirm, setConfirm] = useState("");
  const [showDelete, setShowDelete] = useState(false);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const [panel, setPanel] = useState<"browse" | "log">("browse");
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<{
    files: { path: string; size: number }[];
    totalFiles: number;
    totalBytes: number;
    message: string;
  } | null>(null);
  const [total, setTotal] = useState(0);

  const selectedVersion = useMemo(
    () => versions.find((v) => v.id === vid) ?? null,
    [versions, vid],
  );

  const canBrowse = selectedVersion?.status === "succeeded";

  const loadVersions = async (selectId?: string) => {
    let fs: ServerMeta;
    try {
      fs = await api<ServerMeta>(`/api/file-servers/${id}`);
    } catch {
      navigate("/app/websites", { replace: true });
      return;
    }
    const vs = asArray(await api<Version[] | null>(`/api/file-servers/${id}/versions`));
    setServer(fs);
    setVersions(vs);
    const ok = vs.filter((v) => v.status === "succeeded");
    const want = selectId !== undefined ? selectId : vid;
    if (want && vs.some((v) => v.id === want)) {
      setVid(want);
    } else if (ok[0]) {
      setVid(ok[0].id);
    } else {
      setVid("");
      setPath("");
      setQ("");
      setSelected({});
      setEntries([]);
    }
  };

  const loadTree = async (versionId: string, browsePath: string, query: string) => {
    const params = new URLSearchParams();
    if (query) params.set("q", query);
    else if (browsePath) params.set("path", browsePath);
    const data = await api<{
      mode: string;
      entries: Entry[];
      total?: number;
      path?: string;
      basePath?: string;
      remotePath?: string;
    }>(`/api/file-servers/${id}/versions/${versionId}/tree?${params}`);
    setEntries(data.entries || []);
    if (data.total) setTotal(data.total);
    if (data.mode === "browse") {
      if (data.basePath !== undefined) setBasePath(data.basePath);
      if (data.remotePath) setRemotePath(data.remotePath);
      if (data.path !== undefined) setPath(data.path);
    }
  };

  useEffect(() => {
    const ver = searchParams.get("version") || undefined;
    loadVersions(ver).catch((e) => setError(String(e.message || e)));
  }, [id, searchParams]);

  useEffect(() => {
    if (!vid) return;
    loadTree(vid, path, q).catch((e) => setError(String(e.message || e)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [vid]);

  const selectedPaths = useMemo(
    () => Object.keys(selected).filter((k) => selected[k]),
    [selected],
  );

  const selectVersion = (versionId: string) => {
    const v = versions.find((x) => x.id === versionId);
    setVid(versionId);
    setPath("");
    setBasePath("");
    setRemotePath("");
    setQ("");
    setSelected({});
    setShowDelete(false);
    setDeleteConfirm("");
    if (v && v.status !== "succeeded") {
      setPanel("log");
    }
  };

  const relPath = useMemo(() => {
    if (!path) return "";
    if (basePath) {
      if (path === basePath) return "";
      if (path.startsWith(`${basePath}/`)) return path.slice(basePath.length + 1);
    }
    return path;
  }, [path, basePath]);

  const crumbs = relPath ? relPath.split("/") : [];
  const rootLabel = remotePath || server?.remoteRoot || "root";

  const toggle = (p: string) => setSelected((s) => ({ ...s, [p]: !s[p] }));

  const openDir = (p: string) => {
    setQ("");
    setPath(p);
    if (vid) void loadTree(vid, p, "").catch((e) => setError(String(e.message || e)));
  };

  const openCrumb = (idx: number) => {
    const sub = crumbs.slice(0, idx + 1).join("/");
    const full = basePath ? (sub ? `${basePath}/${sub}` : basePath) : sub;
    openDir(full);
  };

  const search = async (e: FormEvent) => {
    e.preventDefault();
    if (!vid) return;
    setError("");
    try {
      await loadTree(vid, "", q);
    } catch (err) {
      setError(err instanceof Error ? err.message : "search failed");
    }
  };

  const pollJob = async (jobId: string) => {
    for (let i = 0; i < 60; i++) {
      await new Promise((r) => setTimeout(r, 500));
      const job = await api<{ status: string; error: string }>(`/api/jobs/${jobId}`);
      const logs = await api<{ lines: string[] }>(`/api/jobs/${jobId}/logs`);
      if (logs.lines?.length) setInfo(logs.lines.slice(-2).join(" · "));
      if (job.status === "succeeded" || job.status === "failed") {
        setInfo(
          job.status === "succeeded"
            ? `Restore ${job.status}. ${logs.lines?.slice(-1)[0] || ""}`
            : `Restore failed: ${job.error}`,
        );
        return;
      }
    }
  };

  const loadPreview = async () => {
    if (!vid || selectedPaths.length === 0) return;
    setBusy(true);
    setError("");
    try {
      const res = await api<{
        files: { path: string; size: number }[];
        totalFiles: number;
        totalBytes: number;
        message: string;
      }>(`/api/file-servers/${id}/versions/${vid}/restore-preview`, {
        method: "POST",
        body: JSON.stringify({ paths: selectedPaths }),
      });
      setPreview(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : "preview failed");
    } finally {
      setBusy(false);
    }
  };

  const restore = async () => {
    if (!vid || !server) return;
    setBusy(true);
    setError("");
    setInfo("");
    try {
      const res = await api<{ jobId: string }>(
        `/api/file-servers/${id}/versions/${vid}/restore`,
        {
          method: "POST",
          body: JSON.stringify({ paths: selectedPaths, confirmName: confirm }),
        },
      );
      setInfo("Restore started…");
      setPreview(null);
      await pollJob(res.jobId);
    } catch (e) {
      setError(e instanceof Error ? e.message : "restore failed");
    } finally {
      setBusy(false);
    }
  };

  const download = async () => {
    if (!vid || selectedPaths.length === 0) return;
    setBusy(true);
    setError("");
    setInfo("");
    try {
      const res = await fetch(`/api/file-servers/${id}/versions/${vid}/download`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ paths: selectedPaths }),
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => ({}))) as { error?: string };
        throw new Error(data.error || res.statusText);
      }
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `boomerang-${server?.name || "backup"}.zip`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
      setInfo(`Downloaded ${selectedPaths.length} path(s) as zip`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "download failed");
    } finally {
      setBusy(false);
    }
  };

  const verify = async () => {
    if (!vid) return;
    setError("");
    setInfo("");
    setBusy(true);
    try {
      const res = await api<{ jobId: string }>(`/api/file-servers/${id}/versions/${vid}/verify`, {
        method: "POST",
      });
      setInfo(`Verify job ${res.jobId} started`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "verify failed");
    } finally {
      setBusy(false);
    }
  };

  const removeVersion = async () => {
    if (!vid || !server) return;
    setBusy(true);
    setError("");
    setInfo("");
    try {
      await api(`/api/file-servers/${id}/versions/${vid}`, {
        method: "DELETE",
        body: JSON.stringify({ confirmName: deleteConfirm }),
      });
      navigate("/app", { replace: true });
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <Nav />
      <header className="page-head row-head">
        <div>
          <h1>Explore backups</h1>
          <p className="muted">
            {server ? (
              <>
                {server.name} · <code>{server.remoteRoot}</code>
              </>
            ) : (
              "…"
            )}
          </p>
        </div>
        <Link className="ghost btn-link explore-back-btn" to="/app/websites">
          Back to websites
        </Link>
      </header>

      <div className="err">{error}</div>
      {info && <p className="ok pad">{info}</p>}

      <section className="tile explore-main">
        <div className="backup-version-bar">
          {versions.length === 0 ? (
            <p className="muted small">No backups yet.</p>
          ) : (
            <>
              <select
                id="backup-version"
                className="backup-version-select"
                aria-label="Backup version"
                value={vid}
                onChange={(e) => selectVersion(e.target.value)}
              >
                {versions.map((v) => (
                  <option key={v.id} value={v.id}>
                    {formatApplianceDateTime(v.createdAt, timezone)} · {v.status} ·{" "}
                    {fmtBytes(v.bytes)}
                  </option>
                ))}
              </select>
              {selectedVersion && (
                <div className="backup-version-meta">
                  <span className={`pill ${selectedVersion.status}`}>{selectedVersion.status}</span>
                  <span className="muted small backup-version-size">
                    {fmtBytes(selectedVersion.bytes)}
                  </span>
                </div>
              )}
              <div className="backup-version-actions">
                <button
                  type="button"
                  className={`ghost${panel === "browse" ? " active" : ""}`}
                  disabled={!vid || !canBrowse}
                  onClick={() => setPanel("browse")}
                >
                  Browse
                </button>
                <button
                  type="button"
                  className={`ghost${panel === "log" ? " active" : ""}`}
                  disabled={!vid}
                  onClick={() => setPanel("log")}
                >
                  Log
                </button>
                <button
                  type="button"
                  className="ghost"
                  disabled={!vid || busy}
                  onClick={() => void verify()}
                >
                  Verify
                </button>
                <button
                  type="button"
                  className="ghost danger-text"
                  disabled={!vid || busy}
                  onClick={() => {
                    setShowDelete(true);
                    setDeleteConfirm("");
                  }}
                >
                  Delete
                </button>
              </div>
            </>
          )}
        </div>

        {showDelete && vid && (
          <div className="delete-version-box">
            <p className="muted small">
              Permanently delete this backup? Incremental backups that depend on it must be
              removed first.
            </p>
            <label>Type website name ({server?.name}) to confirm</label>
            <input value={deleteConfirm} onChange={(e) => setDeleteConfirm(e.target.value)} />
            <div className="actions">
              <button
                type="button"
                className="danger-text"
                disabled={busy || deleteConfirm !== server?.name}
                onClick={() => void removeVersion()}
              >
                {busy ? "Deleting…" : "Delete backup"}
              </button>
              <button
                type="button"
                className="ghost"
                disabled={busy}
                onClick={() => {
                  setShowDelete(false);
                  setDeleteConfirm("");
                }}
              >
                Cancel
              </button>
            </div>
          </div>
        )}

        {!vid && versions.length === 0 && server && (
          <div className="explore-empty">
            <h2>No backups yet</h2>
            <p className="muted">
              <strong>{server.name}</strong> has never completed a backup, so there is nothing to
              browse here.
            </p>
            <p className="muted small">
              Use <strong>Backup now</strong> on the{" "}
              <Link to="/app/websites">websites</Link> page to run the first backup, or wait
              for the next scheduled run.
            </p>
            <Link className="btn-primary explore-empty-cta" to="/app/websites">
              Go to websites
            </Link>
          </div>
        )}

        {vid && panel === "log" && (
          <VersionLogPanel
            url={`/api/file-servers/${id}/versions/${vid}/logs`}
            title="Backup log"
            tall
          />
        )}

        {vid && panel === "browse" && canBrowse && (
          <>
            <div className="explore-toolbar">
              <div className="crumbs">
                <button type="button" className="ghost crumb" onClick={() => openDir(basePath)}>
                  {rootLabel}
                </button>
                {crumbs.map((c, i) => {
                  const full = crumbs.slice(0, i + 1).join("/");
                  return (
                    <span key={full}>
                      <span className="muted"> / </span>
                      <button type="button" className="ghost crumb" onClick={() => openCrumb(i)}>
                        {c}
                      </button>
                    </span>
                  );
                })}
              </div>
              <form className="search" onSubmit={search}>
                <input
                  placeholder="Search files…"
                  value={q}
                  onChange={(e) => setQ(e.target.value)}
                />
                <button type="submit" className="ghost">
                  Search
                </button>
              </form>
            </div>
            <p className="muted small">{total ? `${total} entries in backup` : ""}</p>

            <div className="file-table">
              {entries.length === 0 && <p className="muted">Empty folder or no matches.</p>}
              {entries.map((e) => (
                <div className="file-row" key={e.path}>
                  <label className="check">
                    <input
                      type="checkbox"
                      checked={!!selected[e.path]}
                      onChange={() => toggle(e.path)}
                    />
                  </label>
                  {e.isDir ? (
                    <button type="button" className="linkish" onClick={() => openDir(e.path)}>
                      [{e.name || e.path}]
                    </button>
                  ) : (
                    <span>
                      {e.name || e.path}
                      {e.size != null && <span className="muted small"> · {fmtBytes(e.size)}</span>}
                    </span>
                  )}
                </div>
              ))}
            </div>

            <div className="restore-box">
              <h2>Restore selected</h2>
              <p className="muted">
                {selectedPaths.length} selected. This overwrites matching files on the live server.
              </p>
              <label>Type website name ({server?.name}) to confirm</label>
              <input value={confirm} onChange={(e) => setConfirm(e.target.value)} />
              <div className="actions">
                <button
                  type="button"
                  disabled={busy || selectedPaths.length === 0}
                  onClick={() => void download()}
                >
                  {busy ? "Working…" : "Download zip"}
                </button>
                <button
                  type="button"
                  className="ghost"
                  disabled={busy || selectedPaths.length === 0}
                  onClick={() => void loadPreview()}
                >
                  Preview restore
                </button>
                <button
                  type="button"
                  disabled={busy || selectedPaths.length === 0 || confirm !== server?.name}
                  onClick={() => void restore()}
                >
                  {busy ? "Restoring…" : "Restore to live server"}
                </button>
                <button type="button" className="ghost" onClick={() => setSelected({})}>
                  Clear selection
                </button>
              </div>
            </div>

            {preview && (
              <div className="tile restore-preview">
                <h3>Restore preview</h3>
                <p className="muted small">{preview.message}</p>
                <p className="muted small">
                  {preview.totalFiles} file(s) · {fmtBytes(preview.totalBytes)}
                </p>
                <ul className="restore-preview-list plain">
                  {(preview.files ?? []).slice(0, 40).map((f) => (
                    <li key={f.path}>
                      <code>{f.path}</code>
                      <span className="muted"> · {fmtBytes(f.size)}</span>
                    </li>
                  ))}
                  {(preview.files ?? []).length > 40 && (
                    <li className="muted">…and {(preview.files ?? []).length - 40} more</li>
                  )}
                </ul>
                <button type="button" className="ghost" onClick={() => setPreview(null)}>
                  Close preview
                </button>
              </div>
            )}
          </>
        )}

        {vid && panel === "browse" && !canBrowse && (
          <p className="muted">
            This backup did not complete successfully. Open the <strong>Log</strong> tab for
            details.
          </p>
        )}
      </section>
      <SiteFooter />
    </div>
  );
}
