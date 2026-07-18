import { FormEvent, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../lib/api";
import { asArray } from "../lib/arrays";
import { useTimezone } from "../context/Timezone";
import { formatApplianceDateTime } from "../lib/formatTime";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";

export type MonitoredServer = {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  publicKey?: string;
  fileServerId?: string | null;
  enabled: boolean;
  online: boolean;
  statusDetail?: string;
  cpuPercent?: number;
  memPercent?: number;
  load1?: number;
  numCpu?: number;
  uptimeSec?: number;
  primaryDiskMount?: string;
  primaryDiskPercent?: number;
  netIface?: string;
  netRxBps?: number | null;
  netTxBps?: number | null;
  lastSampleAt?: string;
  lastPollError?: string;
  clientVersion?: string;
  latestClientVersion?: string;
  clientUpdateAvailable?: boolean;
  activeAlerts?: string[];
  installCommand?: string;
  pollIntervalSec?: number;
  offlineAfterSec?: number;
  alertCpuPercent?: number;
  alertMemPercent?: number;
  alertDiskPercent?: number;
  alertLoadPerCpu?: number;
  alertSustainSec?: number;
  alertsEnabled?: boolean;
};

function fmtUptime(sec?: number) {
  if (!sec || sec < 0) return "—";
  const d = Math.floor(sec / 86400);
  const h = Math.floor((sec % 86400) / 3600);
  if (d > 0) return `${d}d ${h}h`;
  const m = Math.floor((sec % 3600) / 60);
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}

function pct(n?: number) {
  if (n == null || Number.isNaN(n)) return "—";
  if (Math.abs(n) < 10) return `${n.toFixed(1)}%`;
  return `${n.toFixed(0)}%`;
}

function fmtRate(bps?: number | null) {
  if (bps == null || !Number.isFinite(bps) || bps < 0) return "—";
  if (bps < 1024) return `${bps.toFixed(0)} B/s`;
  if (bps < 1024 * 1024) return `${(bps / 1024).toFixed(1)} KB/s`;
  if (bps < 1024 * 1024 * 1024) return `${(bps / (1024 * 1024)).toFixed(2)} MB/s`;
  return `${(bps / (1024 * 1024 * 1024)).toFixed(2)} GB/s`;
}

export default function Monitoring() {
  const { timezone } = useTimezone();
  const [servers, setServers] = useState<MonitoredServer[]>([]);
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [showNew, setShowNew] = useState(false);
  const [name, setName] = useState("");
  const [host, setHost] = useState("");
  const [port, setPort] = useState(22);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    const list = asArray(await api<MonitoredServer[] | null>("/api/monitoring/servers"));
    setServers(list);
  };

  useEffect(() => {
    void load().catch((e) => setError(e instanceof Error ? e.message : "load failed"));
    const t = setInterval(() => {
      void load().catch(() => {});
    }, 30000);
    return () => clearInterval(t);
  }, []);

  const create = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const created = await api<MonitoredServer>("/api/monitoring/servers", {
        method: "POST",
        body: JSON.stringify({ name, host, port, username: "boomerang-monitor" }),
      });
      setShowNew(false);
      setName("");
      setHost("");
      setPort(22);
      setInfo(`Created ${created.name}. Open it to copy the install command.`);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "create failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <Nav />
      <header className="page-head row-head">
        <div>
          <h1>Monitoring</h1>
          <p className="muted">
            Uptime, CPU, memory, disk, and load from Linux servers via a restricted SSH agent.
          </p>
        </div>
        <button type="button" className="btn-primary" onClick={() => setShowNew(true)}>
          Add server
        </button>
      </header>

      <div className="err">{error}</div>
      {info && <p className="ok pad">{info}</p>}

      {showNew && (
        <section className="tile">
          <h2>Add monitored server</h2>
          <p className="muted small">
            Requires a Linux VPS where you can run the installer with sudo once. Shared hosting is
            not supported.
          </p>
          <form className="stack" onSubmit={create}>
            <label>
              Name
              <input value={name} onChange={(e) => setName(e.target.value)} required />
            </label>
            <label>
              SSH host
              <input value={host} onChange={(e) => setHost(e.target.value)} required />
            </label>
            <label>
              SSH port
              <input
                type="number"
                value={port}
                onChange={(e) => setPort(Number(e.target.value) || 22)}
              />
            </label>
            <div className="actions">
              <button type="submit" disabled={busy}>
                {busy ? "Creating…" : "Create & generate key"}
              </button>
              <button type="button" className="ghost" onClick={() => setShowNew(false)}>
                Cancel
              </button>
            </div>
          </form>
        </section>
      )}

      <section className="tile">
        {servers.length === 0 ? (
          <p className="muted">No monitored servers yet. Add a VPS to get started.</p>
        ) : (
          <ul className="list">
            {servers.map((s) => (
              <li key={s.id} className="monitor-row">
                <div className="monitor-row-main">
                  <div className="monitor-server-title">
                    <Link className="text-link" to={`/app/monitoring/${s.id}`}>
                      <strong>{s.name}</strong>
                    </Link>
                    <span className={`pill ${s.online ? "succeeded" : "failed"}`}>
                      {s.online ? "online" : "offline"}
                    </span>
                    {s.clientUpdateAvailable && (
                      <span className="pill warning" title={`Update to ${s.latestClientVersion}`}>
                        update available
                      </span>
                    )}
                  </div>
                  <p className="muted small">
                    {s.host}:{s.port} · {s.statusDetail || ""}
                    {s.clientVersion ? ` · client ${s.clientVersion}` : ""}
                    {s.clientUpdateAvailable && s.latestClientVersion
                      ? ` → ${s.latestClientVersion}`
                      : ""}
                    {s.lastSampleAt
                      ? ` · last ${formatApplianceDateTime(s.lastSampleAt, timezone)}`
                      : ""}
                  </p>
                  <p className="muted small monitor-metrics">
                    up {fmtUptime(s.uptimeSec)} · CPU {pct(s.cpuPercent)} · RAM {pct(s.memPercent)} ·
                    load {s.load1?.toFixed(2) ?? "—"}
                    {s.primaryDiskMount
                      ? ` · ${s.primaryDiskMount} ${pct(s.primaryDiskPercent)}`
                      : ""}
                  </p>
                  {s.netIface && (
                    <p className="muted small monitor-rates">
                      {s.netIface}: ↓ {fmtRate(s.netRxBps)} · ↑ {fmtRate(s.netTxBps)}
                    </p>
                  )}
                  {(s.activeAlerts?.length ?? 0) > 0 && (
                    <p className="err small">Alerts: {s.activeAlerts!.join(", ")}</p>
                  )}
                </div>
                <Link className="ghost btn-link" to={`/app/monitoring/${s.id}`}>
                  Open
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
      <SiteFooter />
    </div>
  );
}
