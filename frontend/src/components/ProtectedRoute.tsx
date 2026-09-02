import { Navigate, Outlet } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { useAuthStore } from '@/store/authStore';

/** Redirects to /login when there is no authenticated user, and to /pin when
 * the user hasn't unlocked the app with their PIN yet this session. */
export function ProtectedRoute() {
  const { user, isPinUnlocked, isInitializing } = useAuthStore();

  if (isInitializing) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  if (!isPinUnlocked) {
    return <Navigate to="/pin" replace />;
  }

  return <Outlet />;
}
