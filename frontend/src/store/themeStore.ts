import { create } from 'zustand';
import type { ThemeMode } from '@/theme';

const STORAGE_KEY = 'nowdone.theme';

/** Read the saved preference. Defaults to 'light' — the app ships light. */
function initialMode(): ThemeMode {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'light' || saved === 'dark') return saved;
  } catch {
    // localStorage unavailable (private mode / SSR) — fall through to default.
  }
  return 'light';
}

function persist(mode: ThemeMode) {
  try {
    localStorage.setItem(STORAGE_KEY, mode);
  } catch {
    // Best-effort: preference just won't survive a reload.
  }
}

interface ThemeState {
  mode: ThemeMode;
  isDark: boolean;
  setMode: (mode: ThemeMode) => void;
  toggleDark: () => void;
}

/** Global light/dark preference, persisted to localStorage. */
export const useThemeStore = create<ThemeState>((set) => ({
  mode: initialMode(),
  isDark: initialMode() === 'dark',
  setMode: (mode) => {
    persist(mode);
    set({ mode, isDark: mode === 'dark' });
  },
  toggleDark: () =>
    set((s) => {
      const mode: ThemeMode = s.mode === 'dark' ? 'light' : 'dark';
      persist(mode);
      return { mode, isDark: mode === 'dark' };
    }),
}));
