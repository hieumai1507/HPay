import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { ROUTE_ACCESS, canAccessRoute, type GuardedRoute } from '@/lib/access';
import { ROLE_PERMISSIONS, useAdminStore, type AdminRole } from '@/stores/admin';

import { RequirePermission } from './RequirePermission';

function loginAs(role: AdminRole) {
  useAdminStore.getState().setSession('t', { id: '1', email: 'a@b.c', full_name: 'A', role });
}

describe('RequirePermission', () => {
  beforeEach(() => sessionStorage.clear());
  afterEach(() => {
    cleanup();
    useAdminStore.getState().clearSession();
  });

  it('shows a no-access panel and hides the page for a viewer', () => {
    loginAs('viewer');
    render(
      <MemoryRouter>
        <RequirePermission anyOf={['ai.manage']}>
          <div>secret page</div>
        </RequirePermission>
      </MemoryRouter>,
    );
    expect(screen.queryByText('secret page')).toBeNull();
    expect(screen.getByRole('alert').textContent).toMatch(/No access/);
  });

  it('renders the page when any listed permission is held', () => {
    loginAs('ops');
    render(
      <MemoryRouter>
        <RequirePermission anyOf={['recon.run', 'ai.manage']}>
          <div>recon page</div>
        </RequirePermission>
      </MemoryRouter>,
    );
    expect(screen.getByText('recon page')).toBeTruthy();
  });
});

describe('route access matrix', () => {
  const can = (role: AdminRole) => (p: Parameters<typeof canAccessRoute>[0] extends (x: infer P) => boolean ? P : never) =>
    ROLE_PERMISSIONS[role].includes(p);
  const routes = Object.keys(ROUTE_ACCESS) as GuardedRoute[];

  it('viewer cannot open any guarded route', () => {
    for (const r of routes) expect(canAccessRoute(can('viewer'), r), r).toBe(false);
  });

  it('superadmin can open every guarded route', () => {
    for (const r of routes) expect(canAccessRoute(can('superadmin'), r), r).toBe(true);
  });

  it('ops reaches recon, orgs (read) and audit but not ai-settings or fraud', () => {
    expect(canAccessRoute(can('ops'), '/recon')).toBe(true);
    expect(canAccessRoute(can('ops'), '/orgs')).toBe(true);
    expect(canAccessRoute(can('ops'), '/audit')).toBe(true);
    expect(canAccessRoute(can('ops'), '/ai-settings')).toBe(false);
    expect(canAccessRoute(can('ops'), '/fraud')).toBe(false);
  });

  it('risk_officer reaches fraud but not recon or ai-settings', () => {
    expect(canAccessRoute(can('risk_officer'), '/fraud')).toBe(true);
    expect(canAccessRoute(can('risk_officer'), '/recon')).toBe(false);
    expect(canAccessRoute(can('risk_officer'), '/ai-settings')).toBe(false);
  });
});
