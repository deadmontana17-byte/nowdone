import { apiRequest } from './client';
import { detectTimezone } from '@/utils/datetime';
import type { User } from '@/types';

export function startLogin() {
  return apiRequest<{ deep_link: string }>('/auth/login/start', { method: 'POST' });
}

export function redeemLogin(code: string) {
  return apiRequest<{ user: User }>('/auth/login/redeem', { method: 'POST', body: { code } });
}

export function logout() {
  return apiRequest<{ ok: boolean }>('/auth/logout', { method: 'POST' });
}

export function fetchMe() {
  return apiRequest<{ user: User }>('/auth/me');
}

/** Update user preferences (currently only the IANA timezone). */
export function updateSettings(input: { timezone: string }) {
  return apiRequest<{ user: User }>('/auth/me', { method: 'PATCH', body: input });
}

/**
 * Adopt the browser's IANA timezone for a user still on the "UTC" default.
 * Run right after login and on app boot so streaks, task dates and reminders
 * roll over at the user's local midnight without them having to open Settings.
 * Returns the updated user on success, or the original one when the sync is
 * unnecessary or fails (non-fatal — Settings still lets them set it by hand).
 */
export async function syncBrowserTimezone(user: User): Promise<User> {
  const browserTz = detectTimezone();
  if (user.timezone !== 'UTC' || browserTz === 'UTC') return user;
  try {
    const { user: synced } = await updateSettings({ timezone: browserTz });
    return synced;
  } catch {
    return user;
  }
}

export function setPin(pin: string) {
  return apiRequest<{ ok: boolean }>('/auth/pin', { method: 'POST', body: { pin } });
}

export function verifyPin(pin: string) {
  return apiRequest<{ ok: boolean }>('/auth/pin/verify', { method: 'POST', body: { pin } });
}

/** Ask the backend to generate a 6-digit reset code and send it to the user's
 * Telegram chat directly. No bot deep link — the caller just opens the code
 * entry modal afterwards. */
export function startPinReset() {
  return apiRequest<{ ok: boolean }>('/auth/pin/reset/start', { method: 'POST' });
}

export function redeemPinReset(code: string) {
  return apiRequest<{ ok: boolean }>('/auth/pin/reset/redeem', { method: 'POST', body: { code } });
}

export function setNewPin(pin: string) {
  return apiRequest<{ ok: boolean }>('/auth/pin/reset/confirm', { method: 'POST', body: { pin } });
}
