import { useCallback, useEffect, useState } from "react";
import { Navigate, Outlet, Route, Routes } from "react-router-dom";
import Dashboard from "./pages/Dashboard";
import Databases from "./pages/Databases";
import DatabaseBackups from "./pages/DatabaseBackups";
import DatabaseWizard from "./pages/DatabaseWizard";
import ExploreBackups from "./pages/ExploreBackups";
import FileServers from "./pages/FileServers";
import FileServerWizard from "./pages/FileServerWizard";
import Gate from "./pages/Gate";
import SettingsPage from "./pages/Settings";
import ErrorBoundary from "./components/ErrorBoundary";
import { TimezoneProvider } from "./context/Timezone";

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

function LoadingScreen() {
  return (
    <div className="shell center">
      <p className="muted">Loading Boomerang…</p>
    </div>
  );
}

export default function App() {
  const [ready, setReady] = useState(false);
  const [setupRequired, setSetupRequired] = useState(true);
  const [authed, setAuthed] = useState(false);

  const refresh = useCallback(async () => {
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
  }, []);

  useEffect(() => {
    refresh().catch((e) => {
      console.error(e);
      setReady(true);
    });
  }, [refresh]);

  const handleLogout = useCallback(async () => {
    await api("/api/logout", { method: "POST" });
    await refresh();
  }, [refresh]);

  if (!ready) {
    return <LoadingScreen />;
  }

  return (
    <ErrorBoundary>
      <TimezoneProvider>
        <Routes>
          <Route
            path="/"
            element={
              setupRequired || !authed ? (
                <Gate setupRequired={setupRequired} onDone={refresh} />
              ) : (
                <Navigate to="/app" replace />
              )
            }
          />
          <Route
            path="/app"
            element={authed ? <Outlet /> : <Navigate to="/" replace />}
          >
            <Route index element={<Dashboard onLogout={handleLogout} />} />
            <Route path="websites" element={<FileServers />} />
            <Route path="websites/new" element={<FileServerWizard />} />
            <Route path="websites/:id/edit" element={<FileServerWizard />} />
            <Route path="websites/:id/backups" element={<ExploreBackups />} />
            <Route path="file-servers" element={<FileServers />} />
            <Route path="file-servers/new" element={<FileServerWizard />} />
            <Route path="file-servers/:id/edit" element={<FileServerWizard />} />
            <Route path="file-servers/:id/backups" element={<ExploreBackups />} />
            <Route path="databases" element={<Databases />} />
            <Route path="databases/new" element={<DatabaseWizard />} />
            <Route path="databases/:id/backups" element={<DatabaseBackups />} />
            <Route path="databases/:id/edit" element={<DatabaseWizard />} />
            <Route path="settings" element={<SettingsPage />} />
            <Route path="*" element={<Navigate to="/app" replace />} />
          </Route>
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </TimezoneProvider>
    </ErrorBoundary>
  );
}
