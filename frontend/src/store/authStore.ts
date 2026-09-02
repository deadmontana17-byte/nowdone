import { create } from 'zustand';
import type { User } from '@/types';

interface AuthState {
  user: User | null;
  isPinUnlocked: boolean;
  isInitializing: boolean;
  setUser: (user: User | null) => void;
  setPinUnlocked: (unlocked: boolean) => void;
  setInitializing: (value: boolean) => void;
  reset: () => void;
}

/** Holds the current authenticated user and whether the PIN gate has been
 * passed for this session (unlocking the app UI and hidden notes). */
export const useAuthStore = create<AuthState>((set) => ({
  user: null,
  isPinUnlocked: false,
  isInitializing: true,
  setUser: (user) => set({ user }),
  setPinUnlocked: (isPinUnlocked) => set({ isPinUnlocked }),
  setInitializing: (isInitializing) => set({ isInitializing }),
  reset: () => set({ user: null, isPinUnlocked: false }),
}));
