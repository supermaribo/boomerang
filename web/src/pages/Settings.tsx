import { FormEvent, useEffect, useState } from "react";
import { api } from "../App";
import { useTimezone } from "../context/Timezone";
import { guessBrowserTimezone, timezoneLabel } from "../lib/formatTime";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import DisasterRecovery from "../components/DisasterRecovery";
import OffsiteSettingsPanel from "../components/OffsiteSettings";
import UpdateSettings from "../components/UpdateSettings";

type Settings = {
  mailMode: "local" | "smtp";
  notifyTo: string;
  notifyFrom: string;
  smtpHost: string;
  smtpPort: number;
  smtpUser: string;
  smtpFrom: string;
  smtpTo: string;
  hasSmtpPassword: boolean;
  alertBackupSuccess: boolean;
  alertBackupFailure: boolean;
  alertRestoreSuccess: boolean;
  alertRestoreFailure: boolean;
  alertOffsiteFailure: boolean;
  timezone?: string;
};

type Tab = "account" | "notifications" | "offsite" | "recovery" | "updates";

const TABS: { id: Tab; label: string; hint: string }[] = [
  { id: "account", label: "Account", hint: "Login password" },
  { id: "notifications", label: "Notifications", hint: "Email alerts" },
  { id: "offsite", label: "Off-site", hint: "Cloudflare R2 mirror" },
  { id: "recovery", label: "Recovery", hint: "Protect this server" },
  { id: "updates", label: "Updates", hint: "Software releases" },
];

const ALERTS: {
  key: keyof Pick<
    Settings,
    | "alertBackupFailure"
    | "alertBackupSuccess"
    | "alertRestoreFailure"
    | "alertRestoreSuccess"
    | "alertOffsiteFailure"
  >;
  label: string;
  desc: string;
}[] = [
  { key: "alertBackupFailure", label: "Backup failed", desc: "When a scheduled or manual backup errors" },
  { key: "alertBackupSuccess", label: "Backup succeeded", desc: "After each successful backup job" },
  { key: "alertRestoreFailure", label: "Restore failed", desc: "When a restore job errors" },
  { key: "alertRestoreSuccess", label: "Restore succeeded", desc: "After a restore completes" },
  { key: "alertOffsiteFailure", label: "Off-site mirror failed", desc: "When R2 sync fails (deduplicated per error)" },
];

export default function SettingsPage() {
  const { timezone, setTimezone: setGlobalTimezone, refreshTimezone } = useTimezone();
  const [tab, setTab] = useState<Tab>("account");

  useEffect(() => {
    const p = new URLSearchParams(window.location.search).get("tab");
    if (p === "account" || p === "notifications" || p === "offsite" || p === "recovery" || p === "updates") {
      setTab(p);
    }
  }, []);

  const [form, setForm] = useState<Settings>({
    mailMode: "local",
    notifyTo: "",
    notifyFrom: "",
    smtpHost: "",
    smtpPort: 587,
    smtpUser: "",
    smtpFrom: "",
    smtpTo: "",
    hasSmtpPassword: false,
    alertBackupSuccess: false,
    alertBackupFailure: true,
    alertRestoreSuccess: false,
    alertRestoreFailure: true,
    alertOffsiteFailure: true,
  });
  const [smtpPassword, setSmtpPassword] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [applianceTimezone, setApplianceTimezone] = useState("UTC");
  const [timezoneOptions, setTimezoneOptions] = useState<string[]>(["UTC"]);
  const [customTimezone, setCustomTimezone] = useState("");

  useEffect(() => {
    api<{ timezone: string; common: string[] }>("/api/settings/timezones")
      .then((d) => {
        setTimezoneOptions(d.common?.length ? d.common : ["UTC"]);
        if (d.timezone) setApplianceTimezone(d.timezone);
      })
      .catch(() => undefined);
  }, []);

  useEffect(() => {
    setApplianceTimezone(timezone);
  }, [timezone]);

  useEffect(() => {
    api<Settings>("/api/settings")
      .then((s) =>
        setForm({
          mailMode: s.mailMode === "smtp" ? "smtp" : "local",
          notifyTo: s.notifyTo || s.smtpTo || "",
          notifyFrom: s.notifyFrom || "",
          smtpHost: s.smtpHost || "",
          smtpPort: s.smtpPort || 587,
          smtpUser: s.smtpUser || "",
          smtpFrom: s.smtpFrom || "",
          smtpTo: s.smtpTo || "",
          hasSmtpPassword: s.hasSmtpPassword,
          alertBackupSuccess: s.alertBackupSuccess ?? false,
          alertBackupFailure: s.alertBackupFailure ?? true,
          alertRestoreSuccess: s.alertRestoreSuccess ?? false,
          alertRestoreFailure: s.alertRestoreFailure ?? true,
          alertOffsiteFailure: s.alertOffsiteFailure ?? true,
        }),
      )
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load settings"));
  }, []);

  const saveTimezone = async () => {
    const tz = customTimezone.trim() || applianceTimezone;
    setBusy(true);
    setError("");
    setInfo("");
    try {
      const res = await api<{ timezone: string }>("/api/settings/timezone", {
        method: "PUT",
        body: JSON.stringify({ timezone: tz }),
      });
      setApplianceTimezone(res.timezone);
      setCustomTimezone("");
      setGlobalTimezone(res.timezone);
      await refreshTimezone();
      setInfo(`Timezone set to ${res.timezone}. Backup schedules use this zone.`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save timezone");
    } finally {
      setBusy(false);
    }
  };

  const set = <K extends keyof Settings>(k: K, v: Settings[K]) =>
    setForm((f) => ({ ...f, [k]: v }));

  const persistNotifications = async () => {
    await api("/api/settings", {
      method: "PUT",
      body: JSON.stringify({ ...form, smtpPassword: smtpPassword || undefined }),
    });
    setSmtpPassword("");
    const next = await api<Settings>("/api/settings");
    setForm((f) => ({
      ...f,
      ...next,
      mailMode: next.mailMode === "smtp" ? "smtp" : "local",
      notifyTo: next.notifyTo || next.smtpTo || f.notifyTo,
      hasSmtpPassword: next.hasSmtpPassword,
      alertBackupSuccess: next.alertBackupSuccess ?? false,
      alertBackupFailure: next.alertBackupFailure ?? true,
      alertRestoreSuccess: next.alertRestoreSuccess ?? false,
      alertRestoreFailure: next.alertRestoreFailure ?? true,
      alertOffsiteFailure: next.alertOffsiteFailure ?? true,
    }));
  };

  const saveNotifications = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError("");
    setInfo("");
    try {
      await persistNotifications();
      setInfo("Notification settings saved.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  };

  const testEmail = async () => {
    setBusy(true);
    setError("");
    setInfo("");
    try {
      await persistNotifications();
      await api("/api/settings/test-email", { method: "POST" });
      setInfo("Test email sent — check your inbox.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Test email failed");
    } finally {
      setBusy(false);
    }
  };

  const changePassword = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setInfo("");
    if (newPassword !== confirmPassword) {
      setError("New passwords do not match");
      return;
    }
    setBusy(true);
    try {
      await api("/api/settings/password", {
        method: "POST",
        body: JSON.stringify({ currentPassword, newPassword }),
      });
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setInfo("Password updated.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Password change failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="shell">
      <Nav />
      <header className="page-head">
        <h1>Settings</h1>
        <p className="muted">Account security, alerts, and appliance recovery</p>
      </header>

      {(error || info) && (
        <div className="settings-flash">
          {error && <p className="err pad">{error}</p>}
          {info && <p className="ok pad">{info}</p>}
        </div>
      )}

      <div className="settings-layout">
        <nav className="settings-tabs" aria-label="Settings sections">
          {TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              className={tab === t.id ? "settings-tab active" : "settings-tab"}
              onClick={() => setTab(t.id)}
            >
              <span className="settings-tab-label">{t.label}</span>
              <span className="settings-tab-hint">{t.hint}</span>
            </button>
          ))}
        </nav>

        <div className="settings-main tile">
          {tab === "account" && (
            <>
              <header className="settings-panel-head">
                <h2>Account</h2>
                <p className="muted">Password and how dates and backup schedules are shown.</p>
              </header>

              <div className="account-settings-grid">
                <section className="account-settings-card">
                  <h3 className="wizard-section-title">Timezone</h3>
                  <p className="muted small account-card-lead">
                    Backup schedules run in this timezone. Timestamps in the UI use it too.
                  </p>
                  <div className="settings-form">
                    <label htmlFor="appliance-timezone">Timezone</label>
                    <select
                      id="appliance-timezone"
                      value={applianceTimezone}
                      onChange={(e) => {
                        setApplianceTimezone(e.target.value || "UTC");
                        setCustomTimezone("");
                      }}
                    >
                      {timezoneOptions.map((z) => (
                        <option key={z} value={z}>
                          {timezoneLabel(z)}
                        </option>
                      ))}
                      {!timezoneOptions.includes(applianceTimezone) && (
                        <option value={applianceTimezone}>{timezoneLabel(applianceTimezone)}</option>
                      )}
                    </select>
                    <label htmlFor="custom-timezone">Or IANA name</label>
                    <input
                      id="custom-timezone"
                      value={customTimezone}
                      onChange={(e) => setCustomTimezone(e.target.value)}
                      placeholder={guessBrowserTimezone()}
                      list="tz-suggestions"
                    />
                    <datalist id="tz-suggestions">
                      {timezoneOptions.map((z) => (
                        <option key={z} value={z} />
                      ))}
                    </datalist>
                    <div className="settings-form-actions">
                      <button type="button" disabled={busy} onClick={() => void saveTimezone()}>
                        Save timezone
                      </button>
                    </div>
                  </div>
                </section>

                <form className="account-settings-card settings-form" onSubmit={changePassword}>
                  <h3 className="wizard-section-title">Password</h3>
                  <p className="muted small account-card-lead">Change the password used to sign in to this appliance.</p>
                  <label>Current password</label>
                  <input
                    type="password"
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    required
                    autoComplete="current-password"
                  />
                  <div className="row2">
                    <div>
                      <label>New password</label>
                      <input
                        type="password"
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        required
                        minLength={8}
                        autoComplete="new-password"
                      />
                    </div>
                    <div>
                      <label>Confirm</label>
                      <input
                        type="password"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        required
                        minLength={8}
                        autoComplete="new-password"
                      />
                    </div>
                  </div>
                  <div className="settings-form-actions">
                    <button type="submit" disabled={busy}>
                      Update password
                    </button>
                  </div>
                </form>
              </div>
            </>
          )}

          {tab === "notifications" && (
            <>
              <header className="settings-panel-head">
                <h2>Email notifications</h2>
                <p className="muted">Choose which job events send email and how mail is delivered.</p>
              </header>
              <form className="settings-form" onSubmit={saveNotifications}>
                <fieldset className="settings-fieldset">
                  <legend>Recipient</legend>
                  <label>Notify address</label>
                  <input
                    type="email"
                    value={form.notifyTo}
                    onChange={(e) => set("notifyTo", e.target.value)}
                    placeholder="you@example.com"
                    required
                  />
                </fieldset>

                <fieldset className="settings-fieldset">
                  <legend>Send an email when…</legend>
                  <div className="alert-cards">
                    {ALERTS.map((a) => (
                      <label key={a.key} className="alert-card">
                        <input
                          type="checkbox"
                          checked={form[a.key]}
                          onChange={(e) => set(a.key, e.target.checked)}
                        />
                        <span className="alert-card-text">
                          <strong>{a.label}</strong>
                          <span className="muted small">{a.desc}</span>
                        </span>
                      </label>
                    ))}
                  </div>
                </fieldset>

                <fieldset className="settings-fieldset">
                  <legend>Delivery</legend>
                  <div className="segmented">
                    <button
                      type="button"
                      className={form.mailMode === "local" ? "active" : ""}
                      onClick={() => set("mailMode", "local")}
                    >
                      Local mail
                    </button>
                    <button
                      type="button"
                      className={form.mailMode === "smtp" ? "active" : ""}
                      onClick={() => set("mailMode", "smtp")}
                    >
                      Custom SMTP
                    </button>
                  </div>

                  {form.mailMode === "local" ? (
                    <div className="delivery-panel">
                      <p className="muted small">
                        Delivers via postfix on <code>127.0.0.1:25</code> (installed by default on
                        Debian). Best for addresses on this server (e.g. <code>root@localhost</code>
                        ). For Gmail or other external inboxes, use <strong>Custom SMTP</strong>.
                      </p>
                      <label>From address (optional)</label>
                      <input
                        value={form.notifyFrom}
                        onChange={(e) => set("notifyFrom", e.target.value)}
                        placeholder="boomerang@your-server"
                      />
                    </div>
                  ) : (
                    <div className="delivery-panel">
                      <div className="row2">
                        <div>
                          <label>SMTP host</label>
                          <input
                            value={form.smtpHost}
                            onChange={(e) => set("smtpHost", e.target.value)}
                            placeholder="smtp.example.com"
                          />
                        </div>
                        <div>
                          <label>Port</label>
                          <input
                            type="number"
                            value={form.smtpPort}
                            onChange={(e) => set("smtpPort", Number(e.target.value))}
                          />
                        </div>
                      </div>
                      <div className="row2">
                        <div>
                          <label>Username</label>
                          <input
                            value={form.smtpUser}
                            onChange={(e) => set("smtpUser", e.target.value)}
                          />
                        </div>
                        <div>
                          <label>
                            Password {form.hasSmtpPassword ? "(blank = keep)" : ""}
                          </label>
                          <input
                            type="password"
                            value={smtpPassword}
                            onChange={(e) => setSmtpPassword(e.target.value)}
                            autoComplete="new-password"
                          />
                        </div>
                      </div>
                      <label>From address</label>
                      <input
                        value={form.smtpFrom}
                        onChange={(e) => set("smtpFrom", e.target.value)}
                        placeholder="alerts@example.com"
                      />
                    </div>
                  )}
                </fieldset>

                <div className="settings-form-actions">
                  <button type="submit" disabled={busy}>
                    Save notifications
                  </button>
                  <button type="button" className="ghost" disabled={busy} onClick={() => void testEmail()}>
                    Send test email
                  </button>
                </div>
              </form>
            </>
          )}

          {tab === "offsite" && (
            <OffsiteSettingsPanel
              busy={busy}
              setBusy={setBusy}
              onFlash={(err, inf) => {
                setError(err);
                setInfo(inf);
              }}
            />
          )}

          {tab === "recovery" && (
            <>
              <header className="settings-panel-head">
                <h2>Disaster recovery</h2>
              </header>
              <DisasterRecovery embedded />
            </>
          )}

          {tab === "updates" && (
            <UpdateSettings
              busy={busy}
              setBusy={setBusy}
              onFlash={(err, inf) => {
                setError(err);
                setInfo(inf);
              }}
            />
          )}
        </div>
      </div>
      <SiteFooter />
    </div>
  );
}
