import type { ReactNode } from 'react';

import { canAccessAny } from '@/lib/access';
import { useAdminStore, type Permission } from '@/stores/admin';

/**
 * Route guard, nested INSIDE RequireAdmin: renders children only when the
 * admin holds at least one of `anyOf`; otherwise a visible "no access" panel.
 *
 * UX only. The backend does not check admin roles yet (audit BE-H6), so this
 * hides pages and buttons; it does not protect the API.
 */
export function RequirePermission({ anyOf, children }: { anyOf: readonly Permission[]; children: ReactNode }) {
  const can = useAdminStore((s) => s.can);
  const role = useAdminStore((s) => s.admin?.role);

  if (!canAccessAny(can, anyOf)) {
    return (
      <div role="alert" className="max-w-lg mx-auto mt-16 bg-white border border-slate-200 rounded-xl p-6 text-center">
        <div className="text-base font-bold text-slate-900">No access</div>
        <p className="text-sm text-slate-600 mt-2">
          Your role{role ? ` (${role})` : ''} cannot open this page. Ask a superadmin if you need access.
        </p>
      </div>
    );
  }
  return <>{children}</>;
}
