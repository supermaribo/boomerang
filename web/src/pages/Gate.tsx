import { FormEvent, useEffect, useState } from "react";
import { api } from "../lib/api";
import SiteFooter from "../components/SiteFooter";

type Props = {
  setupRequired: boolean;
  statusError?: string;
  onDone: () => Promise<void>;
};

type SetupMode = "new" | "restore";

export default function Gate({ setupRequired, statusError, onDone }: Props) {
  const [mode, setMode] = useState<SetupMode>("new");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [accountId, setAccountId] = useState("");
  const [bucket, setBucket] = useState("");
  const [prefix, setPrefix] = useState("boomerang");
  const [accessKey, setAccessKey] = useState("");
  const [secretKey, setSecretKey] = useState("");
  const [restoreInfo, setRestoreInfo] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!setupRequired) {
      setMode("new");
    }
  }, [setupRequired]);

  const submitNew = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setRestoreInfo("");
    if (password !== confirm) {
      setError("Passwords do not match");
      return;
    }
    setBusy(true);
    try {
      await api("/api/setup", {
        method: "POST",
        body: JSON.stringify({ password }),
      });
      await onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed");
    } finally {
      setBusy(false);
    }
  };

  const submitRestore = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setRestoreInfo("");
    setBusy(true);
    try {
      const res = await api<{
        ok: boolean;
        message?: string;
        files?: number;
        bytes?: number;
      }>("/api/setup/restore-r2", {
        method: "POST",
        body: JSON.stringify({
          accountId,
          bucket,
          prefix: prefix || "boomerang",
          accessKey,
          secretKey,
        }),
      });
      setRestoreInfo(
        res.message ||
          `Restored ${res.files ?? 0} file(s). Waiting for the service to restart…`,
      );
      for (let i = 0; i < 40; i++) {
        await new Promise((r) => setTimeout(r, 1500));
        try {
          const st = await api<{ setupRequired: boolean }>("/api/status");
          if (!st.setupRequired) {
            setRestoreInfo("Restore complete — sign in with your previous admin password.");
            return;
          }
        } catch {
          // service restarting
        }
      }
      setRestoreInfo("Restore finished. Refresh the page and sign in with your previous password.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Restore failed");
    } finally {
      setBusy(false);
    }
  };

  const testRestore = async () => {
    setError("");
    setRestoreInfo("");
    setBusy(true);
    try {
      await api("/api/setup/test-r2", {
        method: "POST",
        body: JSON.stringify({
          accountId,
          bucket,
          prefix: prefix || "boomerang",
          accessKey,
          secretKey,
        }),
      });
      setRestoreInfo("Connection to R2 succeeded.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Connection test failed");
    } finally {
      setBusy(false);
    }
  };

  const submitLogin = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await api("/api/login", {
        method: "POST",
        body: JSON.stringify({ password }),
      });
      await onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Request failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell center">
      <div className="atmosphere" aria-hidden />
      <div className="gate-portal">
        <form
          className="card gate"
          onSubmit={
            statusError
              ? (e) => e.preventDefault()
              : setupRequired
              ? mode === "new"
                ? submitNew
                : submitRestore
              : submitLogin
          }
        >
          <p className="brand">Boomerang</p>
          <h1>{statusError ? "Connection problem" : setupRequired ? "First flight" : "Welcome back"}</h1>
          <p className="lede">
            {statusError
              ? "The appliance did not respond. Check that the Boomerang service is running and try again."
              : setupRequired
              ? mode === "new"
                ? "Set the admin password for a new appliance."
                : "Import a previous appliance from your Cloudflare R2 mirror."
              : "Sign in to manage websites, databases, and rollbacks."}
          </p>

          {statusError ? (
            <>
              <div className="err" role="alert">
                {statusError}
              </div>
              <button type="button" disabled={busy} onClick={() => void onDone()}>
                {busy ? "Retrying…" : "Retry"}
              </button>
            </>
          ) : (
            <>
          {setupRequired && (
            <>
              <div className="segmented gate-mode">
                <button
                  type="button"
                  className={mode === "new" ? "active" : ""}
                  onClick={() => {
                    setMode("new");
                    setError("");
                    setRestoreInfo("");
                  }}
                >
                  New appliance
                </button>
                <button
                  type="button"
                  className={mode === "restore" ? "active" : ""}
                  onClick={() => {
                    setMode("restore");
                    setError("");
                    setRestoreInfo("");
                  }}
                >
                  Restore from R2
                </button>
              </div>
              <aside className="callout warn setup-security" role="note">
                <strong>Internal use only.</strong> Keep port <code>8080</code> on your LAN or VPN
                only — no public internet exposure.
              </aside>
            </>
          )}

          <div className="err" role="alert">
            {error}
          </div>
          {restoreInfo && (
            <p className="ok pad small" role="status">
              {restoreInfo}
            </p>
          )}

          {setupRequired && mode === "restore" ? (
            <>
              <label htmlFor="accountId">Cloudflare account ID</label>
              <input
                id="accountId"
                value={accountId}
                onChange={(e) => setAccountId(e.target.value)}
                required
                autoComplete="off"
              />
              <label htmlFor="bucket">Bucket name</label>
              <input
                id="bucket"
                value={bucket}
                onChange={(e) => setBucket(e.target.value)}
                required
                autoComplete="off"
              />
              <label htmlFor="prefix">Object prefix</label>
              <input
                id="prefix"
                value={prefix}
                onChange={(e) => setPrefix(e.target.value)}
                placeholder="boomerang"
                autoComplete="off"
              />
              <label htmlFor="accessKey">Access key ID</label>
              <input
                id="accessKey"
                value={accessKey}
                onChange={(e) => setAccessKey(e.target.value)}
                required
                autoComplete="off"
              />
              <label htmlFor="secretKey">Secret access key</label>
              <input
                id="secretKey"
                type="password"
                value={secretKey}
                onChange={(e) => setSecretKey(e.target.value)}
                required
                autoComplete="new-password"
              />
              <div className="gate-actions">
                <button type="button" className="ghost" disabled={busy} onClick={() => void testRestore()}>
                  Test connection
                </button>
                <button type="submit" disabled={busy}>
                  {busy ? "Restoring…" : "Restore appliance"}
                </button>
              </div>
            </>
          ) : (
            <>
              <label htmlFor="password">Password</label>
              <input
                id="password"
                type="password"
                autoComplete={setupRequired ? "new-password" : "current-password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                minLength={setupRequired ? 8 : 1}
              />
              {setupRequired && (
                <>
                  <label htmlFor="confirm">Confirm</label>
                  <input
                    id="confirm"
                    type="password"
                    autoComplete="new-password"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                    required
                    minLength={8}
                  />
                </>
              )}
              <button type="submit" disabled={busy}>
                {busy ? "Working…" : setupRequired ? "Create admin" : "Sign in"}
              </button>
            </>
          )}
            </>
          )}
        </form>
        <SiteFooter portal />
      </div>
    </div>
  );
}
