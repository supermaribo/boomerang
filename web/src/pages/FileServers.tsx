import { Link } from "react-router-dom";
import { useEffect, useState } from "react";
import { api } from "../lib/api";
import { asArray } from "../lib/arrays";
import { useTimezone } from "../context/Timezone";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import { describeSchedule, parseSchedule } from "../lib/schedule";
import { formatApplianceDateTime } from "../lib/formatTime";
import TargetHealthBadge, { healthMap, type TargetHealthRow } from "../components/TargetHealthBadge";
import { pollJob, cancelJob } from "../lib/jobPoll";
import { retentionSummary } from "../components/ScheduleRetention";

export type FileServer = {
  id: string;
  name: string;
  protocol: string;
  host: string;
  port: number;
  username: string;
  remoteRoot: string;
  includePaths: string[];
  excludePaths: string[];
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
  incrementalEnabled: boolean;
  skipIfUnchanged: boolean;
  enabled: boolean;
  hasSecret: boolean;
  publicKey?: string;
};

function pathSummary(f: FileServer) {
  const paths = f.includePaths?.length ? f.includePaths : [f.remoteRoot];
  if (paths.length === 1) return paths[0];
  return `${paths.length} paths`;
}

export default function FileServers() {
  const { timezone } = useTimezone();
  const [list, setList] = useState<FileServer[]>([]);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");

  const [healthByID, setHealthByID] = useState<Record<string, TargetHealthRow>>({});

  const [deleteTarget, setDeleteTarget] = useState<FileServer | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const [activeJobId, setActiveJobId] = useState<string | null>(null);

  const load = async () => {
    const [list, health] = await Promise.all([
      api<FileServer[]>("/api/file-servers"),
      api<{ targets: TargetHealthRow[] }>("/api/target-health"),
    ]);
    setList(asArray(list));
    setHealthByID(healthMap(asArray(health.targets).filter((t) => t.targetType === "file")));
  };

  useEffect(() => {
    void load().catch((e) => setError(String(e.message || e)));
  }, []);

  const remove = async () => {
    if (!deleteTarget || deleteConfirm !== deleteTarget.name) return;
    setError("");
    try {
      await api(`/api/file-servers/${deleteTarget.id}`, {
        method: "DELETE",
        body: JSON.stringify({ confirmName: deleteConfirm }),
      });
      setDeleteTarget(null);
      setDeleteConfirm("");
      await load();
      setInfo(`Deleted ${deleteTarget.name}.`);
    } catch (e) {
      setError(String((e as Error).message || e));
    }
  };

  const backupNow = async (id: string) => {
    setError("");
    setInfo("");
    try {
      const res = await api<{ jobId: string }>(`/api/file-servers/${id}/backup`, { method: "POST" });
      setActiveJobId(res.jobId);
      setInfo(`Backup started (job ${res.jobId.slice(0, 8)}…).`);
      const result = await pollJob(res.jobId, (lines) => setInfo(lines.join(" · ")));
      setActiveJobId(null);
      setInfo(
        result.status === "succeeded"
          ? `Backup succeeded. ${result.lastLines.slice(-1)[0] || ""}`
          : result.status === "cancelled"
            ? "Backup cancelled."
            : `Backup failed: ${result.error || result.lastLines.slice(-1)[0] || ""}`,
      );
      await load();
    } catch (e) {
      setActiveJobId(null);
      setError(e instanceof Error ? e.message : "backup failed");
    }
  };

  const backupAll = async () => {
    setError("");
    setInfo("");
    try {
      const res = await api<{ jobs: { targetName: string; jobId: string; error?: string }[] }>(
        "/api/file-servers/backup-all",
        { method: "POST" },
      );
      const ok = asArray(res.jobs).filter((j) => !j.error);
      setInfo(`Started ${ok.length} backup job(s)…`);
      for (const j of ok) {
        await pollJob(j.jobId, () => {});
      }
      setInfo(`Queued ${ok.length} website backup(s).`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "bulk backup failed");
    }
  };

  return (
    <div className="shell">
      <Nav />
      <header className="page-head row-head">
        <div>
          <h1>Websites</h1>
          <p className="muted">Website file backup targets</p>
        </div>
        <div className="head-actions">
          <Link className="btn-primary" to="/app/websites/new">
            Add website
          </Link>
          {list.some((f) => f.enabled) && (
            <button type="button" className="ghost" onClick={() => void backupAll()}>
              Backup all
            </button>
          )}
        </div>
      </header>

      {error && <p className="err pad">{error}</p>}
      {info && (
        <p className="ok pad">
          {info}
          {activeJobId && (
            <>
              {" "}
              <button type="button" className="ghost danger-text" onClick={() => void cancelJob(activeJobId)}>
                Cancel job
              </button>
            </>
          )}
        </p>
      )}

      <section className="tile">
        {list.length === 0 && (
          <div className="empty-state">
            <p className="muted">No websites yet.</p>
            <Link className="btn-primary" to="/app/websites/new">
              Add your first website
            </Link>
          </div>
        )}
        <ul className="list list-stack">
          {list.map((f) => {
            const sched = parseSchedule(f.scheduleCron, f.scheduleStart || "", timezone);
            const health = healthByID[`file:${f.id}`];
            return (
              <li key={f.id}>
                <div className="list-main">
                  <strong>
                    <Link className="text-link" to={`/app/websites/${f.id}/backups`}>
                      {f.name}
                    </Link>
                  </strong>
                  {health && <TargetHealthBadge health={health.health} detail={health.healthDetail} />}
                  {health && (health.health === "warning" || health.health === "error") && health.healthDetail && (
                    <span className="muted small"> — {health.healthDetail}</span>
                  )}
                  {!f.enabled && <span className="badge">disabled</span>}
                  <span className="muted">
                    {" "}
                    · {f.protocol.toUpperCase()} · {f.username}@{f.host}:{f.port}
                  </span>
                  <div className="muted small">
                    <code>{pathSummary(f)}</code>
                    {f.includePaths?.length > 1 && (
                      <span> · {f.includePaths.map((p) => p.split("/").pop()).join(", ")}</span>
                    )}
                  </div>
                  <div className="muted small">
                    {describeSchedule(sched)} · {retentionSummary(f, sched)}
                    {health?.lastSuccessAt && (
                      <span> · last backup {formatApplianceDateTime(health.lastSuccessAt, timezone)}</span>
                    )}
                    {health?.nextRunAt && (
                      <span> · next run {formatApplianceDateTime(health.nextRunAt, timezone)}</span>
                    )}
                    {f.protocol === "rsync" ? (
                      <span> · full snapshot (RSYNC)</span>
                    ) : (
                      <span> · {f.incrementalEnabled !== false ? "incremental" : "full only"}</span>
                    )}
                    {f.skipIfUnchanged && <span> · skip if unchanged</span>}
                  </div>
                </div>
                <div className="list-actions">
                  <button type="button" className="ghost" onClick={() => void backupNow(f.id)}>
                    Backup now
                  </button>
                  <Link className="ghost btn-link" to={`/app/websites/${f.id}/edit`}>
                    Edit
                  </Link>
                  <button type="button" className="ghost danger-text" onClick={() => setDeleteTarget(f)}>
                    Delete
                  </button>
                </div>
              </li>
            );
          })}
        </ul>
        {deleteTarget && (
          <div className="modal-backdrop">
            <div className="modal tile">
              <h2>Delete website</h2>
              <p className="muted">
                This removes <strong>{deleteTarget.name}</strong> and all of its backup versions from
                this appliance.
              </p>
              <label>Type <strong>{deleteTarget.name}</strong> to confirm</label>
              <input value={deleteConfirm} onChange={(e) => setDeleteConfirm(e.target.value)} />
              <div className="actions">
                <button
                  type="button"
                  className="danger-text"
                  disabled={deleteConfirm !== deleteTarget.name}
                  onClick={() => void remove()}
                >
                  Delete permanently
                </button>
                <button
                  type="button"
                  className="ghost"
                  onClick={() => {
                    setDeleteTarget(null);
                    setDeleteConfirm("");
                  }}
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        )}
      </section>
      <SiteFooter />
    </div>
  );
}
