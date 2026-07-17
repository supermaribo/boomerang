import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../lib/api";
import { asArray } from "../lib/arrays";
import { useTimezone } from "../context/Timezone";
import { formatApplianceDateTime } from "../lib/formatTime";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import type { MonitoredServer } from "./Monitoring";

type HistoryPoint = {
  at: string;
  cpu?: number;
  mem?: number;
  load1?: number;
  disk?: number;
};

type FS = {
  mount: string;
  device?: string;
  fsType?: string;
  totalBytes: number;
  usedBytes: number;
  freeBytes: number;
};

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function Sparkline({
  points,
  valueKey,
  color,
  unit = "",
}: {
  points: HistoryPoint[];
  valueKey: keyof HistoryPoint;
  color: string;
  unit?: string;
}) {
  const vals = points
    .map((p) => Number(p[valueKey] ?? 0))
    .filter((n) => Number.isFinite(n));
  const fmt = (v: number) =>
    unit === "%" ? `${v.toFixed(1)}%` : v >= 100 ? v.toFixed(0) : v.toFixed(2);
  if (vals.length === 0) {
    return <p className="muted small">Not enough history yet.</p>;
  }
  if (vals.length === 1) {
    return (
      <p className="muted small">
        Collecting data — 1 sample so far (now {fmt(vals[0])}).
      </p>
    );
  }
  const w = 600;
  const h = 140;
  const padX = 4;
  const padY = 6;
  const dataMax = Math.max(...vals);
  // Scale to the data so small values (0.3% CPU, load 0.1) stay visible.
  const yMax = dataMax <= 0 ? 1 : dataMax * 1.15;
  const x = (i: number) => padX + (i / (vals.length - 1)) * (w - padX * 2);
  const y = (v: number) => h - padY - (v / yMax) * (h - padY * 2);
  const line = vals.map((v, i) => `${x(i).toFixed(1)},${y(v).toFixed(1)}`).join(" ");
  const area = `${padX},${h - padY} ${line} ${(w - padX).toFixed(1)},${h - padY}`;
  const latest = vals[vals.length - 1];
  return (
    <div className="monitor-chart-wrap">
      <svg
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        className="monitor-chart"
        role="img"
        aria-label="history chart"
      >
        <polygon points={area} fill={color} opacity="0.12" />
        <polyline
          fill="none"
          stroke={color}
          strokeWidth="2"
          vectorEffect="non-scaling-stroke"
          points={line}
        />
      </svg>
      <p className="muted small monitor-chart-meta">
        now {fmt(latest)} · peak {fmt(dataMax)} · {vals.length} points
      </p>
    </div>
  );
}

export default function MonitorDetail() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { timezone } = useTimezone();
  const [server, setServer] = useState<MonitoredServer | null>(null);
  const [range, setRange] = useState<"24h" | "7d" | "30d">("24h");
  const [points, setPoints] = useState<HistoryPoint[]>([]);
  const [filesystems, setFilesystems] = useState<FS[]>([]);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [edit, setEdit] = useState({
    alertCpuPercent: 90,
    alertMemPercent: 90,
    alertDiskPercent: 90,
    alertLoadPerCpu: 2,
    alertSustainSec: 300,
    offlineAfterSec: 180,
    alertsEnabled: true,
    enabled: true,
  });

  const load = async () => {
    const s = await api<MonitoredServer>(`/api/monitoring/servers/${id}`);
    setServer(s);
    setEdit({
      alertCpuPercent: s.alertCpuPercent ?? 90,
      alertMemPercent: s.alertMemPercent ?? 90,
      alertDiskPercent: s.alertDiskPercent ?? 90,
      alertLoadPerCpu: s.alertLoadPerCpu ?? 2,
      alertSustainSec: s.alertSustainSec ?? 300,
      offlineAfterSec: s.offlineAfterSec ?? 180,
      alertsEnabled: s.alertsEnabled ?? true,
      enabled: s.enabled,
    });
    const hist = await api<{ points: HistoryPoint[] | null; filesystems: FS[] | null }>(
      `/api/monitoring/servers/${id}/history?range=${range}`,
    );
    setPoints(asArray(hist.points));
    setFilesystems(asArray(hist.filesystems));
  };

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "load failed"));
  }, [id, range]);

  const installCmd = useMemo(() => server?.installCommand || "", [server]);

  const copyInstall = async () => {
    if (!installCmd) return;
    await navigator.clipboard.writeText(installCmd);
    setInfo("Install command copied.");
  };

  const test = async () => {
    setBusy(true);
    setError("");
    try {
      const res = await api<{ message: string }>(`/api/monitoring/servers/${id}/test`, {
        method: "POST",
      });
      setInfo(res.message);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "test failed");
    } finally {
      setBusy(false);
    }
  };

  const pollNow = async () => {
    setBusy(true);
    setError("");
    try {
      await api(`/api/monitoring/servers/${id}/poll`, { method: "POST" });
      setInfo("Polled successfully.");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "poll failed");
    } finally {
      setBusy(false);
    }
  };

  const rotateKey = async () => {
    if (!window.confirm(`Rotate key for ${server?.name}? Re-run the installer afterwards.`)) return;
    setBusy(true);
    try {
      const s = await api<MonitoredServer>(`/api/monitoring/servers/${id}/rotate-key`, {
        method: "POST",
      });
      setServer(s);
      setInfo("Key rotated. Copy the new install command and re-run it on the server.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "rotate failed");
    } finally {
      setBusy(false);
    }
  };

  const saveThresholds = async (e: FormEvent) => {
    e.preventDefault();
    if (!server) return;
    setBusy(true);
    setError("");
    try {
      const s = await api<MonitoredServer>(`/api/monitoring/servers/${id}`, {
        method: "PUT",
        body: JSON.stringify({
          name: server.name,
          host: server.host,
          port: server.port,
          username: server.username,
          ...edit,
        }),
      });
      setServer(s);
      setInfo("Thresholds saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed");
    } finally {
      setBusy(false);
    }
  };

  const remove = async () => {
    if (!server) return;
    setBusy(true);
    try {
      await api(`/api/monitoring/servers/${id}`, {
        method: "DELETE",
        body: JSON.stringify({ confirmName: confirm }),
      });
      navigate("/app/monitoring", { replace: true });
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
          <h1>{server?.name || "Server"}</h1>
          <p className="muted">
            {server ? (
              <>
                {server.host}:{server.port} ·{" "}
                <span className={`pill ${server.online ? "succeeded" : "failed"}`}>
                  {server.online ? "online" : "offline"}
                </span>
                {server.clientVersion ? ` · client ${server.clientVersion}` : ""}
              </>
            ) : (
              "…"
            )}
          </p>
        </div>
        <Link className="ghost btn-link" to="/app/monitoring">
          Back
        </Link>
      </header>

      <div className="err">{error}</div>
      {info && <p className="ok pad">{info}</p>}

      {server && (
        <>
          <section className="tile">
            <h2>Install client</h2>
            <p className="muted small">
              Run once with sudo on the target Linux server. Creates user{" "}
              <code>boomerang-monitor</code> and a forced-command SSH key (metrics export only).
            </p>
            <textarea className="pubkey" readOnly rows={3} value={installCmd} />
            <div className="actions">
              <button type="button" onClick={() => void copyInstall()}>
                Copy install command
              </button>
              <button type="button" className="ghost" disabled={busy} onClick={() => void test()}>
                Test connection
              </button>
              <button type="button" className="ghost" disabled={busy} onClick={() => void pollNow()}>
                Poll now
              </button>
              <button type="button" className="ghost" disabled={busy} onClick={() => void rotateKey()}>
                Rotate key
              </button>
            </div>
            {server.lastSampleAt && (
              <p className="muted small">
                Last sample {formatApplianceDateTime(server.lastSampleAt, timezone)}
                {server.lastPollError ? ` · last error: ${server.lastPollError}` : ""}
              </p>
            )}
          </section>

          <section className="tile">
            <div className="row-head">
              <h2>History</h2>
              <div className="actions">
                {(["24h", "7d", "30d"] as const).map((r) => (
                  <button
                    key={r}
                    type="button"
                    className={`ghost${range === r ? " active" : ""}`}
                    onClick={() => setRange(r)}
                  >
                    {r}
                  </button>
                ))}
              </div>
            </div>
            <div className="monitor-charts">
              <div>
                <h3 className="muted small">CPU %</h3>
                <Sparkline points={points} valueKey="cpu" color="var(--accent)" unit="%" />
              </div>
              <div>
                <h3 className="muted small">Memory %</h3>
                <Sparkline points={points} valueKey="mem" color="#7dd3a7" unit="%" />
              </div>
              <div>
                <h3 className="muted small">Load 1</h3>
                <Sparkline points={points} valueKey="load1" color="#e8b86d" />
              </div>
              {range !== "24h" && (
                <div>
                  <h3 className="muted small">Disk % (max)</h3>
                  <Sparkline points={points} valueKey="disk" color="#e07a7a" unit="%" />
                </div>
              )}
            </div>
          </section>

          <section className="tile">
            <h2>Filesystems</h2>
            {filesystems.length === 0 ? (
              <p className="muted">No filesystem data yet.</p>
            ) : (
              <ul className="list plain">
                {filesystems.map((f) => {
                  const pct = f.totalBytes > 0 ? (100 * f.usedBytes) / f.totalBytes : 0;
                  return (
                    <li key={f.mount}>
                      <code>{f.mount}</code>{" "}
                      <span className="muted">
                        {pct.toFixed(0)}% · {fmtBytes(f.usedBytes)} / {fmtBytes(f.totalBytes)}
                        {f.fsType ? ` · ${f.fsType}` : ""}
                      </span>
                    </li>
                  );
                })}
              </ul>
            )}
          </section>

          <section className="tile">
            <h2>Thresholds</h2>
            <form className="stack" onSubmit={saveThresholds}>
              <label className="check">
                <input
                  type="checkbox"
                  checked={edit.enabled}
                  onChange={(e) => setEdit({ ...edit, enabled: e.target.checked })}
                />
                Enabled
              </label>
              <label className="check">
                <input
                  type="checkbox"
                  checked={edit.alertsEnabled}
                  onChange={(e) => setEdit({ ...edit, alertsEnabled: e.target.checked })}
                />
                Email alerts
              </label>
              <div className="grid-2">
                <label>
                  CPU alert %
                  <input
                    type="number"
                    value={edit.alertCpuPercent}
                    onChange={(e) => setEdit({ ...edit, alertCpuPercent: Number(e.target.value) })}
                  />
                </label>
                <label>
                  Memory alert %
                  <input
                    type="number"
                    value={edit.alertMemPercent}
                    onChange={(e) => setEdit({ ...edit, alertMemPercent: Number(e.target.value) })}
                  />
                </label>
                <label>
                  Disk alert %
                  <input
                    type="number"
                    value={edit.alertDiskPercent}
                    onChange={(e) => setEdit({ ...edit, alertDiskPercent: Number(e.target.value) })}
                  />
                </label>
                <label>
                  Load per CPU
                  <input
                    type="number"
                    step="0.1"
                    value={edit.alertLoadPerCpu}
                    onChange={(e) => setEdit({ ...edit, alertLoadPerCpu: Number(e.target.value) })}
                  />
                </label>
                <label>
                  Sustain seconds
                  <input
                    type="number"
                    value={edit.alertSustainSec}
                    onChange={(e) => setEdit({ ...edit, alertSustainSec: Number(e.target.value) })}
                  />
                </label>
                <label>
                  Offline after seconds
                  <input
                    type="number"
                    value={edit.offlineAfterSec}
                    onChange={(e) => setEdit({ ...edit, offlineAfterSec: Number(e.target.value) })}
                  />
                </label>
              </div>
              <button type="submit" disabled={busy}>
                Save thresholds
              </button>
            </form>
          </section>

          <section className="tile">
            <h2>Remove</h2>
            <label>
              Type {server.name} to confirm
              <input value={confirm} onChange={(e) => setConfirm(e.target.value)} />
            </label>
            <button
              type="button"
              className="danger-text"
              disabled={busy || confirm !== server.name}
              onClick={() => void remove()}
            >
              Delete monitored server
            </button>
          </section>
        </>
      )}
      <SiteFooter />
    </div>
  );
}
