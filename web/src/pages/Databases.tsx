import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { api } from "../App";
import { useTimezone } from "../context/Timezone";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import VersionLogPanel from "../components/VersionLogPanel";
import { retentionSummary } from "../components/ScheduleRetention";
import { describeSchedule, parseSchedule } from "../lib/schedule";

export type Database = {
  id: string;
  name: string;
  mysqlHost: string;
  mysqlPort: number;
  mysqlDb: string;
  mysqlUser: string;
  includeTables: string[];
  tunnelMode: string;
  fileServerId: string | null;
  sshHost: string;
  sshPort: number;
  sshUsername: string;
  authMode: string;
  scheduleCron: string;
  scheduleStart: string;
  retainCount: number;
  retainDays: number;
  retainHourly: number;
  retainDaily: number;
  retainWeekly: number;
  retainMonthly: number;
  retainYearly: number;
  enabled: boolean;
};

type Version = {
  id: string;
  status: string;
  bytes: number;
  createdAt: string;
};

type RestoreState = {
  db: Database;
  vid: string;
  tables: string[];
  selected: Record<string, boolean>;
  confirm: string;
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

export default function Databases() {
  const { timezone } = useTimezone();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [list, setList] = useState<Database[]>([]);
  const [versions, setVersions] = useState<Record<string, Version[]>>({});
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [restore, setRestore] = useState<RestoreState | null>(null);
  const [deleteVersion, setDeleteVersion] = useState<DeleteVersionState | null>(null);
  const [logVersion, setLogVersion] = useState<{ dbId: string; vid: string } | null>(null);

  const load = async () => {
    const dbs = await api<Database[]>("/api/databases");
    setList(dbs);
    const verMap: Record<string, Version[]> = {};
    await Promise.all(
      dbs.map(async (d) => {
        verMap[d.id] = await api<Version[]>(`/api/databases/${d.id}/versions`);
      }),
    );
    setVersions(verMap);
  };

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "load failed"));
  }, []);

  useEffect(() => {
    const dbId = searchParams.get("db");
    if (!dbId || list.length === 0) return;
    const el = document.getElementById(`db-${dbId}`);
    if (el) {
      el.scrollIntoView({ behavior: "smooth", block: "nearest" });
      el.classList.add("dash-highlight");
      const t = window.setTimeout(() => el.classList.remove("dash-highlight"), 2500);
      return () => window.clearTimeout(t);
    }
  }, [list, searchParams]);

  const remove = async (id: string) => {
    if (!confirm("Delete this database target?")) return;
    await api(`/api/databases/${id}`, { method: "DELETE" });
    await load();
  };

  const backupNow = async (id: string) => {
    setError("");
    setInfo("");
    try {
      const res = await api<{ jobId: string }>(`/api/databases/${id}/backup`, { method: "POST" });
      setInfo(`Backup started (job ${res.jobId}).`);
      for (let i = 0; i < 60; i++) {
        await new Promise((r) => setTimeout(r, 700));
        const job = await api<{ status: string; error: string }>(`/api/jobs/${res.jobId}`);
        const logs = await api<{ lines: string[] }>(`/api/jobs/${res.jobId}/logs`);
        if (logs.lines?.length) setInfo(logs.lines.slice(-2).join(" · "));
        if (job.status === "succeeded" || job.status === "failed") {
          setInfo(
            job.status === "succeeded"
              ? `Backup succeeded. ${logs.lines?.slice(-1)[0] || ""}`
              : `Backup failed: ${job.error}`,
          );
          await load();
          break;
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "backup failed");
    }
  };

  const openRestore = async (db: Database, vid: string) => {
    setError("");
    try {
      const { tables } = await api<{ tables: string[] }>(
        `/api/databases/${db.id}/versions/${vid}/tables`,
      );
      const selected: Record<string, boolean> = {};
      for (const t of tables) selected[t] = true;
      setRestore({ db, vid, tables, selected, confirm: "" });
    } catch (e) {
      setError(e instanceof Error ? e.message : "could not load tables");
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
      setInfo("Restore started…");
      for (let i = 0; i < 90; i++) {
        await new Promise((r) => setTimeout(r, 700));
        const job = await api<{ status: string; error: string }>(`/api/jobs/${res.jobId}`);
        const logs = await api<{ lines: string[] }>(`/api/jobs/${res.jobId}/logs`);
        if (logs.lines?.length) setInfo(logs.lines.slice(-2).join(" · "));
        if (job.status === "succeeded" || job.status === "failed") {
          setInfo(
            job.status === "succeeded"
              ? `Restore succeeded. ${logs.lines?.slice(-1)[0] || ""}`
              : `Restore failed: ${job.error}`,
          );
          break;
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "restore failed");
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
          <h1>Databases</h1>
          <p className="muted">Configured MySQL backup targets</p>
        </div>
        <Link className="btn-primary" to="/app/databases/new">
          Add database
        </Link>
      </header>

      {error && <p className="err pad">{error}</p>}
      {info && <p className="ok pad">{info}</p>}

      {restore && (
        <section className="tile restore-modal">
          <h2>Restore {restore.db.name}</h2>
          <p className="muted">
            Leave all tables selected for a full restore, or pick specific tables (e.g. WordPress
            content without sessions).
          </p>
          <div className="table-pick">
            {restore.tables.map((t) => (
              <label key={t} className="check">
                <input
                  type="checkbox"
                  checked={!!restore.selected[t]}
                  onChange={() =>
                    setRestore((r) =>
                      r ? { ...r, selected: { ...r.selected, [t]: !r.selected[t] } } : r,
                    )
                  }
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
          <p className="muted">
            Permanently remove this backup from disk. This cannot be undone.
          </p>
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
        {list.length === 0 && (
          <div className="empty-state">
            <p className="muted">No databases yet.</p>
            <Link className="btn-primary" to="/app/databases/new">
              Add your first database
            </Link>
          </div>
        )}
        <ul className="list list-stack">
          {list.map((d) => {
            const sched = parseSchedule(d.scheduleCron, d.scheduleStart || "", timezone);
            return (
              <li key={d.id} id={`db-${d.id}`}>
                <div className="list-main">
                  <strong>{d.name}</strong>
                  {!d.enabled && <span className="badge">disabled</span>}
                  <span className="muted">
                    {" "}
                    · {d.mysqlUser}@{d.mysqlHost}/{d.mysqlDb} · tunnel {d.tunnelMode}
                  </span>
                  <div className="muted small">
                    {describeSchedule(sched)} · {retentionSummary(d, sched)}
                    {d.includeTables?.length
                      ? ` · ${d.includeTables.length} table(s)`
                      : " · all tables"}
                  </div>
                  {(versions[d.id] || []).length > 0 && (
                    <div className="version-actions">
                      {(versions[d.id] || []).slice(0, 8).map((v) => (
                        <div
                          key={v.id}
                          className={
                            searchParams.get("version") === v.id
                              ? "version-card dash-highlight"
                              : "version-card"
                          }
                        >
                          <div className="version-card-body">
                            <div className="version-card-select version-card-static">
                              <span className="version-card-when">{v.createdAt}</span>
                              <span className="version-card-meta">
                                <span className={`pill ${v.status}`}>{v.status}</span>
                                <span className="muted small">{fmtBytes(v.bytes)}</span>
                              </span>
                            </div>
                            <div className="version-card-actions">
                              <button
                                type="button"
                                className="ghost"
                                disabled={busy}
                                onClick={() =>
                                  setLogVersion(
                                    logVersion?.vid === v.id && logVersion.dbId === d.id
                                      ? null
                                      : { dbId: d.id, vid: v.id },
                                  )
                                }
                              >
                                Log
                              </button>
                              {v.status === "succeeded" && (
                                <button
                                  type="button"
                                  className="ghost"
                                  disabled={busy}
                                  onClick={() => void openRestore(d, v.id)}
                                >
                                  Restore
                                </button>
                              )}
                              <button
                                type="button"
                                className="ghost danger-text"
                                disabled={busy}
                                onClick={() =>
                                  setDeleteVersion({ db: d, vid: v.id, confirm: "" })
                                }
                              >
                                Delete
                              </button>
                            </div>
                          </div>
                          {logVersion?.dbId === d.id && logVersion.vid === v.id && (
                            <div className="version-card-log">
                              <VersionLogPanel
                                url={`/api/databases/${d.id}/versions/${v.id}/logs`}
                                title={`Backup log — ${d.name}`}
                                onClose={() => setLogVersion(null)}
                              />
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
                <div className="list-actions">
                  <button type="button" className="ghost" onClick={() => void backupNow(d.id)}>
                    Backup now
                  </button>
                  <Link className="ghost btn-link" to={`/app/databases/${d.id}/edit`}>
                    Edit
                  </Link>
                  <button type="button" className="ghost danger-text" onClick={() => void remove(d.id)}>
                    Delete
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
      </section>
      <SiteFooter />
    </div>
  );
}
