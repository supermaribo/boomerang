import { FormEvent, useEffect, useMemo, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../App";
import { useTimezone } from "../context/Timezone";
import Nav from "../components/Nav";
import SiteFooter from "../components/SiteFooter";
import FirewallReminder from "../components/FirewallReminder";
import ScheduleRetention, { retentionSummary } from "../components/ScheduleRetention";
import {
  ScheduleState,
  buildCron,
  randomNightSchedule,
  parseSchedule,
  scheduleStartISO,
} from "../lib/schedule";
import type { FileServer } from "./FileServers";

type DirEntry = { name: string; path: string; isDir: boolean; size: number };
type BrowseResult = { path: string; parent: string; entries: DirEntry[] };

const STEPS = ["Connection", "Authentication", "Paths", "Schedule & backup", "Review"] as const;

function commonParent(paths: string[]): string {
  if (paths.length === 0) return "/";
  if (paths.length === 1) return paths[0];
  const split = paths.map((p) => p.split("/").filter(Boolean));
  const out: string[] = [];
  for (let i = 0; ; i++) {
    const part = split[0][i];
    if (!part || !split.every((s) => s[i] === part)) break;
    out.push(part);
  }
  return out.length ? "/" + out.join("/") : "/";
}

const emptyForm = {
  name: "",
  protocol: "sftp",
  host: "",
  port: 22,
  username: "",
  remoteRoot: "/",
  includePaths: [] as string[],
  excludePaths: [] as string[],
  authMode: "key",
  password: "",
  privateKey: "",
  passphrase: "",
  publicKey: "",
  retainHourly: 1,
  retainDaily: 1,
  retainWeekly: 1,
  retainMonthly: 1,
  retainYearly: 1,
  incrementalEnabled: true,
  enabled: true,
};

export default function FileServerWizard() {
  const { timezone } = useTimezone();
  const { id } = useParams();
  const editing = Boolean(id);
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [form, setForm] = useState(emptyForm);
  const [schedule, setSchedule] = useState<ScheduleState>(() => randomNightSchedule(timezone));
  const [error, setError] = useState("");
  const [info, setInfo] = useState("");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState(false);
  const [browse, setBrowse] = useState<BrowseResult | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [loaded, setLoaded] = useState(!editing);

  const set = (k: string, v: string | number | boolean | string[]) =>
    setForm((f) => ({ ...f, [k]: v }));

  useEffect(() => {
    if (!id) return;
    api<FileServer>(`/api/file-servers/${id}`)
      .then((f) => {
        setForm({
          ...emptyForm,
          name: f.name,
          protocol: f.protocol,
          host: f.host,
          port: f.port,
          username: f.username,
          remoteRoot: f.remoteRoot,
          includePaths: f.includePaths || [],
          excludePaths: f.excludePaths || [],
          authMode: f.authMode,
          publicKey: f.publicKey || "",
          retainHourly: f.retainHourly ?? 1,
          retainDaily: f.retainDaily ?? 1,
          retainWeekly: f.retainWeekly ?? 1,
          retainMonthly: f.retainMonthly ?? 1,
          retainYearly: f.retainYearly ?? 1,
          incrementalEnabled: f.incrementalEnabled ?? true,
          enabled: f.enabled,
        });
        setSelected(
          f.includePaths?.length ? f.includePaths : f.remoteRoot ? [f.remoteRoot] : [],
        );
        setSchedule(parseSchedule(f.scheduleCron, f.scheduleStart || "", timezone));
        setLoaded(true);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "load failed"));
  }, [id]);

  useEffect(() => {
    if (editing || step !== 1) return;
    if (form.protocol === "ftp" || form.protocol === "ftps") return;
    if (form.authMode !== "key") return;
    if (form.publicKey || form.privateKey) return;
    void generateKey(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, form.authMode, form.protocol]);

  const generateKey = async (announce = true) => {
    setError("");
    try {
      const keys = await api<{ privateKey: string; publicKey: string }>("/api/keys/generate", {
        method: "POST",
      });
      setForm((f) => ({
        ...f,
        authMode: "key",
        privateKey: keys.privateKey,
        publicKey: keys.publicKey.trim(),
        password: "",
      }));
      setCopied(false);
      if (announce) setInfo("New keypair ready — copy the public key to the remote, then test.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "keygen failed");
    }
  };

  const copyPublicKey = async () => {
    if (!form.publicKey) return;
    try {
      await navigator.clipboard.writeText(form.publicKey.trim());
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      setError("Could not copy — select the key and copy manually");
    }
  };

  const credBody = () => ({
    protocol: form.protocol,
    host: form.host,
    port: form.port,
    username: form.username,
    authMode: form.authMode,
    password: form.password,
    privateKey: form.privateKey,
    passphrase: form.passphrase,
    remoteRoot: form.remoteRoot || "/",
  });

  const test = async () => {
    setBusy(true);
    setError("");
    setInfo("");
    try {
      if (editing && !form.password && !form.privateKey) {
        const res = await api<{ message: string }>(`/api/file-servers/${id}/test`, { method: "POST" });
        setInfo(res.message);
      } else {
        const res = await api<{ message: string }>("/api/file-servers/test", {
          method: "POST",
          body: JSON.stringify(credBody()),
        });
        setInfo(res.message);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "test failed");
    } finally {
      setBusy(false);
    }
  };

  const loadBrowse = async (path = "") => {
    setBusy(true);
    setError("");
    try {
      let res: BrowseResult;
      if (editing && !form.password && !form.privateKey) {
        res = await api<BrowseResult>(`/api/file-servers/${id}/browse`, {
          method: "POST",
          body: JSON.stringify({ path }),
        });
      } else {
        res = await api<BrowseResult>("/api/file-servers/browse", {
          method: "POST",
          body: JSON.stringify({ ...credBody(), path }),
        });
      }
      setBrowse(res);
    } catch (e) {
      setError(e instanceof Error ? e.message : "browse failed");
    } finally {
      setBusy(false);
    }
  };

  const togglePath = (p: string) => {
    setSelected((prev) => (prev.includes(p) ? prev.filter((x) => x !== p) : [...prev, p]));
  };

  const selectCurrent = () => {
    if (!browse?.path) return;
    setSelected((prev) => (prev.includes(browse.path) ? prev : [...prev, browse.path]));
  };

  const resolvedPaths = useMemo(() => {
    if (selected.length === 0) return { remoteRoot: "/", includePaths: [] as string[] };
    if (selected.length === 1) return { remoteRoot: selected[0], includePaths: [] as string[] };
    return { remoteRoot: commonParent(selected), includePaths: selected };
  }, [selected]);

  const canNext = () => {
    if (step === 0) return Boolean(form.name && form.host && form.username);
    if (step === 1) {
      if (form.protocol === "ftp" || form.protocol === "ftps" || form.authMode === "password") {
        return editing || Boolean(form.password);
      }
      return editing || Boolean(form.privateKey && form.publicKey);
    }
    if (step === 2) return selected.length > 0;
    return true;
  };

  const goToStep = (i: number) => {
    if (!editing && i > step) return;
    setStep(i);
    if (i === 2) {
      const start =
        selected[0]?.replace(/\/[^/]+$/, "") ||
        (form.remoteRoot !== "/" ? form.remoteRoot : "");
      void loadBrowse(start);
    }
  };

  const goNext = async () => {
    setError("");
    setInfo("");
    if (step < STEPS.length - 1) {
      const next = step + 1;
      setStep(next);
      if (next === 2) {
        const start =
          selected[0]?.replace(/\/[^/]+$/, "") ||
          (form.remoteRoot !== "/" ? form.remoteRoot : "");
        void loadBrowse(start);
      }
    }
  };

  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (selected.length === 0) {
      setError("Select at least one path to back up");
      setStep(2);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const body = {
        ...form,
        ...resolvedPaths,
        scheduleCron: buildCron(schedule),
        scheduleStart: scheduleStartISO(schedule, timezone),
        retainCount: 0,
        retainDays: 0,
      };
      if (editing) {
        await api(`/api/file-servers/${id}`, { method: "PUT", body: JSON.stringify(body) });
      } else {
        await api("/api/file-servers", { method: "POST", body: JSON.stringify(body) });
      }
      navigate("/app/websites");
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed");
    } finally {
      setBusy(false);
    }
  };

  const sshAuth = form.protocol !== "ftp" && form.protocol !== "ftps";

  if (!loaded) {
    return (
      <div className="shell">
        <Nav />
        <p className="muted">Loading…</p>
        <SiteFooter />
      </div>
    );
  }

  return (
    <div className="shell">
      <Nav />
      <header className="page-head">
        <h1>{editing ? "Edit website" : "Add website"}</h1>
        <p className="muted">
          Step {step + 1} of {STEPS.length}: {STEPS[step]}
        </p>
      </header>

      <ol className="wizard-steps">
        {STEPS.map((label, i) => (
          <li key={label} className={i === step ? "active" : i < step ? "done" : ""}>
            <button
              type="button"
              onClick={() => goToStep(i)}
              disabled={!editing && i > step}
              title={editing ? `Jump to ${label}` : undefined}
            >
              {label}
            </button>
          </li>
        ))}
      </ol>

      <section className="tile wizard-tile">
        <div className="err">{error}</div>
        {info && <p className="ok">{info}</p>}

        {step === 0 && (
          <>
            <label>Name</label>
            <input value={form.name} onChange={(e) => set("name", e.target.value)} required />
            <label>Protocol</label>
            <select
              value={form.protocol}
              onChange={(e) => {
                const p = e.target.value;
                setForm((f) => ({
                  ...f,
                  protocol: p,
                  port: p === "ftp" || p === "ftps" ? 21 : 22,
                  authMode: p === "ftp" || p === "ftps" ? "password" : f.authMode,
                  incrementalEnabled: p === "rsync" ? false : f.incrementalEnabled,
                }));
              }}
            >
              <option value="sftp">SFTP (recommended)</option>
              <option value="rsync">RSYNC over SSH</option>
              <option value="ftp">FTP</option>
              <option value="ftps">FTPS</option>
            </select>
            <div className="row2">
              <div>
                <label>Host</label>
                <input value={form.host} onChange={(e) => set("host", e.target.value)} required />
              </div>
              <div>
                <label>Port</label>
                <input
                  type="number"
                  value={form.port}
                  onChange={(e) => set("port", Number(e.target.value))}
                />
              </div>
            </div>
            <label>Username</label>
            <input value={form.username} onChange={(e) => set("username", e.target.value)} required />
            <p className="callout warn">
              This account must have <strong>read access</strong> to all paths you back up and{" "}
              <strong>write access</strong> where you may restore. If permissions are too tight, backups
              can be incomplete (files skipped) and restores may fail.
            </p>
            <FirewallReminder
              targetHost={form.host}
              port={form.port}
              protocol={
                form.protocol === "ftp"
                  ? "FTP"
                  : form.protocol === "ftps"
                    ? "FTPS"
                    : form.protocol === "rsync"
                      ? "SSH (RSYNC)"
                      : "SFTP"
              }
            />
          </>
        )}

        {step === 1 && (
          <>
            {sshAuth && (
              <>
                <label>Authentication</label>
                <select value={form.authMode} onChange={(e) => set("authMode", e.target.value)}>
                  <option value="key">SSH key (recommended)</option>
                  <option value="password">Password</option>
                </select>
              </>
            )}
            {(form.authMode === "password" || !sshAuth) && (
              <>
                <label>Password {editing ? "(leave blank to keep)" : ""}</label>
                <input
                  type="password"
                  value={form.password}
                  onChange={(e) => set("password", e.target.value)}
                  autoComplete="new-password"
                />
              </>
            )}
            {form.authMode === "key" && sshAuth && (
              <div className="key-panel">
                <h3>Install this public key on the remote</h3>
                <p className="muted small">
                  Add it for user <code>{form.username || "USER"}</code> in CloudPanel or{" "}
                  <code>~/.ssh/authorized_keys</code>.
                </p>
                <textarea className="pubkey" rows={3} readOnly value={form.publicKey} />
                <div className="key-actions">
                  <button type="button" className="secondary" onClick={() => void generateKey(true)}>
                    {form.publicKey ? "Regenerate key" : "Generate key"}
                  </button>
                  <button
                    type="button"
                    className="secondary"
                    disabled={!form.publicKey}
                    onClick={() => void copyPublicKey()}
                  >
                    {copied ? "Copied" : "Copy public key"}
                  </button>
                </div>
                {editing && (
                  <p className="hint">Leave private key blank when editing to keep the stored key.</p>
                )}
                {!editing && (
                  <details className="advanced">
                    <summary>Show private key</summary>
                    <textarea rows={4} value={form.privateKey} readOnly />
                  </details>
                )}
              </div>
            )}
            <div className="actions">
              <button type="button" className="ghost" disabled={busy} onClick={() => void test()}>
                Test connection
              </button>
            </div>
          </>
        )}

        {step === 2 && (
          <div className="path-picker">
            <p className="muted small">
              Browse the remote and tick directories (or files) to include. You can select multiple.
            </p>
            <div className="path-toolbar">
              <button
                type="button"
                className="secondary"
                disabled={!browse?.parent || busy}
                onClick={() => void loadBrowse(browse?.parent || "")}
              >
                ↑ Up
              </button>
              <code className="path-current">{browse?.path || "…"}</code>
              <button type="button" className="secondary" disabled={!browse || busy} onClick={selectCurrent}>
                Select this folder
              </button>
              <button type="button" className="secondary" disabled={busy} onClick={() => void loadBrowse("")}>
                Refresh
              </button>
            </div>
            <div className="path-list">
              {(browse?.entries || []).map((e) => (
                <label key={e.path} className={`path-row ${e.isDir ? "is-dir" : ""}`}>
                  <input
                    type="checkbox"
                    checked={selected.includes(e.path)}
                    onChange={() => togglePath(e.path)}
                  />
                  {e.isDir ? (
                    <button
                      type="button"
                      className="path-name"
                      onClick={() => void loadBrowse(e.path)}
                    >
                      {e.name}/
                    </button>
                  ) : (
                    <span className="path-name">{e.name}</span>
                  )}
                </label>
              ))}
              {browse && browse.entries.length === 0 && <p className="muted">Empty directory</p>}
            </div>
            <div className="selected-paths">
              <h3>Selected ({selected.length})</h3>
              {selected.length === 0 && <p className="muted small">Nothing selected yet</p>}
              <ul>
                {selected.map((p) => (
                  <li key={p}>
                    <code>{p}</code>
                    <button type="button" className="ghost" onClick={() => togglePath(p)}>
                      Remove
                    </button>
                  </li>
                ))}
              </ul>
            </div>
            <div className="exclude-paths">
              <h3>Exclude globs</h3>
              <p className="muted small">
                One pattern per line — e.g. <code>cache/</code>, <code>node_modules/</code>,{" "}
                <code>*.log</code>
              </p>
              <textarea
                rows={4}
                placeholder={"cache/\nnode_modules/\n*.log"}
                value={form.excludePaths.join("\n")}
                onChange={(e) =>
                  set(
                    "excludePaths",
                    e.target.value
                      .split("\n")
                      .map((s) => s.trim())
                      .filter(Boolean),
                  )
                }
              />
            </div>
          </div>
        )}

        {step === 3 && (
          <>
            {form.protocol === "rsync" ? (
              <p className="callout warn">
                RSYNC always captures a <strong>full snapshot</strong> each run (not incremental on disk).
                Best for large sites (Laravel, WordPress, etc.) with thousands of small files — much faster
                than SFTP because rsync batches the transfer instead of opening each file individually.
              </p>
            ) : (
              <div className="wizard-section">
                <h3 className="wizard-section-title">Backup mode</h3>
                <p className="muted small">
                  For PHP/Laravel sites with many small files, consider{" "}
                  <strong>RSYNC over SSH</strong> on the connection step — it is usually much faster than
                  SFTP.
                </p>
                <label className="check">
                  <input
                    type="checkbox"
                    checked={form.incrementalEnabled}
                    onChange={(e) => set("incrementalEnabled", e.target.checked)}
                  />
                  Incremental backups — only copy files changed since the last successful backup
                </label>
                <p className="muted small">
                  Turn off to run a full backup every time (slower, uses more disk).
                </p>
              </div>
            )}
            <div className="wizard-section">
              <h3 className="wizard-section-title">Schedule & retention</h3>
              <ScheduleRetention
                schedule={schedule}
                onSchedule={setSchedule}
                timeZone={timezone}
                retention={{
                  retainHourly: form.retainHourly,
                  retainDaily: form.retainDaily,
                  retainWeekly: form.retainWeekly,
                  retainMonthly: form.retainMonthly,
                  retainYearly: form.retainYearly,
                }}
                onRetention={(k, v) => set(k, v)}
              />
            </div>
          </>
        )}

        {step === 4 && (
          <div className="review">
            <dl>
              <dt>Name</dt>
              <dd>{form.name}</dd>
              <dt>Server</dt>
              <dd>
                {form.protocol.toUpperCase()} · {form.username}@{form.host}:{form.port}
              </dd>
              <dt>Paths</dt>
              <dd>
                <ul className="plain">
                  {(resolvedPaths.includePaths.length
                    ? resolvedPaths.includePaths
                    : [resolvedPaths.remoteRoot]
                  ).map((p) => (
                    <li key={p}>
                      <code>{p}</code>
                    </li>
                  ))}
                </ul>
              </dd>
              {form.excludePaths.length > 0 && (
                <>
                  <dt>Exclude</dt>
                  <dd>
                    <ul className="plain">
                      {form.excludePaths.map((p) => (
                        <li key={p}>
                          <code>{p}</code>
                        </li>
                      ))}
                    </ul>
                  </dd>
                </>
              )}
              <dt>Schedule</dt>
              <dd>
                <code>{buildCron(schedule)}</code>
              </dd>
              <dt>Retention</dt>
              <dd>
                {retentionSummary(form, schedule)}
              </dd>
              {form.protocol !== "rsync" && (
                <>
                  <dt>Incremental</dt>
                  <dd>{form.incrementalEnabled ? "Enabled" : "Disabled (full backup each run)"}</dd>
                </>
              )}
            </dl>
            <label className="check">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => set("enabled", e.target.checked)}
              />
              Enabled
            </label>
          </div>
        )}

        <div className="wizard-nav">
          <Link className="ghost btn-link" to="/app/websites">
            Cancel
          </Link>
          <div className="actions">
            {step > 0 && (
              <button type="button" className="ghost" onClick={() => setStep((s) => s - 1)}>
                Back
              </button>
            )}
            {step < STEPS.length - 1 ? (
              <button type="button" disabled={busy || !canNext()} onClick={() => void goNext()}>
                Continue
              </button>
            ) : (
              <button type="button" disabled={busy} onClick={(e) => void save(e as unknown as FormEvent)}>
                {editing ? "Save changes" : "Create website"}
              </button>
            )}
          </div>
        </div>
      </section>
      <SiteFooter />
    </div>
  );
}
