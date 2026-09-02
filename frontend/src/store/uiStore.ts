import { create } from 'zustand';
import { todayISODate } from '@/utils/datetime';

interface SnackbarState {
  message: string | null;
  severity: 'success' | 'error' | 'info' | 'warning';
}

interface UiState {
  snackbar: SnackbarState;
  showSnackbar: (message: string, severity?: SnackbarState['severity']) => void;
  hideSnackbar: () => void;
  selectedDate: string; // YYYY-MM-DD, the currently focused day in the planner
  setSelectedDate: (date: string) => void;
}

/** Global UI state: Snackbar messages (per global error-handling rule) and
 * the planner's currently selected date. */
export const useUiStore = create<UiState>((set) => ({
  snackbar: { message: null, severity: 'info' },
  showSnackbar: (message, severity = 'info') => set({ snackbar: { message, severity } }),
  hideSnackbar: () => set((s) => ({ snackbar: { ...s.snackbar, message: null } })),
  selectedDate: todayISODate(), // local calendar date, not UTC
  setSelectedDate: (selectedDate) => set({ selectedDate }),
}));
