import { useCallback, useEffect, useState } from "react";
import { api } from "../lib/api";

type TailscaleStatus = {
  enabled: boolean;
  hostname: string;
  hasAuthKey: boolean;
  hasState: boolean;
  running: boolean;
  connected: boolean;
  dnsName?: string;
  ips?: string[];
  urls?: string[];
  backendState?: string;
  lastError?: string;
  mode?: string;
  socksAddr?: string;
  tunAvailable?: boolean;
};

type Props = {
  busy: boolean;
  setBusy: (v: boolean) => void;
  onFlash: (error: string, info: string) => void;
};

export default function TailscaleSettings({ busy, setBusy, onFlash }: Props) {
  const [status, setStatus] = useState<TailscaleStatus | null>(null);
  const [hostname, setHostname] = useState("boomerang");
  const [authKey, setAuthKey] = useState("");
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    const s = await api<TailscaleStatus>("/api/settings/tailscale");
    setStatus(s);
    if (s.hostname) setHostname(s.hostname);
    return s;
  }, []);

  useEffect(() => {
    setLoading(true);
    void refresh()
      .catch((e) => onFlash(e instanceof Error ? e.message : "Failed to load Tailscale status", ""))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refresh]);

  useEffect(() => {
    if (!status?.running && !status?.connected) return;
    const t = window.setInterval(() => {
      void refresh().catch(() => undefined);
    }, 4000);
    return () => window.clearInterval(t);
  }, [status?.running, status?.connected, refresh]);

  const connect = async () => {
    setBusy(true);
    onFlash("", "");
    try {
      const s = await api<TailscaleStatus>("/api/settings/tailscale/connect", {
        method: "POST",
        body: JSON.stringify({ hostname, authKey: authKey.trim() || undefined }),
      });
      setStatus(s);
      setAuthKey("");
      onFlash(
        "",
        s.connected
          ? "Connected. Website and database targets can use Tailscale 100.x addresses."
          : "Connecting to Tailscale…",
      );
    } catch (e) {
      onFlash(e instanceof Error ? e.message : "Connect failed", "");
    } finally {
      setBusy(false);
    }
  };

  const disconnect = async () => {
    setBusy(true);
    onFlash("", "");
    try {
      const s = await api<TailscaleStatus>("/api/settings/tailscale/disconnect", { method: "POST" });
      setStatus(s);
      onFlash("", "Disconnected from Tailscale. LAN access is unchanged.");
    } catch (e) {
      onFlash(e instanceof Error ? e.message : "Disconnect failed", "");
    } finally {
      setBusy(false);
    }
  };

  const forget = async () => {
    if (!window.confirm("Forget this Tailscale node? You will need a new auth key to connect again.")) {
      return;
    }
    setBusy(true);
    onFlash("", "");
    try {
      const s = await api<TailscaleStatus>("/api/settings/tailscale/forget", { method: "POST" });
      setStatus(s);
      setAuthKey("");
      onFlash("", "Tailscale node forgotten.");
    } catch (e) {
      onFlash(e instanceof Error ? e.message : "Forget failed", "");
    } finally {
      setBusy(false);
    }
  };

  const stateLabel = () => {
    if (loading && !status) return "Loading…";
    if (status?.connected) return "Connected";
    if (status?.running) return status.backendState || "Connecting…";
    return "Disconnected";
  };

  return (
    <>
      <header className="settings-panel-head">
        <h2>Remote access</h2>
        <p className="muted">
          Installs system Tailscale on this appliance when you connect with an auth key. That lets
          you open the UI from your Tailnet <em>and</em> reach website/DB hosts on{" "}
          <code>100.x</code> addresses.
        </p>
      </header>

      <div className="settings-form">
        <div className={`callout ${status?.connected ? "ok" : "warn"}`}>
          <strong>{stateLabel()}</strong>
          {status?.mode && (
            <p className="small muted">
              Mode: {status.mode}
              {status.mode === "userspace" ? " (SOCKS for 100.x dials; enable TUN on Proxmox for native routing)" : ""}
              {status.mode === "tun" ? " (kernel, native 100.x routing)" : ""}
            </p>
          )}
          {status?.dnsName && (
            <p className="small">
              MagicDNS: <code>{status.dnsName}</code>
            </p>
          )}
          {status?.ips && status.ips.length > 0 && (
            <p className="small muted">IPs: {status.ips.join(", ")}</p>
          )}
          {status?.urls && status.urls.length > 0 && (
            <ul className="plain small">
              {status.urls.map((u) => (
                <li key={u}>
                  <a href={u} target="_blank" rel="noreferrer">
                    {u}
                  </a>
                </li>
              ))}
            </ul>
          )}
          {status?.lastError && <p className="small err">{status.lastError}</p>}
        </div>

        <label>
          Hostname on Tailnet
          <input
            value={hostname}
            onChange={(e) => setHostname(e.target.value)}
            placeholder="boomerang"
            disabled={busy || !!status?.connected}
          />
        </label>

        <label>
          Auth key {status?.hasAuthKey || status?.hasState ? "(blank = keep / use saved state)" : ""}
          <input
            type="password"
            value={authKey}
            onChange={(e) => setAuthKey(e.target.value)}
            placeholder="tskey-auth-…"
            autoComplete="off"
            disabled={busy || !!status?.connected}
          />
        </label>

        <p className="muted small">
          Create an auth key in the{" "}
          <a href="https://login.tailscale.com/admin/settings/keys" target="_blank" rel="noreferrer">
            Tailscale admin console
          </a>
          . Prefer a reusable or tagged key. Connect installs{" "}
          <code>tailscaled</code> on this CT if needed.
        </p>

        <div className="settings-form-actions">
          {!status?.connected ? (
            <button type="button" disabled={busy || loading} onClick={() => void connect()}>
              Connect
            </button>
          ) : (
            <button type="button" className="ghost" disabled={busy} onClick={() => void disconnect()}>
              Disconnect
            </button>
          )}
          <button type="button" className="ghost danger-text" disabled={busy} onClick={() => void forget()}>
            Forget node
          </button>
          <button type="button" className="ghost" disabled={busy || loading} onClick={() => void refresh()}>
            Refresh status
          </button>
        </div>
      </div>
    </>
  );
}
