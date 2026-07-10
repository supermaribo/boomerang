import { useEffect, useState } from "react";
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
import type { Database } from "./Databases";
import type { FileServer } from "./FileServers";

const STEPS = ["Database", "Connection", "Tables", "Schedule", "Review"] as const;

const emptyForm = {
  name: "",
  mysqlHost: "127.0.0.1",
  mysqlPort: 3306,
  mysqlDb: "",
  mysqlUser: "",
  mysqlPassword: "",
  includeTables: [] as string[],
  tunnelMode: "none",
  fileServerId: "",
  sshHost: "",
  sshPort: 22,
  sshUsername: "",
  authMode: "password",
  password: "",
  privateKey: "",
  passphrase: "",
  retainHourly: 1,
  retainDaily: 1,
  retainWeekly: 1,
  retainMonthly: 1,
  retainYearly: 1,
  enabled: true,
};

export default function DatabaseWizard() {
  const { timezone } = useTimezone();
  const { id } = useParams();
  const editing = Boolean(id);
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const [form, setForm] = useState(emptyForm);
  const [schedule, setSchedule] = useState<ScheduleState>(() => randomNightSchedule(timezone));
  const [servers, setServers] = useState<FileServer[]>([]);
  const [allTables, setAllTables] = useState<string[]>([]);
  const [selectedTables, setSelectedTables] = useState<string[]>([]);
  const [tablesLoaded, setTablesLoaded] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [loaded, setLoaded] = useState(!editing);

  const linkedServer = servers.find((s) => s.id === form.fileServerId);
  const firewallTarget =
    form.tunnelMode === "none"
      ? { host: form.mysqlHost, port: form.mysqlPort, protocol: "MySQL" }
      : form.tunnelMode === "inline"
        ? { host: form.sshHost, port: form.sshPort, protocol: "SSH" }
        : {
            host: linkedServer?.host || "",
            port: linkedServer?.port || 22,
            protocol: "SSH",
          };

  const set = (k: string, v: string | number | boolean | string[]) =>
    setForm((f) => ({ ...f, [k]: v }));

  useEffect(() => {
    api<FileServer[]>("/api/file-servers").then(setServers).catch(() => undefined);
  }, []);

  useEffect(() => {
    if (!id) return;
    api<Database>(`/api/databases/${id}`)
      .then((d) => {
        setForm({
          ...emptyForm,
          name: d.name,
          mysqlHost: d.mysqlHost,
          mysqlPort: d.mysqlPort,
          mysqlDb: d.mysqlDb,
          mysqlUser: d.mysqlUser,
          includeTables: d.includeTables || [],
          tunnelMode: d.tunnelMode,
          fileServerId: d.fileServerId || "",
          sshHost: d.sshHost,
          sshPort: d.sshPort,
          sshUsername: d.sshUsername,
          authMode: d.authMode,
          retainHourly: d.retainHourly ?? 1,
          retainDaily: d.retainDaily ?? 1,
          retainWeekly: d.retainWeekly ?? 1,
          retainMonthly: d.retainMonthly ?? 1,
          retainYearly: d.retainYearly ?? 1,
          enabled: d.enabled,
        });
        if (d.includeTables?.length) {
          setSelectedTables(d.includeTables);
        }
        setSchedule(parseSchedule(d.scheduleCron, d.scheduleStart || "", timezone));
        setLoaded(true);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "load failed"));
  }, [id]);

  const browseBody = () => ({
    ...form,
    fileServerId: form.fileServerId || null,
    scheduleCron: buildCron(schedule),
    scheduleStart: scheduleStartISO(schedule, timezone),
    retainCount: 0,
    retainDays: 0,
  });

  const loadTables = async () => {
    setBusy(true);
    setError("");
    try {
      const url = editing
        ? `/api/databases/${id}/browse-tables`
        : "/api/databases/browse-tables";
      const data = await api<{ tables: string[] }>(url, {
        method: "POST",
        body: JSON.stringify(browseBody()),
      });
      const tables = data.tables || [];
      setAllTables(tables);
      setTablesLoaded(true);
      if (selectedTables.length === 0) {
        if (form.includeTables.length > 0) {
          setSelectedTables(form.includeTables.filter((t) => tables.includes(t)));
        } else {
          setSelectedTables(tables);
        }
      } else {
        setSelectedTables((prev) => prev.filter((t) => tables.includes(t)));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "could not list tables");
      setAllTables([]);
      setTablesLoaded(false);
    } finally {
      setBusy(false);
    }
  };

  const toggleTable = (table: string) => {
    setSelectedTables((prev) =>
      prev.includes(table) ? prev.filter((t) => t !== table) : [...prev, table],
    );
  };

  const canNext = () => {
    if (step === 0) {
      return Boolean(form.name && form.mysqlDb && form.mysqlUser && (editing || form.mysqlPassword));
    }
    if (step === 1) {
      if (form.tunnelMode === "fileserver") return Boolean(form.fileServerId);
      if (form.tunnelMode === "inline") return Boolean(form.sshHost && form.sshUsername);
      return true;
    }
    if (step === 2) return selectedTables.length > 0;
    return true;
  };

  const goNext = async () => {
    setError("");
    if (step < STEPS.length - 1) {
      const next = step + 1;
      if (next === 2) {
        await loadTables();
      }
      setStep(next);
    }
  };

  const save = async () => {
    if (selectedTables.length === 0) {
      setError("Select at least one table to back up");
      setStep(2);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const body = {
        ...browseBody(),
        includeTables: selectedTables,
      };
      if (editing) {
        await api(`/api/databases/${id}`, { method: "PUT", body: JSON.stringify(body) });
      } else {
        await api("/api/databases", { method: "POST", body: JSON.stringify(body) });
      }
      navigate("/app/databases");
    } catch (err) {
      setError(err instanceof Error ? err.message : "save failed");
    } finally {
      setBusy(false);
    }
  };

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
        <h1>{editing ? "Edit database" : "Add database"}</h1>
        <p className="muted">
          Step {step + 1} of {STEPS.length}: {STEPS[step]}
        </p>
      </header>

      <ol className="wizard-steps">
        {STEPS.map((label, i) => (
          <li key={label} className={i === step ? "active" : i < step ? "done" : ""}>
            <button type="button" onClick={() => i < step && setStep(i)} disabled={i > step}>
              {label}
            </button>
          </li>
        ))}
      </ol>

      <section className="tile wizard-tile">
        <div className="err">{error}</div>

        {step === 0 && (
          <>
            <label>Name</label>
            <input value={form.name} onChange={(e) => set("name", e.target.value)} required />
            <div className="row2">
              <div>
                <label>MySQL host</label>
                <input value={form.mysqlHost} onChange={(e) => set("mysqlHost", e.target.value)} />
              </div>
              <div>
                <label>Port</label>
                <input
                  type="number"
                  value={form.mysqlPort}
                  onChange={(e) => set("mysqlPort", Number(e.target.value))}
                />
              </div>
            </div>
            <label>Database name</label>
            <input value={form.mysqlDb} onChange={(e) => set("mysqlDb", e.target.value)} required />
            <label>MySQL user</label>
            <input value={form.mysqlUser} onChange={(e) => set("mysqlUser", e.target.value)} required />
            <label>MySQL password {editing ? "(leave blank to keep)" : ""}</label>
            <input
              type="password"
              value={form.mysqlPassword}
              onChange={(e) => set("mysqlPassword", e.target.value)}
              autoComplete="new-password"
            />
          </>
        )}

        {step === 1 && (
          <>
            <label>SSH tunnel</label>
            <select value={form.tunnelMode} onChange={(e) => set("tunnelMode", e.target.value)}>
              <option value="none">None (direct TCP)</option>
              <option value="fileserver">Via existing file server SSH</option>
              <option value="inline">Inline SSH credentials</option>
            </select>
            {form.tunnelMode === "fileserver" && (
              <>
                <label>File server</label>
                <select
                  value={form.fileServerId}
                  onChange={(e) => set("fileServerId", e.target.value)}
                  required
                >
                  <option value="">Select…</option>
                  {servers.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              </>
            )}
            {form.tunnelMode === "inline" && (
              <>
                <div className="row2">
                  <div>
                    <label>SSH host</label>
                    <input value={form.sshHost} onChange={(e) => set("sshHost", e.target.value)} />
                  </div>
                  <div>
                    <label>SSH port</label>
                    <input
                      type="number"
                      value={form.sshPort}
                      onChange={(e) => set("sshPort", Number(e.target.value))}
                    />
                  </div>
                </div>
                <label>SSH username</label>
                <input value={form.sshUsername} onChange={(e) => set("sshUsername", e.target.value)} />
                <label>SSH auth</label>
                <select value={form.authMode} onChange={(e) => set("authMode", e.target.value)}>
                  <option value="password">Password</option>
                  <option value="key">Private key</option>
                </select>
                {form.authMode === "password" ? (
                  <>
                    <label>SSH password</label>
                    <input
                      type="password"
                      value={form.password}
                      onChange={(e) => set("password", e.target.value)}
                    />
                  </>
                ) : (
                  <>
                    <label>Private key</label>
                    <textarea
                      rows={4}
                      value={form.privateKey}
                      onChange={(e) => set("privateKey", e.target.value)}
                    />
                  </>
                )}
              </>
            )}
            <FirewallReminder
              targetHost={firewallTarget.host}
              port={firewallTarget.port}
              protocol={firewallTarget.protocol}
            />
            {form.tunnelMode !== "none" && (
              <p className="muted small">
                MySQL is reached through the SSH tunnel on the remote host (usually{" "}
                <code>127.0.0.1:3306</code>) — you only need to open SSH from Boomerang, not MySQL
                to the internet.
              </p>
            )}
          </>
        )}

        {step === 2 && (
          <div className="path-picker">
            <p className="muted small">
              Connect to the database and tick the tables to include in each backup (e.g. skip
              sessions or cache tables on WordPress).
            </p>
            <div className="path-toolbar">
              <button type="button" className="secondary" disabled={busy} onClick={() => void loadTables()}>
                {tablesLoaded ? "Refresh tables" : "Load tables"}
              </button>
              <button
                type="button"
                className="secondary"
                disabled={busy || allTables.length === 0}
                onClick={() => setSelectedTables(allTables)}
              >
                Select all
              </button>
              <button
                type="button"
                className="secondary"
                disabled={busy || selectedTables.length === 0}
                onClick={() => setSelectedTables([])}
              >
                Clear
              </button>
            </div>
            <div className="path-list">
              {allTables.map((t) => (
                <label key={t} className="path-row">
                  <input
                    type="checkbox"
                    checked={selectedTables.includes(t)}
                    onChange={() => toggleTable(t)}
                  />
                  <span className="path-name">
                    <code>{t}</code>
                  </span>
                </label>
              ))}
              {tablesLoaded && allTables.length === 0 && (
                <p className="muted">No tables found in this database.</p>
              )}
              {!tablesLoaded && <p className="muted">Load tables to choose what to back up.</p>}
            </div>
            <div className="selected-paths">
              <h3>Selected ({selectedTables.length})</h3>
              {selectedTables.length === 0 && <p className="muted small">Nothing selected yet</p>}
              <ul>
                {selectedTables.map((t) => (
                  <li key={t}>
                    <code>{t}</code>
                    <button type="button" className="ghost" onClick={() => toggleTable(t)}>
                      Remove
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        )}

        {step === 3 && (
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
        )}

        {step === 4 && (
          <div className="review">
            <dl>
              <dt>Name</dt>
              <dd>{form.name}</dd>
              <dt>MySQL</dt>
              <dd>
                {form.mysqlUser}@{form.mysqlHost}:{form.mysqlPort}/{form.mysqlDb}
              </dd>
              <dt>Tables</dt>
              <dd>
                <ul className="plain">
                  {selectedTables.map((t) => (
                    <li key={t}>
                      <code>{t}</code>
                    </li>
                  ))}
                </ul>
              </dd>
              <dt>Tunnel</dt>
              <dd>{form.tunnelMode}</dd>
              <dt>Schedule</dt>
              <dd>
                <code>{buildCron(schedule)}</code>
              </dd>
              <dt>Retention</dt>
              <dd>
                {retentionSummary(form, schedule)}
              </dd>
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
          <Link className="ghost btn-link" to="/app/databases">
            Cancel
          </Link>
          <div className="actions">
            {step > 0 && (
              <button type="button" className="ghost" onClick={() => setStep((s) => s - 1)}>
                Back
              </button>
            )}
            {step < STEPS.length - 1 ? (
              <button
                type="button"
                disabled={!canNext() || busy}
                onClick={() => void goNext()}
              >
                Continue
              </button>
            ) : (
              <button type="button" disabled={busy} onClick={() => void save()}>
                {editing ? "Save changes" : "Create database"}
              </button>
            )}
          </div>
        </div>
      </section>
      <SiteFooter />
    </div>
  );
}
