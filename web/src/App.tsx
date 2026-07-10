import { useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import Dashboard from "./pages/Dashboard";
import Databases from "./pages/Databases";
import DatabaseWizard from "./pages/DatabaseWizard";
import ExploreBackups from "./pages/ExploreBackups";
import FileServers from "./pages/FileServers";
import FileServerWizard from "./pages/FileServerWizard";
import Gate from "./pages/Gate";
import SettingsPage from "./pages/Settings";

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
    ...init,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error((data as { error?: string }).error || res.statusText);
  }
  return data as T;
}

export { api };

export default function App() {
  const [ready, setReady] = useState(false);
  const [setupRequired, setSetupRequired] = useState(true);
  const [authed, setAuthed] = useState(false);

  const refresh = async () => {
    const st = await api<{ setupRequired: boolean }>("/api/status");
    setSetupRequired(st.setupRequired);
    if (!st.setupRequired) {
      try {
        await api("/api/me");
        setAuthed(true);
      } catch {
        setAuthed(false);
      }
    } else {
      setAuthed(false);
    }
    setReady(true);
  };

  useEffect(() => {
    refresh().catch((e) => {
      console.error(e);
      setReady(true);
    });
  }, []);

  if (!ready) {
    return (
      <div className="shell center">
        <p className="muted">Loading Boomerang…</p>
      </div>
    );
  }

  return (
    <Routes>
      <Route
        path="/"
        element={
          setupRequired || !authed ? (
            <Gate setupRequired={setupRequired} onDone={async () => { await refresh(); }} />
          ) : (
            <Navigate to="/app" replace />
          )
        }
      />
      {authed ? (
        <>
          <Route
            path="/app"
            element={
              <Dashboard
                onLogout={async () => {
                  await api("/api/logout", { method: "POST" });
                  await refresh();
                }}
              />
            }
          />
          <Route path="/app/file-servers" element={<FileServers />} />
          <Route path="/app/file-servers/new" element={<FileServerWizard />} />
          <Route path="/app/file-servers/:id/edit" element={<FileServerWizard />} />
          <Route path="/app/file-servers/:id/backups" element={<ExploreBackups />} />
          <Route path="/app/databases" element={<Databases />} />
          <Route path="/app/databases/new" element={<DatabaseWizard />} />
          <Route path="/app/databases/:id/edit" element={<DatabaseWizard />} />
          <Route path="/app/settings" element={<SettingsPage />} />
        </>
      ) : (
        <Route path="/app/*" element={<Navigate to="/" replace />} />
      )}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
