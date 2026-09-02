/**
 * Timezone-aware helpers for the reminder feature.
 *
 * The backend stores `reminder_time` as an absolute UTC instant and returns it
 * as an ISO string ("...Z"). The user picks/reads reminders as wall-clock time
 * in *their* configured timezone (User.timezone), not the browser's — so we
 * convert here with Intl instead of the native Date local-time methods.
 */

/**
 * Format a UTC ISO timestamp as "YYYY-MM-DDTHH:mm" wall-clock in `timeZone`,
 * suitable for the value of an <input type="datetime-local">.
 */
export function isoToLocalInput(iso: string, timeZone: string): string {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).formatToParts(new Date(iso));

  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
  const hour = get('hour') === '24' ? '00' : get('hour'); // some engines emit "24" at midnight
  return `${get('year')}-${get('month')}-${get('day')}T${hour}:${get('minute')}`;
}

/** Format a UTC ISO timestamp as localized "HH:mm" in `timeZone`. */
export function isoToLocalTime(iso: string, timeZone: string): string {
  return new Intl.DateTimeFormat('ru-RU', {
    timeZone,
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(iso));
}

/**
 * Format a Date as a "YYYY-MM-DD" string in the browser's *local* calendar,
 * unlike `Date#toISOString().slice(0,10)` which is always UTC and can land on
 * the wrong day for users east/west of UTC.
 */
export function toISODate(d: Date): string {
  const year = d.getFullYear();
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

/** Today's local calendar date as "YYYY-MM-DD". */
export function todayISODate(): string {
  return toISODate(new Date());
}

/** Parse a "YYYY-MM-DD" string into a local Date at midnight (no UTC shift). */
export function fromISODate(s: string): Date {
  const [year, month, day] = s.split('-').map(Number);
  return new Date(year, month - 1, day);
}

/** The browser's best guess at the user's IANA timezone. */
export function detectTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

/** Timezones offered in Settings (superset of the common RU/CIS zones). */
export const TIMEZONE_OPTIONS = [
  'UTC',
  'Europe/Kaliningrad',
  'Europe/Moscow',
  'Europe/Samara',
  'Asia/Yekaterinburg',
  'Asia/Omsk',
  'Asia/Novosibirsk',
  'Asia/Krasnoyarsk',
  'Asia/Irkutsk',
  'Asia/Yakutsk',
  'Asia/Vladivostok',
  'Asia/Magadan',
  'Asia/Kamchatka',
  'Europe/Kyiv',
  'Europe/Minsk',
  'Asia/Almaty',
  'Asia/Tashkent',
  'Asia/Tbilisi',
  'Asia/Yerevan',
  'Asia/Baku',
];
