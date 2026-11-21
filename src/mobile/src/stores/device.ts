import * as Crypto from 'expo-crypto';
import * as SecureStore from 'expo-secure-store';
import { create } from 'zustand';

const DEVICE_ID_KEY = 'hpay_device_id';

interface DeviceState {
  deviceId: string | null;
  ready: boolean;
}

export const useDeviceStore = create<DeviceState>(() => ({
  deviceId: null,
  ready: false,
}));

let initPromise: Promise<string> | null = null;

/**
 * Loads the persisted device id, generating + persisting one on first run.
 * Memoized — safe to call from many places (bootstrap, api client) without duplicate work.
 */
export function ensureDeviceId(): Promise<string> {
  if (initPromise) return initPromise;
  initPromise = (async () => {
    let id = await SecureStore.getItemAsync(DEVICE_ID_KEY);
    if (!id) {
      id = Crypto.randomUUID();
      await SecureStore.setItemAsync(DEVICE_ID_KEY, id);
    }
    useDeviceStore.setState({ deviceId: id, ready: true });
    return id;
  })();
  return initPromise;
}

/** Synchronous accessor for request headers. Returns null until ensureDeviceId() has resolved once. */
export function getDeviceId(): string | null {
  return useDeviceStore.getState().deviceId;
}
