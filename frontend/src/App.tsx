import { useEffect } from 'react';
import { Routes, Route } from 'react-router-dom';
import { Snackbar, Alert } from '@mui/material';

import { ProtectedRoute } from '@/components/ProtectedRoute';
import { AppLayout } from '@/components/AppLayout';
import { LoginPage } from '@/pages/LoginPage';
import { PinPage } from '@/pages/PinPage';
import { HomePage } from '@/pages/HomePage';
import { NotesPage } from '@/pages/NotesPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { useAuthStore } from '@/store/authStore';
import { useUiStore } from '@/store/uiStore';
import { fetchMe, updateSettings } from '@/api/auth';
import { detectTimezone } from '@/utils/datetime';

export default function App() {
  const { setUser, setInitializing } = useAuthStore();
  const { snackbar, hideSnackbar } = useUiStore();

  useEffect(() => {
    fetchMe()
      .then(async ({ user }) => {
        // New accounts keep the "UTC" default until the user opens Settings, so
        // the streak / character would roll over at UTC midnight instead of the
        // user's local midnight. Adopt the browser's zone once, silently.
        const browserTz = detectTimezone();
        if (user.timezone === 'UTC' && browserTz !== 'UTC') {
          try {
            const { user: synced } = await updateSettings({ timezone: browserTz });
            setUser(synced);
            return;
          } catch {
            // Non-fatal: fall back to the un-synced user; Settings still works.
          }
        }
        setUser(user);
      })
      .catch(() => setUser(null))
      .finally(() => setInitializing(false));
  }, [setUser, setInitializing]);

  return (
    <>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/pin" element={<PinPage />} />

        <Route element={<ProtectedRoute />}>
          <Route element={<AppLayout />}>
            <Route path="/" element={<HomePage />} />
            <Route path="/notes" element={<NotesPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Route>
        </Route>
      </Routes>

      <Snackbar
        open={Boolean(snackbar.message)}
        autoHideDuration={4000}
        onClose={hideSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert onClose={hideSnackbar} severity={snackbar.severity} variant="filled" sx={{ width: '100%' }}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </>
  );
}
