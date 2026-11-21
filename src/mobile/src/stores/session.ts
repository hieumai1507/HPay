import * as SecureStore from 'expo-secure-store';
import { create } from 'zustand';

import type { components } from '@/src/api/schema';

export type User = components['schemas']['User'];

const REFRESH_TOKEN_KEY = 'hpay_refresh_token';

/** Why the session ended — drives the copy shown on the (auth) screen (M04/M05). */
export type LogoutReason = 'user' | 'refresh_expired' | 'refresh_revoked' | null;

interface SessionState {
  /** Access token lives in memory ONLY — never persisted, never logged (doc 02 §5). */
  accessToken: string | null;
  user: User | null;
  status: 'bootstrapping' | 'authenticated' | 'unauthenticated';
  logoutReason: LogoutReason;
  kycSkippedInSession: boolean;
  setSession: (accessToken: string, user?: User | null) => void;
  clearSession: (reason: LogoutReason) => void;
  setKycSkipped: (val: boolean) => void;
}

export const useSessionStore = create<SessionState>((set) => ({
  accessToken: null,
  user: null,
  status: 'bootstrapping',
  logoutReason: null,
  kycSkippedInSession: false,
  setSession: (accessToken, user) =>
    set((state) => ({
      accessToken,
      user: user !== undefined ? user : state.user,
      status: 'authenticated',
      logoutReason: null,
    })),
  clearSession: (reason) =>
    set({
      accessToken: null,
      user: null,
      status: 'unauthenticated',
      logoutReason: reason,
      kycSkippedInSession: false,
    }),
  setKycSkipped: (val) => set({ kycSkippedInSession: val }),
}));

/** Synchronous accessor for the api client (avoids re-rendering hooks on every request). */
export function getAccessToken(): string | null {
  return useSessionStore.getState().accessToken;
}

export async function getStoredRefreshToken(): Promise<string | null> {
  return SecureStore.getItemAsync(REFRESH_TOKEN_KEY);
}

export async function storeRefreshToken(token: string): Promise<void> {
  await SecureStore.setItemAsync(REFRESH_TOKEN_KEY, token);
}

export async function clearStoredRefreshToken(): Promise<void> {
  await SecureStore.deleteItemAsync(REFRESH_TOKEN_KEY);
}
