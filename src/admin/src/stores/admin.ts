import { create } from 'zustand';

export type AdminRole = 'viewer' | 'kyc_reviewer' | 'risk_officer' | 'ops' | 'superadmin';

export type Permission =
  | 'kyc.review'
  | 'fraud.resolve'
  | 'workflow.action'
  | 'recon.run'
  | 'orgs.edit'
  | 'ai.manage'
  | 'admins.manage';

// FE-side visibility only (UX). The backend is the real enforcement point —
// nothing here grants any capability the API would not verify again.
export const ROLE_PERMISSIONS: Record<AdminRole, readonly Permission[]> = {
  viewer: [],
  kyc_reviewer: ['kyc.review'],
  risk_officer: ['fraud.resolve', 'workflow.action'],
  ops: ['workflow.action', 'recon.run'],
  superadmin: [
    'kyc.review',
    'fraud.resolve',
    'workflow.action',
    'recon.run',
    'orgs.edit',
    'ai.manage',
    'admins.manage',
  ],
};

export interface AdminProfile {
  id: string;
  email: string;
  full_name: string;
  role: AdminRole;
}

interface AdminState {
  token: string | null;
  admin: AdminProfile | null;
  setSession: (token: string, admin: AdminProfile) => void;
  clearSession: () => void;
  can: (permission: Permission) => boolean;
}

const STORAGE_KEY = 'hpay-admin-session';

// sessionStorage on purpose: an admin session must not survive closing the tab.
function loadInitial(): Pick<AdminState, 'token' | 'admin'> {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as Pick<AdminState, 'token' | 'admin'>;
  } catch {
    sessionStorage.removeItem(STORAGE_KEY);
  }
  return { token: null, admin: null };
}

export const useAdminStore = create<AdminState>()((set, get) => ({
  ...loadInitial(),
  setSession: (token, admin) => {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({ token, admin }));
    set({ token, admin });
  },
  clearSession: () => {
    sessionStorage.removeItem(STORAGE_KEY);
    set({ token: null, admin: null });
  },
  can: (permission) => {
    const role = get().admin?.role;
    if (!role) return false;
    return (ROLE_PERMISSIONS[role] ?? []).includes(permission);
  },
}));
