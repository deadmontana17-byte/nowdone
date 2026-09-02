import { create } from 'zustand';

/**
 * Session-scoped unlock state for hidden ("под PIN") notes on the Notes page.
 *
 * Deliberately NOT persisted (no localStorage, not even sessionStorage): the
 * store is recreated on every full page load, so a reload always starts locked.
 * NotesPage additionally calls `lock()` on unmount, so navigating away and back
 * also requires re-entering the PIN.
 *
 * `hiddenNotesVisible` is kept as an explicit alias of `isUnlocked` because the
 * spec names both; they always change together via `unlock()` / `lock()`.
 */
interface NotesState {
  isUnlocked: boolean;
  hiddenNotesVisible: boolean;
  unlock: () => void;
  lock: () => void;
}

export const useNotesStore = create<NotesState>((set) => ({
  isUnlocked: false,
  hiddenNotesVisible: false,
  unlock: () => set({ isUnlocked: true, hiddenNotesVisible: true }),
  lock: () => set({ isUnlocked: false, hiddenNotesVisible: false }),
}));
