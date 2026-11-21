import { NavLink } from 'react-router-dom';
import {
  LayoutDashboard,
  IdCard,
  ShieldAlert,
  Workflow,
  Scale,
  Building2,
  ScrollText,
  Bot,
  Layers,
  Package,
  Inbox,
  ChevronsLeft,
  ChevronsRight,
} from 'lucide-react';
import { useState } from 'react';

import { LogoMark } from './Logo';

const NAV_ITEMS = [
  { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/kyc', label: 'KYC Review', icon: IdCard },
  { to: '/fraud', label: 'Fraud Monitor', icon: ShieldAlert },
  { to: '/workflows', label: 'Workflow', icon: Workflow },
  { to: '/tasks', label: 'My Tasks', icon: Inbox },
  { to: '/recon', label: 'Đối soát', icon: Scale },
  { to: '/services', label: 'Service Catalog', icon: Layers },
  { to: '/offers', label: 'Offer Factory', icon: Package },
  { to: '/orgs', label: 'Tổ chức & Policy', icon: Building2 },
  { to: '/ai-settings', label: 'AI Settings', icon: Bot },
  { to: '/audit', label: 'Audit Log', icon: ScrollText },
];

export function Sidebar() {
  const [collapsed, setCollapsed] = useState(false);

  return (
    <aside
      className={`bg-[#0b1220] text-slate-400 flex flex-col transition-all duration-200 border-r border-slate-800 ${
        collapsed ? 'w-16 min-w-16' : 'w-58 min-w-58'
      }`}
    >
      <div className="h-14 flex items-center px-4 gap-2.5 border-b border-slate-800/60">
        <LogoMark size={24} />
        {!collapsed && (
          <strong className="text-white font-bold text-base tracking-tight">HPay Admin</strong>
        )}
      </div>

      <nav className="flex-1 p-2 flex flex-col gap-0.5 overflow-y-auto">
        {NAV_ITEMS.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            title={label}
            className={({ isActive }) =>
              `flex items-center gap-2.5 px-3 py-2.5 rounded-lg text-sm font-semibold transition-colors ${
                isActive
                  ? 'text-cyan-400 bg-slate-800'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
              } ${collapsed ? 'justify-center' : 'justify-start'}`
            }
          >
            <Icon size={17} />
            {!collapsed && label}
          </NavLink>
        ))}
      </nav>

      <button
        onClick={() => setCollapsed((c) => !c)}
        aria-label={collapsed ? 'Mở rộng menu' : 'Thu gọn menu'}
        className={`p-3.5 border-t border-slate-800/60 text-slate-400 hover:text-white transition-colors cursor-pointer flex ${
          collapsed ? 'justify-center' : 'justify-end'
        }`}
      >
        {collapsed ? <ChevronsRight size={16} /> : <ChevronsLeft size={16} />}
      </button>
    </aside>
  );
}
