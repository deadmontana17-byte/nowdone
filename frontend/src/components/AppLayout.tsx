import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { AppBar, Toolbar, Typography, BottomNavigation, BottomNavigationAction, Box, Paper } from '@mui/material';
import ChecklistIcon from '@mui/icons-material/Checklist';
import StickyNote2Icon from '@mui/icons-material/StickyNote2';
import SettingsIcon from '@mui/icons-material/Settings';

import { StreakBar } from '@/components/StreakBar';

const NAV_ROUTES = ['/', '/notes', '/settings'];

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const currentIndex = Math.max(NAV_ROUTES.indexOf(location.pathname), 0);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', pb: 8 }}>
      <AppBar position="sticky" elevation={0} color="transparent" sx={{ backdropFilter: 'blur(12px)' }}>
        <Toolbar>
          <Typography variant="h6" sx={{ fontWeight: 700, flexGrow: 1 }}>
            NowDone
          </Typography>
        </Toolbar>
        <StreakBar />
      </AppBar>

      <Box component="main" sx={{ flexGrow: 1, px: { xs: 1.5, sm: 3 }, py: 2, maxWidth: 720, mx: 'auto', width: '100%' }}>
        <Outlet />
      </Box>

      <Paper elevation={8} sx={{ position: 'fixed', bottom: 0, left: 0, right: 0 }}>
        <BottomNavigation
          showLabels
          value={currentIndex}
          onChange={(_, newIndex) => navigate(NAV_ROUTES[newIndex])}
        >
          <BottomNavigationAction label="Задачи" icon={<ChecklistIcon />} />
          <BottomNavigationAction label="Заметки" icon={<StickyNote2Icon />} />
          <BottomNavigationAction label="Настройки" icon={<SettingsIcon />} />
        </BottomNavigation>
      </Paper>
    </Box>
  );
}
