import { create } from 'zustand';

interface SecurityState {
  isLocked: boolean;
  isRooted: boolean;
  biometricEnabled: boolean;
  pendingRoute: string | null;
  setLocked: (val: boolean) => void;
  setRooted: (val: boolean) => void;
  setBiometricEnabled: (val: boolean) => void;
  setPendingRoute: (route: string | null) => void;
}

export const useSecurityStore = create<SecurityState>((set) => ({
  isLocked: false,
  isRooted: false,
  biometricEnabled: false, // User must explicitly enable biometric in settings
  pendingRoute: null,
  setLocked: (val) => set({ isLocked: val }),
  setRooted: (val) => set({ isRooted: val }),
  setBiometricEnabled: (val) => set({ biometricEnabled: val }),
  setPendingRoute: (route) => set({ pendingRoute: route }),
}));
