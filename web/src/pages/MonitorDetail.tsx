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
  netRxBps?: number;
  netTxBps?: number;
  netIface?: string;
};

type FS = {
  mount: string;
  device?: string;
  fsType?: string;
  totalBytes: number;
  usedBytes: number;
  freeBytes: number;
};

type LogSource = {
  id: string;
  label: string;
  kind: "journal" | "file";
};

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function fmtRate(bps: number) {
  if (!Number.isFinite(bps) || bps < 0) return "—";
  if (bps < 1024) return `${bps.toFixed(0)} B/s`;
  if (bps < 1024 * 1024) return `${(bps / 1024).toFixed(1)} KB/s`;
  if (bps < 1024 * 1024 * 1024) return `${(bps / (1024 * 1024)).toFixed(2)} MB/s`;
  return `${(bps / (1024 * 1024 * 1024)).toFixed(2)} GB/s`;
}

function Sparkline({
  points,
  series,
  unit = "",
  emptyHint,
}: {
  points: HistoryPoint[];
  series: { key: keyof HistoryPoint; color: string; label?: string }[];
  unit?: "" | "%" | "rate";
  emptyHint?: string;
}) {
  const seriesVals = series.map((s) =>
    points
      .map((p) => {
        const v = p[s.key];
        return typeof v === "number" && Number.isFinite(v) ? v : null;
      })
      .filter((n): n is number => n != null),
  );
  const allVals = seriesVals.flat();
  const fmt = (v: number) => {
    if (unit === "%") return `${v.toFixed(1)}%`;
    if (unit === "rate") return fmtRate(v);
    return v >= 100 ? v.toFixed(0) : v.toFixed(2);
  };
  if (allVals.length === 0) {
    return <p className="muted small">{emptyHint || "Not enough history yet."}</p>;
  }
  if (allVals.length < 2 && series.length === 1 && seriesVals[0].length < 2) {
    return (
      <p className="muted small">
        Collecting data — 1 sample so far (now {fmt(allVals[0])}).
      </p>
    );
  }
  const w = 600;
  const h = 140;
  const isPercent = unit === "%";
  const padLeft = isPercent ? 44 : unit === "rate" ? 52 : 4;
  const padRight = 4;
  const padY = 6;
  const dataMax = Math.max(...allVals, 0);
  const yMax = isPercent ? 100 : dataMax <= 0 ? 1 : dataMax * 1.15;
  const x = (i: number, n: number) =>
    padLeft + (i / Math.max(n - 1, 1)) * (w - padLeft - padRight);
  const y = (v: number) =>
    h - padY - (Math.min(Math.max(v, 0), yMax) / yMax) * (h - padY * 2);
  const percentTicks = [100, 75, 50, 25, 0];
  const rateTicks =
    unit === "rate"
      ? [yMax, yMax * 0.75, yMax * 0.5, yMax * 0.25, 0].map((t) => Math.max(0, t))
      : [];

  return (
    <div className="monitor-chart-wrap">
      <svg
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        className="monitor-chart"
        role="img"
        aria-label="history chart"
      >
        {isPercent &&
          percentTicks.map((tick) => (
            <line
              key={tick}
              x1={padLeft}
              x2={w - padRight}
              y1={y(tick)}
              y2={y(tick)}
              className="monitor-chart-gridline"
              vectorEffect="non-scaling-stroke"
            />
          ))}
        {unit === "rate" &&
          rateTicks.map((tick, i) => (
            <line
              key={`r${i}`}
              x1={padLeft}
              x2={w - padRight}
              y1={y(tick)}
              y2={y(tick)}
              className="monitor-chart-gridline"
              vectorEffect="non-scaling-stroke"
            />
          ))}
        {series.map((s) => {
          const vals = points
            .map((p) => {
              const v = p[s.key];
              return typeof v === "number" && Number.isFinite(v) ? v : null;
            });
          const indexed = vals
            .map((v, i) => (v == null ? null : { i, v }))
            .filter((p): p is { i: number; v: number } => p != null);
          if (indexed.length < 2) return null;
          const line = indexed
            .map(({ i, v }) => `${x(i, points.length).toFixed(1)},${y(v).toFixed(1)}`)
            .join(" ");
          const area = `${x(indexed[0].i, points.length).toFixed(1)},${h - padY} ${line} ${x(
            indexed[indexed.length - 1].i,
            points.length,
          ).toFixed(1)},${h - padY}`;
          return (
            <g key={String(s.key)}>
              <polygon points={area} fill={s.color} opacity="0.1" />
              <polyline
                fill="none"
                stroke={s.color}
                strokeWidth="2"
                vectorEffect="non-scaling-stroke"
                points={line}
              />
            </g>
          );
        })}
      </svg>
      {isPercent && (
        <div className="monitor-chart-axis" aria-hidden="true">
          {percentTicks.map((tick) => (
            <span key={tick}>{tick}%</span>
          ))}
        </div>
      )}
      {unit === "rate" && (
        <div className="monitor-chart-axis monitor-chart-axis-wide" aria-hidden="true">
          {rateTicks.map((tick, i) => (
            <span key={i}>{fmtRate(tick)}</span>
          ))}
        </div>
      )}
      <p className="muted small monitor-chart-meta">
        {series
          .map((s) => {
            const vals = seriesVals[series.indexOf(s)];
            if (vals.length === 0) return `${s.label || String(s.key)} —`;
            const latest = vals[vals.length - 1];
            const peak = Math.max(...vals);
            return `${s.label || String(s.key)} now ${fmt(latest)} · peak ${fmt(peak)}`;
          })
          .join(" · ")}
        {` · ${points.length} points`}
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
  const [netIface, setNetIface] = useState("");
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState("");
  const [logText, setLogText] = useState("");
  const [logSources, setLogSources] = useState<LogSource[]>([]);
  const [logSource, setLogSource] = useState("");
  const [logSourceError, setLogSourceError] = useState("");
  const [logLines, setLogLines] = useState(200);
  const [logBusy, setLogBusy] = useState(false);
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
    const hist = await api<{
      points: HistoryPoint[] | null;
      filesystems: FS[] | null;
      netIface?: string;
    }>(`/api/monitoring/servers/${id}/history?range=${range}`);
    setPoints(asArray(hist.points));
    setFilesystems(asArray(hist.filesystems));
    setNetIface(hist.netIface || s.netIface || "");
  };

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "load failed"));
  }, [id, range]);

  useEffect(() => {
    setLogSources([]);
    setLogSource("");
    setLogSourceError("");
    void api<{ sources: LogSource[] | null }>(`/api/monitoring/servers/${id}/logs/sources`)
      .then((res) => {
        const sources = asArray(res.sources);
        setLogSources(sources);
        setLogSource(sources[0]?.id || "");
      })
      .catch((e) =>
        setLogSourceError(
          e instanceof Error
            ? e.message
            : "Could not discover logs. Re-run the monitor install command on this host.",
        ),
      );
  }, [id]);

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

  const loadLogs = async () => {
    setLogBusy(true);
    setError("");
    try {
      const q = new URLSearchParams({ lines: String(logLines) });
      q.set("source", logSource || "journal");
      const res = await api<{ text: string }>(`/api/monitoring/servers/${id}/logs?${q}`);
      setLogText(res.text || "(empty)");
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load logs");
    } finally {
      setLogBusy(false);
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
                <Sparkline
                  points={points}
                  series={[{ key: "cpu", color: "var(--accent)", label: "CPU" }]}
                  unit="%"
                />
              </div>
              <div>
                <h3 className="muted small">Memory %</h3>
                <Sparkline
                  points={points}
                  series={[{ key: "mem", color: "#7dd3a7", label: "RAM" }]}
                  unit="%"
                />
              </div>
              <div>
                <h3 className="muted small">Load 1</h3>
                <Sparkline
                  points={points}
                  series={[{ key: "load1", color: "#e8b86d", label: "Load" }]}
                />
              </div>
              <div>
                <h3 className="muted small">
                  Network throughput{netIface ? ` · ${netIface}` : ""}
                </h3>
                <Sparkline
                  points={points}
                  series={[
                    { key: "netRxBps", color: "#6cb6ff", label: "RX" },
                    { key: "netTxBps", color: "#c792ea", label: "TX" },
                  ]}
                  unit="rate"
                  emptyHint={
                    netIface
                      ? "Collecting network rates — needs two samples after the agent upgrade."
                      : "No network data yet. Re-run the monitor install command on this host."
                  }
                />
              </div>
              {range !== "24h" && (
                <div>
                  <h3 className="muted small">Disk % (max)</h3>
                  <Sparkline
                    points={points}
                    series={[{ key: "disk", color: "#e07a7a", label: "Disk" }]}
                    unit="%"
                  />
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
            <div className="row-head">
              <h2>Server logs</h2>
              <button
                type="button"
                className="ghost"
                disabled={logBusy || logSources.length === 0}
                onClick={() => void loadLogs()}
              >
                {logBusy ? "Loading…" : logText ? "Refresh" : "Load logs"}
              </button>
            </div>
            <p className="muted small">
              Read-only logs pulled over the same restricted SSH key. Only sources discovered and
              allowlisted by the monitor agent are shown.
            </p>
            {logSourceError && (
              <p className="err">
                {logSourceError} Re-run the install command on the host to add log discovery and
                Apache/Nginx read access.
              </p>
            )}
            <div className="grid-2 monitor-log-controls">
              <label>
                Lines
                <input
                  type="number"
                  min={50}
                  max={1000}
                  value={logLines}
                  onChange={(e) => setLogLines(Number(e.target.value) || 200)}
                />
              </label>
              <label>
                Log
                <select
                  value={logSource}
                  disabled={logSources.length === 0}
                  onChange={(e) => {
                    setLogSource(e.target.value);
                    setLogText("");
                  }}
                >
                  {logSources.length === 0 && <option value="">No readable logs discovered</option>}
                  {logSources.map((source) => (
                    <option key={source.id} value={source.id}>
                      {source.label}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {logText ? (
              <pre className="monitor-log-panel">{logText}</pre>
            ) : (
              <p className="muted">Click Load logs to fetch recent journal entries.</p>
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
