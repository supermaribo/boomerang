import { useEffect, useState } from "react";
import { api } from "../App";

type LogResponse = {
  lines: string[];
  skipped?: string[];
};

type Props = {
  url: string;
  title?: string;
  onClose?: () => void;
};

export default function VersionLogPanel({ url, title = "Backup log", onClose }: Props) {
  const [lines, setLines] = useState<string[]>([]);
  const [skipped, setSkipped] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setError("");
    api<LogResponse>(url)
      .then((d) => {
        setLines(d.lines || []);
        setSkipped(d.skipped || []);
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load log"))
      .finally(() => setLoading(false));
  }, [url]);

  return (
    <div className="version-log-panel">
      <div className="version-log-head">
        <h3>{title}</h3>
        {onClose && (
          <button type="button" className="ghost" onClick={onClose}>
            Close
          </button>
        )}
      </div>
      {loading && <p className="muted small">Loading log…</p>}
      {error && <p className="err">{error}</p>}
      {!loading && !error && lines.length === 0 && (
        <p className="muted small">No log saved for this backup (older backups may not have one).</p>
      )}
      {lines.length > 0 && (
        <pre className="version-log-body">{lines.join("\n")}</pre>
      )}
      {skipped.length > 0 && (
        <>
          <h4 className="version-log-subhead">Skipped paths ({skipped.length})</h4>
          <pre className="version-log-body version-log-skipped">{skipped.join("\n")}</pre>
        </>
      )}
    </div>
  );
}
