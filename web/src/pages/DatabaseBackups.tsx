import { Link, useParams, useSearchParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { asArray } from "../lib/arrays";
import { useTimezone } from "../context/Timezone";
import { formatApplianceDateTime } from "../lib/formatTime";
import { pollJob, downloadDBBackup } from "../lib/jobPoll";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import VersionLogPanel from "../components/VersionLogPanel";
import type { Database } from "./Databases";

type Version = {
  id: string;
  status: string;
  bytes: number;
  createdAt: string;
};

type RestorePreview = {
  tables: { name: string; inBackup: boolean; inLive: boolean }[];
  onlyBackup: string[];
  onlyLive: string[];
  message: string;
};

type RestoreState = {
  db: Database;
  vid: string;
  tables: string[];
  selected: Record<string, boolean>;
  confirm: string;
  preview: RestorePreview | null;
};

type DeleteVersionState = {
  db: Database;
  vid: string;
  confirm: string;
};

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export default function DatabaseBackups() {
  const { id = "" } = useParams();
  const [searchParams] = useSearchParams();
  const { timezone } = useTimezone();
  const [db, setDb] = useState<Database | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [logVersion, setLogVersion] = useState<string | null>(null);
  const [restore, setRestore] = useState<RestoreState | null>(null);
  const [deleteVersion, setDeleteVersion] = useState<DeleteVersionState | null>(null);

  const load = async () => {
    const [d, vs] = await Promise.all([
      api<Database>(`/api/databases/${id}`),
      api<Version[] | null>(`/api/databases/${id}/versions`),
    ]);
    setDb(d);
    setVersions(asArray(vs));
  };

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "load failed"));
  }, [id]);

  const loadRestorePreview = async (state: RestoreState) => {
    const tables = state.tables.filter((t) => state.selected[t]);
    const preview = await api<RestorePreview>(
      `/api/databases/${state.db.id}/versions/${state.vid}/restore-preview`,
      {
        method: "POST",
        body: JSON.stringify({
          confirmName: state.db.name,
          tables: tables.length === state.tables.length ? [] : tables,
        }),
      },
    );
    setRestore((r) => (r ? { ...r, preview } : r));
  };

  const openRestore = async (vid: string) => {
    if (!db) return;
    setError("");
    try {
      const { tables } = await api<{ tables: string[] | null }>(
        `/api/databases/${db.id}/versions/${vid}/tables`,
      );
      const tableList = asArray(tables);
      const selected: Record<string, boolean> = {};
      for (const t of tableList) selected[t] = true;
      const state: RestoreState = { db, vid, tables: tableList, selected, confirm: "", preview: null };
      setRestore(state);
      await loadRestorePreview(state);
    } catch (e) {
      setError(e instanceof Error ? e.message : "could not load tables");
    }
  };

  const toggleTable = async (table: string) => {
    if (!restore) return;
    const next = {
      ...restore,
      selected: { ...restore.selected, [table]: !restore.selected[table] },
      preview: null,
    };
    setRestore(next);
    try {
      await loadRestorePreview(next);
    } catch {
      /* preview optional */
    }
  };

  const runRestore = async () => {
    if (!restore) return;
    if (restore.confirm !== restore.db.name) {
      setError("Type the database name to confirm restore");
      return;
    }
    const tables = restore.tables.filter((t) => restore.selected[t]);
    setBusy(true);
    setError("");
    setInfo("");
    try {
      const res = await api<{ jobId: string }>(
        `/api/databases/${restore.db.id}/versions/${restore.vid}/restore`,
        {
          method: "POST",
          body: JSON.stringify({
            confirmName: restore.db.name,
            tables: tables.length === restore.tables.length ? [] : tables,
          }),
        },
      );
      setRestore(null);
      const result = await pollJob(res.jobId, (lines) => setInfo(lines.join(" · ")), {
        maxAttempts: 90,
        intervalMs: 700,
      });
      setInfo(
        result.status === "succeeded"
          ? `Restore succeeded. ${result.lastLines.slice(-1)[0] || ""}`
          : `Restore failed: ${result.error || result.lastLines.slice(-1)[0] || ""}`,
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "restore failed");
    } finally {
      setBusy(false);
    }
  };

  const runVerify = async (vid: string) => {
    if (!db) return;
    setBusy(true);
    setError("");
    setInfo("");
    try {
      const res = await api<{ jobId: string }>(
        `/api/databases/${db.id}/versions/${vid}/verify`,
        { method: "POST" },
      );
      const result = await pollJob(res.jobId, (lines) => setInfo(lines.join(" · ")));
      setInfo(
        result.status === "succeeded"
          ? "Backup verified OK."
          : `Verify failed: ${result.error || ""}`,
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "verify failed");
    } finally {
      setBusy(false);
    }
  };

  const runDownload = async (vid: string) => {
    if (!db) return;
    setBusy(true);
    setError("");
    try {
      await downloadDBBackup(db.id, vid);
      setInfo("Download started.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "download failed");
    } finally {
      setBusy(false);
    }
  };

  const runDeleteVersion = async () => {
    if (!deleteVersion) return;
    if (deleteVersion.confirm !== deleteVersion.db.name) {
      setError("Type the database name to confirm delete");
      return;
    }
    setBusy(true);
    setError("");
    setInfo("");
    try {
      await api(`/api/databases/${deleteVersion.db.id}/versions/${deleteVersion.vid}`, {
        method: "DELETE",
        body: JSON.stringify({ confirmName: deleteVersion.db.name }),
      });
      setDeleteVersion(null);
      await load();
      setInfo("Backup deleted.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <Nav />
      <header className="page-head">
        <h1>Database backups</h1>
        <p className="muted">
          {db ? (
            <>
              {db.name} · {db.mysqlUser}@{db.mysqlHost}/{db.mysqlDb}
            </>
          ) : (
            "…"
          )}
          {" · "}
          <Link to="/app/databases">Back to databases</Link>
        </p>
      </header>

      {error && <p className="err pad">{error}</p>}
      {info && <p className="ok pad">{info}</p>}

      {restore && (
        <section className="tile restore-modal">
          <h2>Restore {restore.db.name}</h2>
          <p className="muted">
            Leave all tables selected for a full restore, or pick specific tables.
          </p>
          {restore.preview && (
            <div className="tile restore-preview">
              <p className="muted small">{restore.preview.message}</p>
              {(restore.preview.onlyBackup ?? []).length > 0 && (
                <p className="muted small">
                  Only in backup:{" "}
                  <code>{(restore.preview.onlyBackup ?? []).join(", ")}</code>
                </p>
              )}
              {(restore.preview.onlyLive ?? []).length > 0 && (
                <p className="muted small">
                  Only on live DB: <code>{(restore.preview.onlyLive ?? []).join(", ")}</code>
                </p>
              )}
              <ul className="restore-preview-list plain">
                {(restore.preview.tables ?? []).map((row) => (
                  <li key={row.name}>
                    <code>{row.name}</code>
                    <span className="muted small">
                      {row.inBackup ? "backup" : ""}
                      {row.inBackup && row.inLive ? " · " : ""}
                      {row.inLive ? "live" : ""}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          )}
          <div className="table-pick">
            {restore.tables.map((t) => (
              <label key={t} className="check">
                <input
                  type="checkbox"
                  checked={!!restore.selected[t]}
                  onChange={() => void toggleTable(t)}
                />
                <code>{t}</code>
              </label>
            ))}
          </div>
          <label>Type database name ({restore.db.name}) to confirm</label>
          <input
            value={restore.confirm}
            onChange={(e) => setRestore((r) => (r ? { ...r, confirm: e.target.value } : r))}
          />
          <div className="actions">
            <button
              type="button"
              disabled={busy || restore.confirm !== restore.db.name}
              onClick={() => void runRestore()}
            >
              {busy ? "Restoring…" : "Restore"}
            </button>
            <button type="button" className="ghost" disabled={busy} onClick={() => setRestore(null)}>
              Cancel
            </button>
          </div>
        </section>
      )}

      {deleteVersion && (
        <section className="tile restore-modal">
          <h2>Delete backup — {deleteVersion.db.name}</h2>
          <p className="muted">Permanently remove this backup from disk. This cannot be undone.</p>
          <label>Type database name ({deleteVersion.db.name}) to confirm</label>
          <input
            value={deleteVersion.confirm}
            onChange={(e) =>
              setDeleteVersion((d) => (d ? { ...d, confirm: e.target.value } : d))
            }
          />
          <div className="actions">
            <button
              type="button"
              className="danger-text"
              disabled={busy || deleteVersion.confirm !== deleteVersion.db.name}
              onClick={() => void runDeleteVersion()}
            >
              {busy ? "Deleting…" : "Delete backup"}
            </button>
            <button
              type="button"
              className="ghost"
              disabled={busy}
              onClick={() => setDeleteVersion(null)}
            >
              Cancel
            </button>
          </div>
        </section>
      )}

      <section className="tile">
        {versions.length === 0 ? (
          <p className="muted">
            No backups yet. Run <strong>Backup now</strong> from the{" "}
            <Link to="/app/databases">databases</Link> page.
          </p>
        ) : (
          <ul className="list list-stack">
            {versions.map((v) => (
              <li
                key={v.id}
                className={
                  searchParams.get("version") === v.id ? "backup-version-row dash-highlight" : "backup-version-row"
                }
              >
                <div className="list-main">
                  <strong>{formatApplianceDateTime(v.createdAt, timezone)}</strong>
                  <span className="muted small">
                    {" "}
                    · <span className={`pill ${v.status}`}>{v.status}</span> · {fmtBytes(v.bytes)}
                  </span>
                </div>
                <div className="list-actions">
                  <button
                    type="button"
                    className="ghost"
                    disabled={busy}
                    onClick={() => setLogVersion(logVersion === v.id ? null : v.id)}
                  >
                    Log
                  </button>
                  {v.status === "succeeded" && (
                    <>
                      <button
                        type="button"
                        className="ghost"
                        disabled={busy}
                        onClick={() => void runVerify(v.id)}
                      >
                        Verify
                      </button>
                      <button
                        type="button"
                        className="ghost"
                        disabled={busy}
                        onClick={() => void runDownload(v.id)}
                      >
                        Download
                      </button>
                      <button
                        type="button"
                        className="ghost"
                        disabled={busy}
                        onClick={() => void openRestore(v.id)}
                      >
                        Restore
                      </button>
                    </>
                  )}
                  <button
                    type="button"
                    className="ghost danger-text"
                    disabled={busy}
                    onClick={() => db && setDeleteVersion({ db, vid: v.id, confirm: "" })}
                  >
                    Delete
                  </button>
                </div>
                {logVersion === v.id && (
                  <div className="version-card-log list-full-width">
                    <VersionLogPanel
                      url={`/api/databases/${id}/versions/${v.id}/logs`}
                      title={`Backup log — ${db?.name || ""}`}
                      onClose={() => setLogVersion(null)}
                    />
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>
      <SiteFooter />
    </div>
  );
}
