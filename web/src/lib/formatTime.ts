/** Parse ISO timestamp from API (UTC or with offset). */
export function parseApiTime(iso: string): Date | null {
  if (!iso) return null;
  const s = iso.includes("T") ? iso : `${iso}Z`;
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? null : d;
}

export function formatApplianceDate(iso: string, timeZone: string): string {
  const d = parseApiTime(iso);
  if (!d) return iso;
  return d.toLocaleDateString(undefined, { timeZone });
}

export function formatApplianceTime(iso: string, timeZone: string): string {
  const d = parseApiTime(iso);
  if (!d) return iso;
  return d.toLocaleTimeString(undefined, { timeZone, hour: "2-digit", minute: "2-digit" });
}

export function formatApplianceDateTime(iso: string, timeZone: string): string {
  const d = parseApiTime(iso);
  if (!d) return iso;
  return d.toLocaleString(undefined, {
    timeZone,
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function guessBrowserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
  } catch {
    return "UTC";
  }
}

export function timezoneLabel(name: string): string {
  return name.replace(/_/g, " ");
}
