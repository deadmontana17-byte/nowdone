import { useMemo } from 'react';
import { BrowserRouter } from 'react-router-dom';
import { ThemeProvider, CssBaseline } from '@mui/material';

import App from './App';
import { ErrorBoundary } from './components/ErrorBoundary';
import { createAppTheme } from './theme';
import { useThemeStore } from './store/themeStore';

/** Rebuilds the MUI theme whenever the user flips the light/dark preference,
 * so the switch applies globally to every component. */
export function ThemedApp() {
  const mode = useThemeStore((s) => s.mode);
  const theme = useMemo(() => createAppTheme(mode), [mode]);

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <ErrorBoundary>
        <BrowserRouter>
          <App />
        </BrowserRouter>
      </ErrorBoundary>
    </ThemeProvider>
  );
}
