import {
  ScheduleState,
  buildCron,
  describeSchedule,
  scheduleStartISO,
} from "../lib/schedule";

type Retention = {
  retainHourly: number;
  retainDaily: number;
  retainWeekly: number;
  retainYearly: number;
};

type Props = {
  schedule: ScheduleState;
  onSchedule: (s: ScheduleState) => void;
  retention: Retention;
  onRetention: (k: keyof Retention, v: number) => void;
};

export default function ScheduleRetention({
  schedule,
  onSchedule,
  retention,
  onRetention,
}: Props) {
  const setSched = (patch: Partial<ScheduleState>) => {
    const next = { ...schedule, ...patch };
    if (patch.frequency && patch.frequency !== "custom") {
      next.customCron = buildCron(next);
    }
    onSchedule(next);
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

      <div className="row2">
        <div>
          <label>Start time (UTC)</label>
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
              · first eligible <code>{scheduleStartISO(schedule)}</code>
            </>
          ) : null}
        </p>
      )}

      <h3 className="retain-head">Version retention</h3>
      <p className="muted small">
        Keep the newest backup in each period. Older versions outside these windows are removed
        after each successful backup.
      </p>
      <div className="row4">
        <div>
          <label>Hourly</label>
          <input
            type="number"
            min={0}
            value={retention.retainHourly}
            onChange={(e) => onRetention("retainHourly", Number(e.target.value))}
          />
        </div>
        <div>
          <label>Daily</label>
          <input
            type="number"
            min={0}
            value={retention.retainDaily}
            onChange={(e) => onRetention("retainDaily", Number(e.target.value))}
          />
        </div>
        <div>
          <label>Weekly</label>
          <input
            type="number"
            min={0}
            value={retention.retainWeekly}
            onChange={(e) => onRetention("retainWeekly", Number(e.target.value))}
          />
        </div>
        <div>
          <label>Yearly</label>
          <input
            type="number"
            min={0}
            value={retention.retainYearly}
            onChange={(e) => onRetention("retainYearly", Number(e.target.value))}
          />
        </div>
      </div>
    </div>
  );
}

export function retentionSummary(r: {
  retainHourly?: number;
  retainDaily?: number;
  retainWeekly?: number;
  retainYearly?: number;
}) {
  return `keep ${r.retainHourly ?? 0}h / ${r.retainDaily ?? 0}d / ${r.retainWeekly ?? 0}w / ${r.retainYearly ?? 0}y`;
}
