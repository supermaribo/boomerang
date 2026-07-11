import { FormEvent, useEffect, useState } from "react";
import { api } from "../lib/api";
import { useTimezone } from "../context/Timezone";
import { formatApplianceDateTime } from "../lib/formatTime";

export type OffsiteSettings = {
  enabled: boolean;
  accountId: string;
  bucket: string;
  prefix: string;
  hasAccessKey: boolean;
  hasSecretKey: boolean;
  lastSync: string;
  lastError: string;
  lastFiles: number;
  lastBytes: number;
  syncing: boolean;
};

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

type Props = {
  onFlash: (error: string, info: string) => void;
  busy: boolean;
  setBusy: (v: boolean) => void;
};

export default function OffsiteSettingsPanel({ onFlash, busy, setBusy }: Props) {
  const { timezone } = useTimezone();
  const [form, setForm] = useState<OffsiteSettings>({
    enabled: false,
    accountId: "",
    bucket: "",
    prefix: "boomerang",
    hasAccessKey: false,
    hasSecretKey: false,
    lastSync: "",
    lastError: "",
    lastFiles: 0,
    lastBytes: 0,
    syncing: false,
  });
  const [accessKey, setAccessKey] = useState("");
  const [secretKey, setSecretKey] = useState("");

  const load = () =>
    api<OffsiteSettings>("/api/offsite").then((s) => {
      setForm({
        ...s,
        prefix: s.prefix || "boomerang",
      });
    });

  useEffect(() => {
    load().catch((e) => onFlash(e instanceof Error ? e.message : "Failed to load off-site settings", ""));
  }, []);

  useEffect(() => {
    if (!form.syncing) return;
    const t = window.setInterval(() => {
      void load();
    }, 3000);
    return () => window.clearInterval(t);
  }, [form.syncing]);

  const payload = () => ({
    enabled: form.enabled,
    accountId: form.accountId,
    bucket: form.bucket,
    prefix: form.prefix || "boomerang",
    accessKey: accessKey || undefined,
    secretKey: secretKey || undefined,
  });

  const save = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    onFlash("", "");
    try {
      const next = await api<OffsiteSettings>("/api/offsite", {
        method: "PUT",
        body: JSON.stringify(payload()),
      });
      setForm(next);
      setAccessKey("");
      setSecretKey("");
      onFlash("", "Off-site settings saved.");
    } catch (err) {
      onFlash(err instanceof Error ? err.message : "Save failed", "");
    } finally {
      setBusy(false);
    }
  };

  const test = async () => {
    setBusy(true);
    onFlash("", "");
    try {
      await api("/api/offsite", { method: "PUT", body: JSON.stringify(payload()) });
      await api("/api/offsite/test", { method: "POST", body: JSON.stringify(payload()) });
      onFlash("", "Connection to R2 succeeded.");
      await load();
    } catch (err) {
      onFlash(err instanceof Error ? err.message : "Connection test failed", "");
    } finally {
      setBusy(false);
    }
  };

  const syncNow = async () => {
    setBusy(true);
    onFlash("", "");
    try {
      await api("/api/offsite/sync", { method: "POST" });
      setForm((f) => ({ ...f, syncing: true }));
      onFlash("", "Off-site mirror started.");
      await load();
    } catch (err) {
      onFlash(err instanceof Error ? err.message : "Sync failed", "");
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <header className="settings-panel-head">
        <h2>Off-site mirror (Cloudflare R2)</h2>
        <p className="muted">
          After each local backup, Boomerang mirrors <code>/var/lib/boomerang</code> to R2 — your
          off-site copy for 3-2-1. Backup files are already encrypted; the master key is included so
          you can restore on a new appliance.
        </p>
      </header>

      <div className="settings-blocks">
        <div className="settings-block callout">
          <h3>R2 setup</h3>
          <ol className="plain numbered compact">
            <li>
              <a href="https://dash.cloudflare.com/" target="_blank" rel="noreferrer">
                Cloudflare dashboard
              </a>{" "}
              → <strong>Storage &amp; databases</strong> → <strong>R2</strong> →{" "}
              <strong>Overview</strong> → create a private bucket.
            </li>
            <li>
              On that Overview page, copy your <strong>Account ID</strong> from Account details.
            </li>
            <li>
              Click <strong>Manage</strong> next to <strong>R2 API tokens</strong> →{" "}
              <strong>Create API token</strong> → <strong>Object Read &amp; Write</strong> → limit
              to your bucket → create.
            </li>
            <li>
              Copy <strong>Access Key ID</strong> and <strong>Secret Access Key</strong> immediately
              — the secret is only shown once. These are S3 API keys, not your Cloudflare login.
            </li>
          </ol>
          <p className="muted small">
            R2&apos;s free tier includes 10 GB storage and generous egress-free bandwidth — good for
            appliance DR.
          </p>
        </div>

        {form.lastSync && (
          <div className="settings-block">
            <h3>Last mirror</h3>
            <p className="muted small">
              {formatApplianceDateTime(form.lastSync, timezone)}
              {form.syncing ? " · syncing now…" : ""}
            </p>
            <p className="muted small">
              {form.lastFiles} file(s) · {fmtBytes(form.lastBytes)}
            </p>
            {form.lastError && <p className="err small">{form.lastError}</p>}
          </div>
        )}
      </div>

      <form className="settings-form" onSubmit={save}>
        <label className="check">
          <input
            type="checkbox"
            checked={form.enabled}
            onChange={(e) => setForm((f) => ({ ...f, enabled: e.target.checked }))}
          />
          Enable off-site mirror after backups
        </label>

        <label>Cloudflare account ID</label>
        <input
          value={form.accountId}
          onChange={(e) => setForm((f) => ({ ...f, accountId: e.target.value }))}
          placeholder="32-character account id"
          autoComplete="off"
        />

        <label>Bucket name</label>
        <input
          value={form.bucket}
          onChange={(e) => setForm((f) => ({ ...f, bucket: e.target.value }))}
          placeholder="boomerang-dr"
          autoComplete="off"
        />

        <label>Object prefix (folder in bucket)</label>
        <input
          value={form.prefix}
          onChange={(e) => setForm((f) => ({ ...f, prefix: e.target.value }))}
          placeholder="boomerang"
          autoComplete="off"
        />

        <div className="row2">
          <div>
            <label>Access key ID {form.hasAccessKey ? "(blank = keep)" : ""}</label>
            <input
              value={accessKey}
              onChange={(e) => setAccessKey(e.target.value)}
              autoComplete="off"
            />
          </div>
          <div>
            <label>Secret access key {form.hasSecretKey ? "(blank = keep)" : ""}</label>
            <input
              type="password"
              value={secretKey}
              onChange={(e) => setSecretKey(e.target.value)}
              autoComplete="new-password"
            />
          </div>
        </div>

        <div className="settings-form-actions">
          <button type="submit" disabled={busy}>
            Save
          </button>
          <button type="button" className="ghost" disabled={busy} onClick={() => void test()}>
            Test connection
          </button>
          <button
            type="button"
            className="ghost"
            disabled={busy || !form.enabled || form.syncing}
            onClick={() => void syncNow()}
          >
            {form.syncing ? "Syncing…" : "Mirror now"}
          </button>
        </div>
      </form>
    </>
  );
}
