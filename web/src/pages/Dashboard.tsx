import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../App";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";

type Dash = {
  fileServers: number;
  databases: number;
  backupCount: number;
  storageBytes: number;
  dataDir: string;
  applianceStatus?: StatusItem[];
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

function parseWhen(s: string) {
  const d = new Date(s.includes("T") ? s : `${s}Z`);
  return Number.isNaN(d.getTime()) ? null : d;
}

function fmtDate(s: string) {
  const d = parseWhen(s);
  return d ? d.toLocaleDateString() : s;
}

function fmtTime(s: string) {
  const d = parseWhen(s);
  return d ? d.toLocaleTimeString() : "";
}

function RecentBackupList({ rows }: { rows: RecentRow[] }) {
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
              {fmtBytes(b.bytes)} · {fmtDate(b.createdAt)} · {fmtTime(b.createdAt)}
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
  const [data, setData] = useState<Dash | null>(null);
  const [fileBackups, setFileBackups] = useState<RecentRow[]>([]);
  const [dbBackups, setDbBackups] = useState<RecentRow[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      api<Dash>("/api/dashboard"),
      api<RecentRow[]>("/api/backups/recent?limit=15&type=file"),
      api<RecentRow[]>("/api/backups/recent?limit=15&type=db"),
    ])
      .then(([d, files, dbs]) => {
        setData(d);
        setFileBackups(files);
        setDbBackups(dbs);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed"));
  }, []);

  return (
    <div className="shell">
      <Nav onLogout={() => void onLogout()} />
      <header className="page-head">
        <h1>Dashboard</h1>
        <p className="muted">Overview and recent activity</p>
      </header>

      {error && <p className="err pad">{error}</p>}

      <section className="tile dash-overview-panel">
        <div className="dash-overview">
          <article className="dash-tile">
            <h2>File servers</h2>
            <p className="stat">{data?.fileServers ?? "—"}</p>
            <Link className="text-link" to="/app/file-servers">
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
            <p className="muted small dash-meta">
              Data dir: <code>{data?.dataDir ?? "—"}</code>
            </p>
          </article>
        </div>
      </section>

      <div className="dash-recents" id="recent-backups">
        <section className="tile dash-section">
          <div className="section-head">
            <h2>Recent file backups</h2>
            <Link className="text-link" to="/app/file-servers">
              All file servers →
            </Link>
          </div>
          <RecentBackupList rows={fileBackups} />
        </section>

        <section className="tile dash-section">
          <div className="section-head">
            <h2>Recent database backups</h2>
            <Link className="text-link" to="/app/databases">
              All databases →
            </Link>
          </div>
          <RecentBackupList rows={dbBackups} />
        </section>
      </div>

      {data?.applianceStatus && data.applianceStatus.length > 0 && (
        <ApplianceStatus items={data.applianceStatus} />
      )}

      <SiteFooter />
    </div>
  );
}
