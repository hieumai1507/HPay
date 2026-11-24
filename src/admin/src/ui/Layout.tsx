import { useEffect, useState } from 'react';
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
  const [menuOpen, setMenuOpen] = useState(false);
  // Close the phone menu whenever the page changes, and with Escape.
  useEffect(() => setMenuOpen(false), [pathname]);
  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setMenuOpen(false);
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [menuOpen]);

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50">
      <Sidebar mobileOpen={menuOpen} onNavigate={() => setMenuOpen(false)} />
      {menuOpen && (
        <div aria-hidden="true" onClick={() => setMenuOpen(false)} className="md:hidden fixed inset-0 z-30 bg-slate-900/50" />
      )}
      <div className="flex-1 flex flex-col min-w-0">
        <Topbar title={title} onMenu={() => setMenuOpen((o) => !o)} menuOpen={menuOpen} />
        <main className="flex-1 overflow-y-auto p-3 sm:p-6">
          <Outlet />
        </main>
        <Toasts />
      </div>
    </div>
  );
}
