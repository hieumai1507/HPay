import { Navigate, useLocation } from 'react-router-dom';
import type { ReactNode } from 'react';

import { useAdminStore } from '@/stores/admin';

/** Route guard: no token → /login, preserving the attempted URL (incl. filters in search params, A03). */
export function RequireAdmin({ children }: { children: ReactNode }) {
  const token = useAdminStore((s) => s.token);
  const location = useLocation();

  if (!token) {
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />;
  }
  return <>{children}</>;
}
