import { useCallback, useEffect, useState } from "react";
import { api } from "../App";

type UpdateCheck = {
  currentVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  releaseUrl?: string;
  releaseNotes?: string;
  publishedAt?: string;
  assetName?: string;
  assetBytes?: number;
  canApply: boolean;
  checkError?: string;
};

function formatBytes(n: number) {
  if (!n) return "";
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function sleep(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

type Props = {
  busy: boolean;
  setBusy: (v: boolean) => void;
  onFlash: (error: string, info: string) => void;
};

export default function UpdateSettings({ busy, setBusy, onFlash }: Props) {
  const [check, setCheck] = useState<UpdateCheck | null>(null);
  const [checking, setChecking] = useState(false);
  const [updating, setUpdating] = useState(false);

  const runCheck = useCallback(async () => {
    setChecking(true);
    onFlash("", "");
    try {
      const res = await api<UpdateCheck>("/api/update/check");
      setCheck(res);
    } catch (err) {
      onFlash(err instanceof Error ? err.message : "Update check failed", "");
      setCheck(null);
    } finally {
      setChecking(false);
    }
  }, [onFlash]);

  useEffect(() => {
    void runCheck();
  }, [runCheck]);

  const applyUpdate = async () => {
    if (!check?.updateAvailable) return;
    if (
      !window.confirm(
        `Install Boomerang ${check.latestVersion}? The appliance will restart and this page will reload.`,
      )
    ) {
      return;
    }
    setBusy(true);
    setUpdating(true);
    onFlash("", "Downloading and installing update…");
    try {
      await api("/api/update/apply", { method: "POST" });
      onFlash("", "Update installed. Waiting for the appliance to come back…");
      for (let i = 0; i < 45; i++) {
        await sleep(2000);
        try {
          await api<{ product: string }>("/api/status");
          window.location.reload();
          return;
        } catch {
          /* still restarting */
        }
      }
      onFlash("", "Update may have finished — refresh the page if it does not reload automatically.");
    } catch (err) {
      onFlash(err instanceof Error ? err.message : "Update failed", "");
    } finally {
      setBusy(false);
      setUpdating(false);
    }
  };

  return (
    <>
      <header className="settings-panel-head">
        <h2>Software updates</h2>
        <p className="muted">
          Check GitHub for new releases and install them on this appliance.
        </p>
      </header>

      <div className="settings-form">
        <dl className="update-versions">
          <dt>Installed version</dt>
          <dd>
            <code>{check?.currentVersion ?? "…"}</code>
          </dd>
          <dt>Latest release</dt>
          <dd>
            {checking && !check ? (
              <span className="muted">Checking…</span>
            ) : check?.checkError ? (
              <span className="muted">{check.checkError}</span>
            ) : (
              <>
                <code>{check?.latestVersion ?? "—"}</code>
                {check?.publishedAt && (
                  <span className="muted small">
                    {" "}
                    · {new Date(check.publishedAt).toLocaleDateString()}
                  </span>
                )}
              </>
            )}
          </dd>
        </dl>

        {check?.updateAvailable && !check.checkError && (
          <div className="callout ok">
            <strong>Update available</strong>
            <p className="small">
              {check.currentVersion} → {check.latestVersion}
              {check.assetName && (
                <>
                  {" "}
                  · {check.assetName}
                  {check.assetBytes ? ` (${formatBytes(check.assetBytes)})` : ""}
                </>
              )}
            </p>
            {check.releaseNotes && (
              <details className="update-notes">
                <summary>Release notes</summary>
                <pre>{check.releaseNotes}</pre>
              </details>
            )}
            {check.releaseUrl && (
              <p className="small">
                <a href={check.releaseUrl} target="_blank" rel="noreferrer">
                  View on GitHub
                </a>
              </p>
            )}
          </div>
        )}

        {check && !check.checkError && !check.updateAvailable && (
          <p className="ok pad">You are on the latest release.</p>
        )}

        {!check?.canApply && check && !check.checkError && check.updateAvailable && (
          <p className="callout warn small">
            One-click install is not available on this host. Re-run{" "}
            <code>sudo ./install.sh --no-build /path/to/boomerang</code> on the server instead.
          </p>
        )}

        <div className="settings-form-actions">
          <button type="button" className="ghost" disabled={busy || checking} onClick={() => void runCheck()}>
            {checking ? "Checking…" : "Check again"}
          </button>
          {check?.updateAvailable && check.canApply && (
            <button type="button" disabled={busy || updating} onClick={() => void applyUpdate()}>
              {updating ? "Installing…" : `Update to ${check.latestVersion}`}
            </button>
          )}
        </div>
      </div>
    </>
  );
}
