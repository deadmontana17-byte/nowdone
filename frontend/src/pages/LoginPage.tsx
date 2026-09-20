import { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Button, TextField, Typography, Stack, Link as MuiLink, CircularProgress } from '@mui/material';
import TelegramIcon from '@mui/icons-material/Telegram';

import { startLogin, redeemLogin, fetchMe, syncBrowserTimezone } from '@/api/auth';
import { ApiError } from '@/api/client';
import { openTelegramLogin } from '@/utils/telegram';
import { useAuthStore } from '@/store/authStore';
import { useUiStore } from '@/store/uiStore';

// The one-time code baked into the deep link is valid for 5 minutes (backend
// AuthCodeTTL). Refresh it comfortably before that so the link wired to the
// button is always live.
const DEEP_LINK_MAX_AGE_MS = 4 * 60 * 1000;

export function LoginPage() {
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [isStarting, setIsStarting] = useState(false);
  const [isRedeeming, setIsRedeeming] = useState(false);
  const [isCheckingSession, setIsCheckingSession] = useState(false);
  // Only reveal the code-entry UI after a real tap — the prefetch below must
  // not make it appear on its own.
  const [loginRequested, setLoginRequested] = useState(false);
  // Wall-clock time the current deepLink was fetched, to detect a stale code.
  const deepLinkFetchedAt = useRef(0);

  // Fetch (or refetch) the bot deep link outside of any click handler.
  const refreshDeepLink = useCallback(async () => {
    const { deep_link } = await startLogin();
    setDeepLink(deep_link);
    deepLinkFetchedAt.current = Date.now();
    return deep_link;
  }, []);

  // Prefetch the deep link as soon as the page loads, and again whenever the
  // user comes back to the app (e.g. returns from Telegram to retry).
  //
  // Why prefetch: in an installed PWA — iOS in particular — navigating to a
  // `tg://` scheme or calling window.open only works while the browser still
  // considers us inside the user gesture that triggered it. If the click
  // handler first `await`s the /auth/login/start round-trip, that activation
  // is gone by the time we try to open Telegram, so the redirect silently
  // does nothing. Having the link ready in advance turns the button tap into
  // a pure synchronous navigation, which the OS honours.
  useEffect(() => {
    void refreshDeepLink().catch(() => {
      // Non-fatal: handleStartLogin fetches on demand as a fallback.
    });
    const onVisible = () => {
      if (document.visibilityState === 'visible' && Date.now() - deepLinkFetchedAt.current > DEEP_LINK_MAX_AGE_MS) {
        void refreshDeepLink().catch(() => {});
      }
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, [refreshDeepLink]);

  async function handleStartLogin() {
    setLoginRequested(true);

    // Fast path: a fresh link is already in hand, so open Telegram
    // synchronously inside this tap. No await in between => the user gesture
    // is still valid and the tg:// redirect works even in a standalone PWA.
    if (deepLink && Date.now() - deepLinkFetchedAt.current <= DEEP_LINK_MAX_AGE_MS) {
      // Normal browser: opens a new tab. Installed PWA: hands the OS a tg://
      // link with an https fallback. See utils/telegram.ts.
      openTelegramLogin(deepLink);
      // Rotate the code in the background for the next attempt.
      void refreshDeepLink().catch(() => {});
      return;
    }

    // Slow path: prefetch failed or the code went stale. Fetch then open —
    // this loses the gesture on iOS PWA, so the on-screen "открыть ещё раз"
    // link is the reliable retry there.
    setIsStarting(true);
    try {
      const link = await refreshDeepLink();
      openTelegramLogin(link);
    } catch (err) {
      showSnackbar(err instanceof ApiError ? err.message : 'Не удалось начать вход', 'error');
    } finally {
      setIsStarting(false);
    }
  }

  async function handleRedeem() {
    setIsRedeeming(true);
    try {
      const { user } = await redeemLogin(code);
      // First login on this device: pick up the browser's timezone right away
      // instead of waiting for the next full app load.
      setUser(await syncBrowserTimezone(user));
      navigate('/pin', { replace: true });
    } catch (err) {
      if (err instanceof ApiError && err.status === 202) {
        showSnackbar('Код ещё не подтверждён — нажмите Start в боте', 'info');
      } else {
        showSnackbar(err instanceof ApiError ? err.message : 'Не удалось войти', 'error');
      }
    } finally {
      setIsRedeeming(false);
    }
  }

  // "Уже входили? Ввести PIN": the JWT lives in an http-only cookie that JS
  // can't read, so probe the session via /auth/me. If it's still valid, restore
  // the user and send them to the PIN gate; otherwise tell them they're not
  // signed in and clear the login form.
  async function handleAlreadyLoggedIn() {
    setIsCheckingSession(true);
    try {
      const { user } = await fetchMe();
      setUser(await syncBrowserTimezone(user));
      navigate('/pin', { replace: true });
    } catch {
      showSnackbar('Вы не авторизованы', 'error');
      setCode('');
      setLoginRequested(false);
    } finally {
      setIsCheckingSession(false);
    }
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', px: 3 }}>
      <Typography variant="h4" sx={{ fontWeight: 700, mb: 1 }}>
        NowDone
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 4, textAlign: 'center' }}>
        Ежедневник с задачами, стриками и Telegram-ботом
      </Typography>

      <Stack spacing={2} sx={{ width: '100%', maxWidth: 360 }}>
        <Button
          variant="contained"
          size="large"
          startIcon={isStarting ? <CircularProgress size={18} color="inherit" /> : <TelegramIcon />}
          onClick={handleStartLogin}
          disabled={isStarting}
        >
          Войти через Telegram
        </Button>

        {loginRequested && deepLink && (
          <>
            <Typography variant="body2" color="text.secondary" textAlign="center">
              Нажмите Start в боте, затем введите полученный код:
            </Typography>
            {/* Manual retry: if the automatic redirect was swallowed (common on
                first launch inside a PWA), this tap re-runs openTelegramLogin
                synchronously inside the gesture, which is what makes it work. */}
            <MuiLink component="button" type="button" variant="body2" textAlign="center" onClick={() => openTelegramLogin(deepLink)}>
              Не открылся Telegram? Открыть ещё раз
            </MuiLink>
            <TextField
              label="Код из Telegram"
              value={code}
              onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              inputProps={{ inputMode: 'numeric', maxLength: 6 }}
              fullWidth
            />
            <Button variant="outlined" onClick={handleRedeem} disabled={code.length !== 6 || isRedeeming}>
              {isRedeeming ? <CircularProgress size={20} /> : 'Подтвердить'}
            </Button>
          </>
        )}

        <MuiLink
          component="button"
          variant="body2"
          sx={{ mt: 2 }}
          disabled={isCheckingSession}
          onClick={handleAlreadyLoggedIn}
        >
          {isCheckingSession ? 'Проверяем…' : 'Уже входили? Ввести PIN'}
        </MuiLink>
      </Stack>
    </Box>
  );
}
