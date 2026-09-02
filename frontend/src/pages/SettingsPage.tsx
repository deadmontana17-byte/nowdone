import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Box, Typography, Button, Stack, Paper, Divider, TextField, MenuItem,
  FormControlLabel, Switch,
} from '@mui/material';
import LogoutIcon from '@mui/icons-material/Logout';
import LockResetIcon from '@mui/icons-material/LockReset';
import MyLocationIcon from '@mui/icons-material/MyLocation';

import { logout, updateSettings } from '@/api/auth';
import { PinResetDialog } from '@/components/PinResetDialog';
import { useAuthStore } from '@/store/authStore';
import { useUiStore } from '@/store/uiStore';
import { useThemeStore } from '@/store/themeStore';
import { TIMEZONE_OPTIONS, detectTimezone } from '@/utils/datetime';

export function SettingsPage() {
  const navigate = useNavigate();
  const { user, setUser, reset } = useAuthStore();
  const showSnackbar = useUiStore((s) => s.showSnackbar);
  const isDark = useThemeStore((s) => s.isDark);
  const toggleDark = useThemeStore((s) => s.toggleDark);
  const [savingTz, setSavingTz] = useState(false);
  const [resetOpen, setResetOpen] = useState(false);

  // Show the user's zone even if it isn't in the preset list.
  const tzOptions = user && !TIMEZONE_OPTIONS.includes(user.timezone)
    ? [user.timezone, ...TIMEZONE_OPTIONS]
    : TIMEZONE_OPTIONS;

  async function saveTimezone(timezone: string) {
    if (!timezone || timezone === user?.timezone) return;
    setSavingTz(true);
    try {
      const { user: updated } = await updateSettings({ timezone });
      setUser(updated);
      showSnackbar('Часовой пояс сохранён', 'success');
    } catch {
      showSnackbar('Не удалось сохранить часовой пояс', 'error');
    } finally {
      setSavingTz(false);
    }
  }

  async function handleLogout() {
    try {
      await logout();
    } finally {
      reset();
      navigate('/login', { replace: true });
    }
  }

  return (
    <Box>
      <Typography variant="h6" sx={{ mb: 2 }}>Настройки</Typography>

      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary">Профиль</Typography>
        <Typography sx={{ mt: 0.5 }}>{user?.first_name || 'Пользователь'}</Typography>
        <Divider sx={{ my: 1.5 }} />
        <Typography variant="body2" color="text.secondary">
          Текущий стрик: {user?.current_streak ?? 0} · Рекорд: {user?.max_streak ?? 0}
        </Typography>
      </Paper>

      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
          Оформление
        </Typography>
        <FormControlLabel
          control={<Switch checked={isDark} onChange={toggleDark} />}
          label="Тёмная тема"
        />
        <Typography variant="caption" color="text.secondary" display="block">
          По умолчанию используется светлая тема
        </Typography>
      </Paper>

      <Paper sx={{ p: 2, mb: 2 }}>
        <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1.5 }}>
          Часовой пояс
        </Typography>
        <Stack spacing={1.5}>
          <TextField
            select
            label="Часовой пояс"
            value={user?.timezone ?? 'UTC'}
            onChange={(e) => saveTimezone(e.target.value)}
            disabled={savingTz}
            fullWidth
            helperText="Напоминания приходят по этому времени"
          >
            {tzOptions.map((tz) => (
              <MenuItem key={tz} value={tz}>{tz}</MenuItem>
            ))}
          </TextField>
          <Button
            variant="text"
            size="small"
            startIcon={<MyLocationIcon />}
            disabled={savingTz}
            onClick={() => saveTimezone(detectTimezone())}
            sx={{ alignSelf: 'flex-start' }}
          >
            Определить автоматически ({detectTimezone()})
          </Button>
        </Stack>
      </Paper>

      <Stack spacing={1.5}>
        <Button variant="outlined" startIcon={<LockResetIcon />} onClick={() => setResetOpen(true)}>
          Сбросить PIN
        </Button>
        <Button variant="outlined" color="error" startIcon={<LogoutIcon />} onClick={handleLogout}>
          Выйти
        </Button>
      </Stack>

      <PinResetDialog
        open={resetOpen}
        onClose={() => setResetOpen(false)}
        onDone={() => setResetOpen(false)}
      />
    </Box>
  );
}
