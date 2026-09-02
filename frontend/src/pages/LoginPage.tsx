import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Button, TextField, Typography, Stack, Link as MuiLink, CircularProgress } from '@mui/material';
import TelegramIcon from '@mui/icons-material/Telegram';

import { startLogin, redeemLogin } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useAuthStore } from '@/store/authStore';
import { useUiStore } from '@/store/uiStore';

export function LoginPage() {
  const navigate = useNavigate();
  const setUser = useAuthStore((s) => s.setUser);
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [isStarting, setIsStarting] = useState(false);
  const [isRedeeming, setIsRedeeming] = useState(false);

  async function handleStartLogin() {
    setIsStarting(true);
    try {
      const { deep_link } = await startLogin();
      setDeepLink(deep_link);
      window.open(deep_link, '_blank', 'noopener,noreferrer');
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
      setUser(user);
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

        {deepLink && (
          <>
            <Typography variant="body2" color="text.secondary" textAlign="center">
              Нажмите Start в боте, затем введите полученный код:
            </Typography>
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

        <MuiLink component="button" variant="body2" sx={{ mt: 2 }} onClick={() => navigate('/pin')}>
          Уже входили? Ввести PIN
        </MuiLink>
      </Stack>
    </Box>
  );
}
