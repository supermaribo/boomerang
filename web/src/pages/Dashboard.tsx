import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../App";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";

type Version = {
  id: string;
  targetType: string;
  targetId: string;
  status: string;
  bytes: number;
  createdAt: string;
};

type Job = {
  id: string;
  targetType: string;
  targetId: string;
  kind: string;
  status: string;
  error: string;
  createdAt: string;
};

type Dash = {
  fileServers: number;
  databases: number;
  backupCount: number;
  storageBytes: number;
  dataDir: string;
  recentBackups: Version[];
  recentJobs: Job[];
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

function fmtWhen(s: string) {
  const d = new Date(s.includes("T") ? s : s + "Z");
  return Number.isNaN(d.getTime()) ? s : d.toLocaleString();
}

export default function Dashboard({ onLogout }: Props) {
  const [data, setData] = useState<Dash | null>(null);
  const [recent, setRecent] = useState<RecentRow[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    Promise.all([
      api<Dash>("/api/dashboard"),
      api<RecentRow[]>("/api/backups/recent?limit=20"),
    ])
      .then(([d, r]) => {
        setData(d);
        setRecent(r);
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
            <h2>Recent backups</h2>
          </div>
          {recent.length === 0 && <p className="muted">No backups yet.</p>}
          <ul className="list">
            {recent.map((b) => (
              <li key={b.id}>
                <div>
                  <strong>{b.targetName || b.targetId}</strong>
                  <span className="muted">
                    {" "}
                    · {b.targetType} · {b.status} · {fmtBytes(b.bytes)}
                  </span>
                  <div className="muted small">{fmtWhen(b.createdAt)}</div>
                </div>
                <div className="actions tight">
                  {b.targetType === "file" && b.exploreUrl && (
                    <Link className="ghost btn-link" to={b.exploreUrl}>
                      Explore
                    </Link>
                  )}
                  {b.targetType === "db" && (
                    <Link className="ghost btn-link" to="/app/databases">
                      Databases
                    </Link>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </section>

        <section className="tile dash-section">
          <div className="section-head">
            <h2>Recent jobs</h2>
          </div>
          {(data?.recentJobs || []).length === 0 && <p className="muted">No jobs yet.</p>}
          <ul className="list">
            {(data?.recentJobs || []).map((j) => (
              <li key={j.id}>
                <div>
                  <strong>
                    {j.kind} · {j.targetType}
                  </strong>
                  <span className={`pill ${j.status}`}>{j.status}</span>
                  <div className="muted small">
                    {fmtWhen(j.createdAt)}
                    {j.error ? ` · ${j.error}` : ""}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        </section>
      </div>
      <SiteFooter />
    </div>
  );
}
