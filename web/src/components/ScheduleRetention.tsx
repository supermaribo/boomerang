import { useEffect } from "react";
import {
  ScheduleState,
  buildCron,
  describeSchedule,
  retentionTiersForFrequency,
  scheduleStartISO,
} from "../lib/schedule";

type Retention = {
  retainHourly: number;
  retainDaily: number;
  retainWeekly: number;
  retainMonthly: number;
  retainYearly: number;
};

type Props = {
  schedule: ScheduleState;
  onSchedule: (s: ScheduleState) => void;
  retention: Retention;
  onRetention: (k: keyof Retention, v: number) => void;
  timeZone?: string;
};

const TIER_LABELS: Record<keyof Retention, string> = {
  retainHourly: "Hourly",
  retainDaily: "Daily",
  retainWeekly: "Weekly",
  retainMonthly: "Monthly",
  retainYearly: "Yearly",
};

const TIER_KEYS: Record<string, keyof Retention> = {
  hourly: "retainHourly",
  daily: "retainDaily",
  weekly: "retainWeekly",
  monthly: "retainMonthly",
  yearly: "retainYearly",
};

export default function ScheduleRetention({
  schedule,
  onSchedule,
  retention,
  onRetention,
  timeZone = "UTC",
}: Props) {
  const tiers = retentionTiersForFrequency(schedule.frequency);

  const clearHiddenRetention = (freq: ScheduleState["frequency"]) => {
    const allowed = new Set(retentionTiersForFrequency(freq));
    for (const [tier, key] of Object.entries(TIER_KEYS)) {
      if (!allowed.has(tier as (typeof tiers)[number])) {
        onRetention(key, 0);
      }
    }
  };

  useEffect(() => {
    clearHiddenRetention(schedule.frequency);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [schedule.frequency]);

  const setSched = (patch: Partial<ScheduleState>) => {
    const next = { ...schedule, ...patch };
    if (patch.frequency && patch.frequency !== "custom") {
      next.customCron = buildCron(next);
    }
    onSchedule(next);
    if (patch.frequency) {
      clearHiddenRetention(patch.frequency);
    }
  };

  const cron = buildCron(schedule);

  return (
    <div className="sched-box">
      <h3>Schedule</h3>
      <p className="muted small sched-desc">{describeSchedule(schedule)}</p>

      <label>Frequency</label>
      <select
        value={schedule.frequency}
        onChange={(e) => setSched({ frequency: e.target.value as ScheduleState["frequency"] })}
      >
        <option value="1h">Every hour</option>
        <option value="2h">Every 2 hours</option>
        <option value="4h">Every 4 hours</option>
        <option value="6h">Every 6 hours</option>
        <option value="12h">Every 12 hours</option>
        <option value="daily">Every day</option>
        <option value="weekly">Every week</option>
        <option value="custom">Custom cron</option>
      </select>
      <p className="muted small">
        New backups pick a random time between 23:00 and 06:00 ({timeZone}) so jobs don&apos;t all
        run at once. You can change it below.
      </p>

      <div className="row2">
        <div>
          <label>Start time ({timeZone})</label>
          <input
            type="time"
            value={schedule.startTime}
            onChange={(e) => setSched({ startTime: e.target.value || "02:00" })}
            disabled={schedule.frequency === "custom"}
          />
        </div>
        <div>
          <label>Start date</label>
          <input
            type="date"
            value={schedule.startDate}
            onChange={(e) => setSched({ startDate: e.target.value })}
          />
        </div>
      </div>

      {schedule.frequency === "weekly" && (
        <>
          <label>Day of week</label>
          <select
            value={schedule.weekday}
            onChange={(e) => setSched({ weekday: Number(e.target.value) })}
          >
            <option value={0}>Sunday</option>
            <option value={1}>Monday</option>
            <option value={2}>Tuesday</option>
            <option value={3}>Wednesday</option>
            <option value={4}>Thursday</option>
            <option value={5}>Friday</option>
            <option value={6}>Saturday</option>
          </select>
        </>
      )}

      {schedule.frequency === "custom" ? (
        <>
          <label>Cron expression</label>
          <input
            value={schedule.customCron}
            onChange={(e) => setSched({ customCron: e.target.value })}
            placeholder="0 2 * * *"
          />
        </>
      ) : (
        <p className="hint">
          Runs as <code>{cron}</code>
          {schedule.startDate ? (
            <>
              {" "}
              · first eligible <code>{scheduleStartISO(schedule, timeZone)}</code>
            </>
          ) : null}
        </p>
      )}

      <h3 className="retain-head">Version retention</h3>
      <p className="muted small">
        Keep the newest backup in each period. Older versions outside these windows are removed
        after each successful backup.
        {schedule.frequency === "weekly" && (
          <> Hourly and daily retention are hidden because backups run weekly.</>
        )}
        {schedule.frequency === "daily" && (
          <> Hourly retention is hidden because backups run once per day.</>
        )}
      </p>
      <div className="retain-grid">
        {tiers.map((tier) => {
          const key = TIER_KEYS[tier];
          return (
            <div key={tier}>
              <label>{TIER_LABELS[key]}</label>
              <input
                type="number"
                min={0}
                value={retention[key]}
                onChange={(e) => onRetention(key, Number(e.target.value))}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function retentionSummary(
  r: {
    retainHourly?: number;
    retainDaily?: number;
    retainWeekly?: number;
    retainMonthly?: number;
    retainYearly?: number;
  },
  schedule?: ScheduleState,
) {
  const tiers = schedule ? retentionTiersForFrequency(schedule.frequency) : ["hourly", "daily", "weekly", "monthly", "yearly"];
  const labels: Record<string, string> = {
    hourly: `${r.retainHourly ?? 0}h`,
    daily: `${r.retainDaily ?? 0}d`,
    weekly: `${r.retainWeekly ?? 0}w`,
    monthly: `${r.retainMonthly ?? 0}m`,
    yearly: `${r.retainYearly ?? 0}y`,
  };
  return `keep ${tiers.map((t) => labels[t]).join(" / ")}`;
}
