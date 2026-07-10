export type ScheduleFrequency =
  | "1h"
  | "2h"
  | "4h"
  | "6h"
  | "12h"
  | "daily"
  | "weekly"
  | "custom";

export type ScheduleState = {
  frequency: ScheduleFrequency;
  startTime: string; // HH:MM
  startDate: string; // YYYY-MM-DD
  weekday: number; // 0=Sun … 6=Sat
  customCron: string;
};

export const defaultSchedule = (): ScheduleState => ({
  frequency: "daily",
  startTime: "02:00",
  startDate: new Date().toISOString().slice(0, 10),
  weekday: 0,
  customCron: "0 2 * * *",
});

const NIGHT_HOURS = [23, 0, 1, 2, 3, 4, 5, 6] as const;

/** Random daily schedule between 23:00 and 06:00 UTC to stagger backup load. */
export function randomNightSchedule(): ScheduleState {
  const hour = NIGHT_HOURS[Math.floor(Math.random() * NIGHT_HOURS.length)];
  const minute = Math.floor(Math.random() * 60);
  const startTime = `${pad(hour)}:${pad(minute)}`;
  const startDate = new Date().toISOString().slice(0, 10);
  return {
    frequency: "daily",
    startTime,
    startDate,
    weekday: 0,
    customCron: `${minute} ${hour} * * *`,
  };
}

export function buildCron(s: ScheduleState): string {
  if (s.frequency === "custom") {
    return s.customCron.trim() || "0 2 * * *";
  }
  const [hh, mm] = parseTime(s.startTime);
  if (s.frequency === "daily") {
    return `${mm} ${hh} * * *`;
  }
  if (s.frequency === "weekly") {
    return `${mm} ${hh} * * ${s.weekday}`;
  }
  const every = Number(s.frequency.replace("h", ""));
  const hours: number[] = [];
  for (let i = 0; i < 24; i += every) {
    hours.push((hh + i) % 24);
  }
  hours.sort((a, b) => a - b);
  return `${mm} ${hours.join(",")} * * *`;
}

export function scheduleStartISO(s: ScheduleState): string {
  if (!s.startDate) return "";
  const [hh, mm] = parseTime(s.startTime);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${s.startDate}T${pad(hh)}:${pad(mm)}:00Z`;
}

export function parseSchedule(cron: string, startISO: string): ScheduleState {
  const base = defaultSchedule();
  if (startISO) {
    const d = new Date(startISO);
    if (!Number.isNaN(d.getTime())) {
      base.startDate = d.toISOString().slice(0, 10);
      base.startTime = `${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}`;
    } else if (/^\d{4}-\d{2}-\d{2}/.test(startISO)) {
      base.startDate = startISO.slice(0, 10);
      const m = startISO.match(/T(\d{2}):(\d{2})/);
      if (m) base.startTime = `${m[1]}:${m[2]}`;
    }
  }

  const c = (cron || "").trim();
  base.customCron = c || base.customCron;
  const parts = c.split(/\s+/);
  if (parts.length !== 5) {
    base.frequency = "custom";
    return base;
  }
  const [min, hour, , , dow] = parts;
  if (!/^\d+$/.test(min)) {
    base.frequency = "custom";
    return base;
  }
  base.startTime = `${hour.includes(",") ? hour.split(",")[0] : hour.padStart(2, "0")}:${min.padStart(2, "0")}`;
  if (/^\d+$/.test(hour) && dow === "*") {
    base.frequency = "daily";
    base.startTime = `${pad(Number(hour))}:${pad(Number(min))}`;
    return base;
  }
  if (/^\d+$/.test(hour) && /^\d+$/.test(dow)) {
    base.frequency = "weekly";
    base.weekday = Number(dow);
    base.startTime = `${pad(Number(hour))}:${pad(Number(min))}`;
    return base;
  }
  if (hour.includes(",")) {
    const hours = hour.split(",").map(Number).filter((n) => !Number.isNaN(n)).sort((a, b) => a - b);
    if (hours.length >= 2) {
      const step = hours[1] - hours[0];
      const ok = [1, 2, 4, 6, 12].includes(step) && hours.length === 24 / step;
      if (ok) {
        base.frequency = `${step}h` as ScheduleFrequency;
        base.startTime = `${pad(hours[0])}:${pad(Number(min))}`;
        return base;
      }
    }
  }
  base.frequency = "custom";
  return base;
}

export function describeSchedule(s: ScheduleState): string {
  const time = s.startTime || "02:00";
  const days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
  switch (s.frequency) {
    case "1h":
      return `Every hour at :${time.slice(3)} from ${time}`;
    case "2h":
    case "4h":
    case "6h":
    case "12h":
      return `Every ${s.frequency.replace("h", "")} hours starting ${time}`;
    case "daily":
      return `Daily at ${time}`;
    case "weekly":
      return `Weekly on ${days[s.weekday] || "Sun"} at ${time}`;
    default:
      return `Cron ${s.customCron}`;
  }
}

function parseTime(t: string): [number, number] {
  const [h, m] = (t || "02:00").split(":").map((x) => Number(x));
  return [Number.isFinite(h) ? h % 24 : 2, Number.isFinite(m) ? m % 60 : 0];
}

function pad(n: number) {
  return String(n).padStart(2, "0");
}

export type RetentionTier = "hourly" | "daily" | "weekly" | "monthly" | "yearly";

/** Retention periods that can apply given how often backups run. */
export function retentionTiersForFrequency(freq: ScheduleFrequency): RetentionTier[] {
  if (freq === "weekly") {
    return ["weekly", "monthly", "yearly"];
  }
  if (freq === "daily") {
    return ["daily", "weekly", "monthly", "yearly"];
  }
  return ["hourly", "daily", "weekly", "monthly", "yearly"];
}
