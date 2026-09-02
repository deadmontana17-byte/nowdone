import { useEffect, useLayoutEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Box, Typography, CircularProgress, Link as MuiLink, Stack } from '@mui/material';

import { PinInput } from '@/components/PinInput';
import { PinResetDialog } from '@/components/PinResetDialog';
import { setPin, verifyPin } from '@/api/auth';
import { ApiError } from '@/api/client';
import { useAuthStore } from '@/store/authStore';
import { useUiStore } from '@/store/uiStore';

type Mode = 'enter' | 'create';

export function PinPage() {
  const navigate = useNavigate();
  const { user, isInitializing, setPinUnlocked } = useAuthStore();
  const showSnackbar = useUiStore((s) => s.showSnackbar);

  const [mode, setMode] = useState<Mode>(user?.has_pin ? 'enter' : 'create');
  const [pin, setPinValue] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);

  // This route isn't gated by <ProtectedRoute>, so it can mount before
  // fetchMe() resolves. `mode` is seeded from a possibly-null `user`, so
  // re-sync it once auth settles. useLayoutEffect keeps the wrong form from
  // flashing.
  useLayoutEffect(() => {
    if (isInitializing || !user) return;
    setMode(user.has_pin ? 'enter' : 'create');
  }, [isInitializing, user]);

  useEffect(() => {
    if (!isInitializing && !user) navigate('/login', { replace: true });
  }, [isInitializing, user, navigate]);

  if (isInitializing || !user) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <CircularProgress />
      </Box>
    );
  }

  async function handleEnterComplete(value: string) {
    setIsSubmitting(true);
    try {
      await verifyPin(value);
      setPinUnlocked(true);
      navigate('/', { replace: true });
    } catch {
      showSnackbar('Неверный PIN', 'error');
      setPinValue('');
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleCreateComplete(value: string) {
    setIsSubmitting(true);
    try {
      await setPin(value);
      setPinUnlocked(true);
      showSnackbar('PIN установлен', 'success');
      navigate('/', { replace: true });
    } catch (err) {
      showSnackbar(err instanceof ApiError ? err.message : 'Не удалось установить PIN', 'error');
      setPinValue('');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', minHeight: '100vh', px: 3 }}>
      <Typography variant="h5" sx={{ fontWeight: 700, mb: 1 }}>
        {mode === 'enter' ? 'Введите PIN' : 'Придумайте PIN'}
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 4, textAlign: 'center' }}>
        {mode === 'enter'
          ? 'Для доступа к NowDone введите ваш 4-значный PIN'
          : 'Он понадобится при каждом входе в приложение'}
      </Typography>

      <Stack spacing={3} alignItems="center" sx={{ width: '100%', maxWidth: 360 }}>
        <PinInput
          value={pin}
          onChange={setPinValue}
          onComplete={mode === 'enter' ? handleEnterComplete : handleCreateComplete}
        />

        {mode === 'enter' && (
          <MuiLink component="button" variant="body2" onClick={() => setResetOpen(true)} disabled={isSubmitting}>
            Забыли PIN?
          </MuiLink>
        )}
      </Stack>

      <PinResetDialog
        open={resetOpen}
        onClose={() => setResetOpen(false)}
        onDone={() => {
          setResetOpen(false);
          navigate('/', { replace: true });
        }}
      />
    </Box>
  );
}
