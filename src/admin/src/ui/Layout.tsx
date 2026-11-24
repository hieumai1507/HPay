import { Outlet, useLocation } from 'react-router-dom';

import { Sidebar } from './Sidebar';
import { Toasts } from './Toasts';
import { Topbar } from './Topbar';

const TITLES: Record<string, string> = {
  '/dashboard': 'Dashboard',
  '/kyc': 'KYC Review',
  '/fraud': 'Fraud Monitor',
  '/workflows': 'Workflow Monitor',
  '/tasks': 'My Tasks',
  '/attention': 'Needs attention',
  '/recon': 'Reconciliation',
  '/orgs': 'Organizations & Policy',
  '/ai-settings': 'AI Settings',
  '/audit': 'Audit Log',
};

export function Layout() {
  const { pathname } = useLocation();
  const title = Object.entries(TITLES).find(([p]) => pathname.startsWith(p))?.[1];

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50">
      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0">
        <Topbar title={title} />
        <main className="flex-1 overflow-y-auto p-6">
          <Outlet />
        </main>
        <Toasts />
      </div>
    </div>
  );
}
