import { createTheme, type Theme, type ThemeOptions } from '@mui/material/styles';

export type ThemeMode = 'light' | 'dark';

// Options shared by both palettes — typography, radius, component tweaks.
const sharedOptions: ThemeOptions = {
  shape: {
    borderRadius: 14,
  },
  typography: {
    fontFamily: [
      'Inter',
      '-apple-system',
      'Segoe UI',
      'Roboto',
      'Arial',
      'sans-serif',
    ].join(','),
  },
  components: {
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: 'none',
        },
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          textTransform: 'none',
          fontWeight: 600,
        },
      },
    },
  },
};

// Palette per mode. Light is the default (see themeStore); dark keeps the
// original NowDone colours.
const palettes: Record<ThemeMode, ThemeOptions['palette']> = {
  light: {
    mode: 'light',
    primary: { main: '#6d5efc' },
    secondary: { main: '#e5568f' },
    background: { default: '#f4f4f8', paper: '#ffffff' },
    success: { main: '#16a34a' },
  },
  dark: {
    mode: 'dark',
    primary: { main: '#7c6bff' },
    secondary: { main: '#ff6ba8' },
    background: { default: '#0f0f14', paper: '#17171f' },
    success: { main: '#4ade80' },
  },
};

/** Build the MUI theme for the given mode. */
export function createAppTheme(mode: ThemeMode): Theme {
  return createTheme({ ...sharedOptions, palette: palettes[mode] });
}

export const lightTheme = createAppTheme('light');
export const darkTheme = createAppTheme('dark');
