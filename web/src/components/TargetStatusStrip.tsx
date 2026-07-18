import { Link } from "react-router-dom";
import TargetHealthBadge, { type TargetHealthRow } from "./TargetHealthBadge";
import { formatApplianceDateTime } from "../lib/formatTime";

function fmtBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

type Props = {
  row: TargetHealthRow | null;
  timezone: string;
};

export default function TargetStatusStrip({ row, timezone }: Props) {
  if (!row) return null;
  return (
    <div className="target-status-strip">
      <TargetHealthBadge health={row.health} detail={row.healthDetail} />
      <span className="muted small">
        Last check:{" "}
        {row.lastCheckAt ? formatApplianceDateTime(row.lastCheckAt, timezone) : "—"}
        {row.lastWasSkip ? " (no change)" : ""}
      </span>
      <span className="muted small">
        Last backup:{" "}
        {row.lastSuccessAt ? formatApplianceDateTime(row.lastSuccessAt, timezone) : "—"}
      </span>
      <span className="muted small">
        Next run: {row.nextRunAt ? formatApplianceDateTime(row.nextRunAt, timezone) : "—"}
      </span>
      <span className="muted small">Storage: {fmtBytes(row.storageBytes ?? 0)}</span>
      {row.monitoredServerId && row.monitoredServerName ? (
        <Link className="pill ok btn-link" to={`/app/monitoring/${row.monitoredServerId}`}>
          Monitor: {row.monitoredServerName}
        </Link>
      ) : null}
    </div>
  );
}
