type Props = {
  health: string;
  detail?: string;
};

const LABELS: Record<string, string> = {
  ok: "Healthy",
  warning: "Needs attention",
  error: "Overdue",
  idle: "Idle",
};

export default function TargetHealthBadge({ health, detail }: Props) {
  const label = LABELS[health] || health;
  return (
    <span className={`health-pill ${health}`} title={detail || label}>
      {label}
    </span>
  );
}

export type TargetHealthRow = {
  id: string;
  targetType: string;
  name: string;
  enabled?: boolean;
  health: string;
  healthDetail?: string;
  lastSuccessAt?: string;
  lastSuccessBytes?: number;
  lastCheckAt?: string;
  lastWasSkip?: boolean;
  versionCount?: number;
  nextRunAt?: string;
  storageBytes?: number;
  monitoredServerId?: string;
  monitoredServerName?: string;
};

export function healthMap(rows: TargetHealthRow[]) {
  const out: Record<string, TargetHealthRow> = {};
  for (const row of rows) {
    out[`${row.targetType}:${row.id}`] = row;
  }
  return out;
}
