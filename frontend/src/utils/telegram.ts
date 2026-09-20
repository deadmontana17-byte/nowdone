/**
 * Opening the Telegram bot deep link from the login screen.
 *
 * The backend (`/auth/login/start`) returns an https link built from
 * BOT_USERNAME: `https://t.me/<bot>?start=auth_<code>`.
 *
 * In a normal browser tab that link is fine — `window.open(_, '_blank')` lands
 * on web Telegram or is handed to the desktop/mobile app by the OS.
 *
 * Inside an installed PWA running in `display-mode: standalone` there is no
 * browser chrome and no real tab to open into: `window.open(_, '_blank')` is
 * commonly a no-op (or shows a blank in-app view), so the https link appears to
 * do nothing. The fix is to hand the OS a `tg://` URL instead, which opens the
 * Telegram app directly; if that scheme doesn't resolve (Telegram not
 * installed) we notice the page never went to the background and fall back to
 * the original https link.
 */

/** Milliseconds to wait for the `tg://` scheme to take over before falling back. */
const APP_LINK_FALLBACK_MS = 1500;

/**
 * True when the app runs as an installed PWA with no browser UI.
 *
 * `display-mode: standalone` is the standard signal; iOS Safari predates that
 * media query and instead exposes the non-standard `navigator.standalone`.
 */
export function isStandalonePWA(): boolean {
  if (typeof window === 'undefined') return false;
  const byDisplayMode = window.matchMedia?.('(display-mode: standalone)').matches ?? false;
  const iosStandalone = (window.navigator as Navigator & { standalone?: boolean }).standalone === true;
  return byDisplayMode || iosStandalone;
}

/**
 * Convert `https://t.me/<domain>?start=<payload>` into the equivalent
 * `tg://resolve?domain=<domain>&start=<payload>`.
 *
 * The bot username is taken from the https link's own path (it originates from
 * BOT_USERNAME on the backend), so the client never has to duplicate that env
 * var. Returns `null` when the input isn't a recognisable t.me link, so callers
 * can fall back to using it as-is.
 */
export function toTelegramAppLink(httpsLink: string): string | null {
  try {
    const url = new URL(httpsLink);
    if (url.hostname !== 't.me' && url.hostname !== 'telegram.me') return null;

    // pathname is "/nowdone_bot" -> "nowdone_bot"
    const domain = url.pathname.replace(/^\/+/, '').split('/')[0];
    if (!domain) return null;

    const params = new URLSearchParams({ domain });
    const start = url.searchParams.get('start'); // e.g. "auth_123456"
    if (start) params.set('start', start);

    return `tg://resolve?${params.toString()}`;
  } catch {
    return null;
  }
}

/**
 * Open the Telegram bot for login, choosing the strategy by runtime:
 *
 *  - Normal browser  -> `window.open(httpsLink, '_blank')` (unchanged).
 *  - Installed PWA    -> navigate to the `tg://` deep link; if the page is still
 *                        visible after a short delay the Telegram app didn't
 *                        take over, so open the https link as a fallback.
 */
export function openTelegramLogin(httpsLink: string): void {
  if (!isStandalonePWA()) {
    window.open(httpsLink, '_blank', 'noopener,noreferrer');
    return;
  }

  const appLink = toTelegramAppLink(httpsLink);
  if (!appLink) {
    // Unrecognised link shape — best effort with the https link.
    window.location.href = httpsLink;
    return;
  }

  let handedOff = false;
  const markHandedOff = () => {
    handedOff = true;
  };
  const onVisibility = () => {
    if (document.visibilityState === 'hidden') markHandedOff();
  };

  // When the OS switches to Telegram the PWA is backgrounded (visibilitychange /
  // pagehide / blur). Any of these means the deep link worked — cancel the
  // https fallback so the user doesn't also get a browser tab on return.
  document.addEventListener('visibilitychange', onVisibility);
  window.addEventListener('pagehide', markHandedOff);
  window.addEventListener('blur', markHandedOff);

  const cleanup = () => {
    document.removeEventListener('visibilitychange', onVisibility);
    window.removeEventListener('pagehide', markHandedOff);
    window.removeEventListener('blur', markHandedOff);
  };

  // Trigger the app scheme. On Android an unresolved scheme fails inline
  // without unloading the PWA; on iOS it silently does nothing.
  window.location.href = appLink;

  window.setTimeout(() => {
    cleanup();
    if (handedOff || document.visibilityState === 'hidden') return;

    // tg:// never resolved: open the https link. `window.open` may be blocked
    // in standalone mode, so fall back once more to a same-context navigation.
    const opened = window.open(httpsLink, '_blank', 'noopener,noreferrer');
    if (!opened) window.location.href = httpsLink;
  }, APP_LINK_FALLBACK_MS);
}
