import { useNavigate } from 'react-router-dom';
import { LogOut } from 'lucide-react';

import { useAdminStore } from '@/stores/admin';

import { Badge } from './Badge';

const ROLE_LABELS: Record<string, string> = {
  viewer: 'Viewer',
  kyc_reviewer: 'KYC Reviewer',
  risk_officer: 'Risk Officer',
  ops: 'Ops',
  superadmin: 'Superadmin',
};

export function Topbar({ title }: { title?: string }) {
  const admin = useAdminStore((s) => s.admin);
  const clearSession = useAdminStore((s) => s.clearSession);
  const navigate = useNavigate();

  return (
    <header className="h-14 bg-white border-b border-slate-200 flex items-center justify-between px-6 gap-4">
      <strong className="text-base font-bold text-slate-900">{title ?? ''}</strong>
      <div className="flex items-center gap-3">
        {admin && (
          <>
            <span className="text-sm font-medium text-slate-700">{admin.full_name || admin.email}</span>
            <Badge tone="info">{ROLE_LABELS[admin.role] ?? admin.role}</Badge>
          </>
        )}
        <button
          onClick={() => {
            clearSession();
            navigate('/login', { replace: true });
          }}
          aria-label="Log out"
          title="Log out"
          className="p-1.5 rounded-lg text-slate-400 hover:text-slate-700 hover:bg-slate-100 transition-colors cursor-pointer"
        >
          <LogOut size={17} />
        </button>
      </div>
    </header>
  );
}
