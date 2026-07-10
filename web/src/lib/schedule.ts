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
  startTime: string; // HH:MM in appliance timezone
  startDate: string; // YYYY-MM-DD in appliance timezone
  weekday: number; // 0=Sun … 6=Sat
  customCron: string;
};

export const defaultSchedule = (timeZone = "UTC"): ScheduleState => ({
  frequency: "daily",
  startTime: "02:00",
  startDate: todayInZone(timeZone),
  weekday: 0,
  customCron: "0 2 * * *",
});

const NIGHT_HOURS = [23, 0, 1, 2, 3, 4, 5, 6] as const;

/** Random daily schedule between 23:00 and 06:00 in the appliance timezone. */
export function randomNightSchedule(timeZone = "UTC"): ScheduleState {
  const hour = NIGHT_HOURS[Math.floor(Math.random() * NIGHT_HOURS.length)];
  const minute = Math.floor(Math.random() * 60);
  const startTime = `${pad(hour)}:${pad(minute)}`;
  const startDate = todayInZone(timeZone);
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

/** RFC3339 UTC instant for the schedule's first eligible wall-clock time. */
export function scheduleStartISO(s: ScheduleState, timeZone = "UTC"): string {
  if (!s.startDate) return "";
  return instantFromWallClock(s.startDate, s.startTime, timeZone).toISOString();
}

export function parseSchedule(cron: string, startISO: string, timeZone = "UTC"): ScheduleState {
  const base = defaultSchedule(timeZone);
  if (startISO) {
    const d = new Date(startISO);
    if (!Number.isNaN(d.getTime())) {
      const wall = wallClockFromInstant(startISO, timeZone);
      base.startDate = wall.date;
      base.startTime = wall.time;
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

function todayInZone(timeZone: string): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date());
}

function wallClockFromInstant(iso: string, timeZone: string): { date: string; time: string } {
  const d = new Date(iso);
  const parts = new Intl.DateTimeFormat("en-GB", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).formatToParts(d);
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === type)?.value ?? "";
  let hour = get("hour");
  if (hour === "24") hour = "00";
  return {
    date: `${get("year")}-${get("month")}-${get("day")}`,
    time: `${hour.padStart(2, "0")}:${get("minute").padStart(2, "0")}`,
  };
}

function instantFromWallClock(date: string, time: string, timeZone: string): Date {
  const [y, m, d] = date.split("-").map((x) => Number(x));
  const [hh, mm] = parseTime(time);
  let utcMs = Date.UTC(y, m - 1, d, hh, mm, 0);
  for (let i = 0; i < 5; i++) {
    const parts = new Intl.DateTimeFormat("en-GB", {
      timeZone,
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).formatToParts(new Date(utcMs));
    const get = (type: Intl.DateTimeFormatPartTypes) =>
      Number(parts.find((p) => p.type === type)?.value ?? "0");
    let ph = get("hour");
    if (ph === 24) ph = 0;
    const localAsUtc = Date.UTC(get("year"), get("month") - 1, get("day"), ph, get("minute"), 0);
    const target = Date.UTC(y, m - 1, d, hh, mm, 0);
    const delta = target - localAsUtc;
    utcMs += delta;
    if (delta === 0) break;
  }
  return new Date(utcMs);
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
