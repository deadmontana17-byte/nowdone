import { useEffect, useRef } from 'react';
import { Outlet, useNavigate, useLocation } from 'react-router-dom';
import { AppBar, Toolbar, Typography, BottomNavigation, BottomNavigationAction, Box, Paper } from '@mui/material';
import { alpha } from '@mui/material/styles';
import ChecklistIcon from '@mui/icons-material/Checklist';
import StickyNote2Icon from '@mui/icons-material/StickyNote2';
import SettingsIcon from '@mui/icons-material/Settings';

import { StreakBar } from '@/components/StreakBar';

const NAV_ROUTES = ['/', '/notes', '/settings'];

export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const currentIndex = Math.max(NAV_ROUTES.indexOf(location.pathname), 0);

  // Expose the sticky AppBar's actual rendered height (Toolbar + StreakBar,
  // which varies with content) as a CSS variable, so page content below it
  // (e.g. HomePage's own sticky month bar) can stick right beneath it
  // instead of the two overlapping at a hardcoded guess.
  const appBarRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = appBarRef.current;
    if (!el) return;
    const setVar = () => document.documentElement.style.setProperty('--app-bar-height', `${el.offsetHeight}px`);
    setVar();
    const observer = new ResizeObserver(setVar);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100vh', pb: 10 }}>
      {/* backgroundColor is resolved from the current theme (sx callback re-runs
          on theme switch), so the header follows light/dark instead of staying
          fully transparent and showing the previous theme's colour. */}
      <AppBar
        ref={appBarRef}
        position="sticky"
        elevation={0}
        color="transparent"
        sx={{
          backdropFilter: 'blur(12px)',
          backgroundColor: (theme) => alpha(theme.palette.background.default, 0.8),
          borderBottom: (theme) => `1px solid ${theme.palette.divider}`,
        }}
      >
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
          sx={{
            height: 76,
            '& .MuiBottomNavigationAction-root': { pt: 1 },
            '& .MuiBottomNavigationAction-label': { fontSize: '0.8rem' },
            '& .MuiBottomNavigationAction-label.Mui-selected': { fontSize: '0.85rem' },
            '& .MuiSvgIcon-root': { fontSize: '1.6rem' },
          }}
        >
          <BottomNavigationAction label="Задачи" icon={<ChecklistIcon />} />
          <BottomNavigationAction label="Заметки" icon={<StickyNote2Icon />} />
          <BottomNavigationAction label="Настройки" icon={<SettingsIcon />} />
        </BottomNavigation>
      </Paper>
    </Box>
  );
}
