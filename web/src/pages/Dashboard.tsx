import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../App";
import { useTimezone } from "../context/Timezone";
import { formatApplianceDate, formatApplianceTime } from "../lib/formatTime";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import TargetHealthBadge, { type TargetHealthRow } from "../components/TargetHealthBadge";

type Dash = {
  websites?: number;
  fileServers: number;
  databases: number;
  backupCount: number;
  storageBytes: number;
  storageForecast?: {
    currentBytes: number;
    dailyBytes: number;
    netDailyBytes?: number;
    steadyStateBytes?: number;
    projected30Day: number;
    sampleDays: number;
  };
  dataDir: string;
  applianceStatus?: StatusItem[];
  offsiteBanner?: { show: boolean; level?: string; message?: string };
};

type StatusItem = {
  id: string;
  label: string;
  ok: boolean;
  detail: string;
};

type RecentRow = {
  id: string;
  targetType: string;
  targetId: string;
  status: string;
  bytes: number;
  createdAt: string;
  targetName: string;
  exploreUrl: string;
};

type Props = {
  onLogout: () => Promise<void>;
};

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function est30Days(forecast: NonNullable<Dash["storageForecast"]>) {
  const projected = forecast.projected30Day;
  const cap = forecast.steadyStateBytes ?? 0;
  return cap > projected ? cap : projected;
}

function RecentBackupList({ rows, timeZone }: { rows: RecentRow[]; timeZone: string }) {
  if (rows.length === 0) {
    return <p className="muted">No backups yet.</p>;
  }
  return (
    <ul className="list dash-backup-list">
      {rows.map((b) => (
        <li key={b.id}>
          <div className="dash-backup-main">
            <div className="dash-backup-top">
              <strong className="dash-backup-name">{b.targetName || b.targetId}</strong>
              <span className={`pill ${b.status}`}>{b.status}</span>
            </div>
            <p className="muted small dash-backup-meta">
              {fmtBytes(b.bytes)} · {formatApplianceDate(b.createdAt, timeZone)} ·{" "}
              {formatApplianceTime(b.createdAt, timeZone)}
            </p>
          </div>
          {b.exploreUrl ? (
            <Link className="ghost btn-link dash-backup-explore" to={b.exploreUrl}>
              Explore
            </Link>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

function ApplianceStatus({ items }: { items: StatusItem[] }) {
  if (items.length === 0) {
    return null;
  }
  const allOk = items.every((i) => i.ok);
  return (
    <section className="tile dash-status-panel">
      <div className="dash-status-head">
        <h2>Appliance status</h2>
        <span className={`dash-status-summary ${allOk ? "ok" : "warn"}`}>
          {allOk ? "OK" : "Check"}
        </span>
      </div>
      <ul className="dash-status-grid">
        {items.map((item) => (
          <li key={item.id} className={`dash-status-chip ${item.ok ? "ok" : "warn"}`} title={item.detail}>
            <span className="dash-status-icon" aria-hidden>
              {item.ok ? "✓" : "!"}
            </span>
            <span className="dash-status-label">{item.label}</span>
            <span className="dash-status-detail">{item.detail}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

export default function Dashboard({ onLogout }: Props) {
  const { timezone } = useTimezone();
  const [data, setData] = useState<Dash | null>(null);
  const [fileBackups, setFileBackups] = useState<RecentRow[]>([]);
  const [dbBackups, setDbBackups] = useState<RecentRow[]>([]);
  const [error, setError] = useState("");

  const [targetHealth, setTargetHealth] = useState<TargetHealthRow[]>([]);
  const [bulkBusy, setBulkBusy] = useState(false);
  const [info, setInfo] = useState("");

  useEffect(() => {
    Promise.all([
      api<Dash>("/api/dashboard"),
      api<RecentRow[]>("/api/backups/recent?limit=10&type=file"),
      api<RecentRow[]>("/api/backups/recent?limit=10&type=db"),
      api<{ targets: TargetHealthRow[] }>("/api/target-health"),
    ])
      .then(([d, files, dbs, health]) => {
        setData(d);
        setFileBackups(files);
        setDbBackups(dbs);
        setTargetHealth(health.targets);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed"));
  }, []);

  const websiteCount = data?.websites ?? data?.fileServers;
  const databaseCount = data?.databases ?? 0;
  const hasTargets = (websiteCount ?? 0) > 0 || databaseCount > 0;

  const globalFullBackup = async () => {
    setBulkBusy(true);
    setError("");
    try {
      const res = await api<{
        jobs: { targetType: string; targetName: string; error?: string }[];
      }>("/api/backup/global-full", { method: "POST" });
      const ok = res.jobs.filter((j) => !j.error);
      const files = ok.filter((j) => j.targetType === "file").length;
      const dbs = ok.filter((j) => j.targetType === "db").length;
      setInfo(
        `Started full global backup: ${files} website(s), ${dbs} database(s). Check Jobs for progress.`,
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "global backup failed");
    } finally {
      setBulkBusy(false);
    }
  };

  return (
    <div className="shell">
      <Nav onLogout={() => void onLogout()} />
      <header className="page-head row-head">
        <div>
          <h1>Dashboard</h1>
          <p className="muted">Overview and recent activity</p>
        </div>
        {hasTargets && (
          <div className="head-actions">
            <button
              type="button"
              className="btn-primary"
              disabled={bulkBusy}
              onClick={() => void globalFullBackup()}
            >
              {bulkBusy ? "Starting…" : "Full global backup"}
            </button>
          </div>
        )}
      </header>

      {error && <p className="err pad">{error}</p>}
      {info && <p className="ok pad">{info}</p>}

      {data?.offsiteBanner?.show && (
        <section className={`tile offsite-banner ${data.offsiteBanner.level || "warn"}`}>
          <strong>Off-site mirror</strong>
          <p>{data.offsiteBanner.message}</p>
          <Link className="text-link" to="/app/settings?tab=offsite">
            Check off-site settings →
          </Link>
        </section>
      )}

      {targetHealth.some((t) => t.health === "error" || t.health === "warning") && (
        <section className="tile target-health-panel">
          <h2>Backup health</h2>
          <ul className="target-health-list">
            {targetHealth
              .filter((t) => t.health === "error" || t.health === "warning")
              .map((t) => (
                <li key={`${t.targetType}:${t.id}`}>
                  <strong>{t.name}</strong>{" "}
                  <TargetHealthBadge health={t.health} detail={t.healthDetail} />
                  <span className="muted small"> — {t.healthDetail}</span>
                </li>
              ))}
          </ul>
        </section>
      )}

      <section className="tile dash-overview-panel">
        <div className="dash-overview">
          <article className="dash-tile">
            <h2>Websites</h2>
            <p className="stat">{websiteCount ?? "—"}</p>
            <Link className="text-link" to="/app/websites">
              Manage →
            </Link>
          </article>
          <article className="dash-tile">
            <h2>Databases</h2>
            <p className="stat">{data?.databases ?? "—"}</p>
            <Link className="text-link" to="/app/databases">
              Manage →
            </Link>
          </article>
          <article className="dash-tile">
            <h2>Backups</h2>
            <p className="stat">{data?.backupCount ?? "—"}</p>
            <a className="text-link" href="#recent-backups">
              View recent →
            </a>
          </article>
          <article className="dash-tile">
            <h2>Storage</h2>
            <p className="stat stat-compact">{data ? fmtBytes(data.storageBytes) : "—"}</p>
            {data?.storageForecast &&
              (data.storageForecast.netDailyBytes ?? data.storageForecast.dailyBytes) > 0 && (
              <p className="muted small dash-meta">
                ~Est {fmtBytes(est30Days(data.storageForecast))} in 30 days
              </p>
            )}
            <p className="muted small dash-meta">
              Data dir: <code>{data?.dataDir ?? "—"}</code>
            </p>
          </article>
        </div>
      </section>

      <div className="dash-recents" id="recent-backups">
        <section className="tile dash-section">
          <div className="section-head">
            <h2>Recent website backups</h2>
            <Link className="text-link" to="/app/websites">
              All websites →
            </Link>
          </div>
          <RecentBackupList rows={fileBackups} timeZone={timezone} />
        </section>

        <section className="tile dash-section">
          <div className="section-head">
            <h2>Recent database backups</h2>
            <Link className="text-link" to="/app/databases">
              All databases →
            </Link>
          </div>
          <RecentBackupList rows={dbBackups} timeZone={timezone} />
        </section>
      </div>

      {data?.applianceStatus && data.applianceStatus.length > 0 && (
        <ApplianceStatus items={data.applianceStatus} />
      )}

      <SiteFooter />
    </div>
  );
}
