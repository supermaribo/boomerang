import { Link, useSearchParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { api } from "../App";
import { useTimezone } from "../context/Timezone";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import TargetHealthBadge, { healthMap, type TargetHealthRow } from "../components/TargetHealthBadge";
import { retentionSummary } from "../components/ScheduleRetention";
import { describeSchedule, parseSchedule } from "../lib/schedule";
import { formatApplianceDateTime } from "../lib/formatTime";

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

export default function Databases() {
  const { timezone } = useTimezone();
  const [searchParams] = useSearchParams();
  const [list, setList] = useState<Database[]>([]);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");

  const [healthByID, setHealthByID] = useState<Record<string, TargetHealthRow>>({});

  const load = async () => {
    const [dbs, health] = await Promise.all([
      api<Database[]>("/api/databases"),
      api<{ targets: TargetHealthRow[] }>("/api/target-health"),
    ]);
    setList(dbs);
    setHealthByID(healthMap(health.targets.filter((t) => t.targetType === "db")));
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

  const [deleteTarget, setDeleteTarget] = useState<Database | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState("");

  const remove = async () => {
    if (!deleteTarget || deleteConfirm !== deleteTarget.name) return;
    setError("");
    try {
      await api(`/api/databases/${deleteTarget.id}`, {
        method: "DELETE",
        body: JSON.stringify({ confirmName: deleteConfirm }),
      });
      setDeleteTarget(null);
      setDeleteConfirm("");
      await load();
      setInfo(`Deleted ${deleteTarget.name}.`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "delete failed");
    }
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
          break;
        }
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "backup failed");
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
            const health = healthByID[`db:${d.id}`];
            return (
              <li key={d.id} id={`db-${d.id}`}>
                <div className="list-main">
                  <strong>
                    <Link className="text-link" to={`/app/databases/${d.id}/backups`}>
                      {d.name}
                    </Link>
                  </strong>
                  {health && <TargetHealthBadge health={health.health} detail={health.healthDetail} />}
                  {!d.enabled && <span className="badge">disabled</span>}
                  <span className="muted">
                    {" "}
                    · {d.mysqlUser}@{d.mysqlHost}/{d.mysqlDb} · tunnel {d.tunnelMode}
                  </span>
                  <div className="muted small">
                    {describeSchedule(sched)} · {retentionSummary(d, sched)}
                    {health?.lastSuccessAt && (
                      <span> · last backup {formatApplianceDateTime(health.lastSuccessAt, timezone)}</span>
                    )}
                    {health?.nextRunAt && (
                      <span> · next run {formatApplianceDateTime(health.nextRunAt, timezone)}</span>
                    )}
                    {d.includeTables?.length
                      ? ` · ${d.includeTables.length} table(s)`
                      : " · all tables"}
                  </div>
                </div>
                <div className="list-actions">
                  <button type="button" className="ghost" onClick={() => void backupNow(d.id)}>
                    Backup now
                  </button>
                  <Link className="ghost btn-link" to={`/app/databases/${d.id}/edit`}>
                    Edit
                  </Link>
                  <button type="button" className="ghost danger-text" onClick={() => setDeleteTarget(d)}>
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
              <h2>Delete database</h2>
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
