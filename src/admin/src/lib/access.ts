import type { Permission } from '@/stores/admin';

/**
 * Route-level view access for the admin app. UX ONLY: the backend does not
 * enforce admin roles yet (audit BE-H6), so none of this is a security
 * boundary. A user who calls the API directly is not affected by it.
 *
 * A route is reachable when the admin holds ANY of the listed permissions.
 * Action buttons inside a page use the single specific permission (ACTION_PERMISSION).
 * Product decision pending: services/offers have no dedicated permission yet,
 * so they are limited to the platform-configuration permissions (superadmin).
 */
export const ROUTE_ACCESS = {
  '/orgs': ['orgs.edit', 'workflow.action'],
  '/ai-settings': ['ai.manage'],
  '/services': ['admins.manage', 'orgs.edit', 'ai.manage'],
  '/offers': ['admins.manage', 'orgs.edit', 'ai.manage'],
  '/recon': ['recon.run'],
  '/audit': ['kyc.review', 'fraud.resolve', 'workflow.action', 'recon.run', 'orgs.edit', 'ai.manage', 'admins.manage'],
  '/fraud': ['fraud.resolve'],
  '/attention': ['workflow.action'],
} as const satisfies Record<string, readonly Permission[]>;

export type GuardedRoute = keyof typeof ROUTE_ACCESS;

/** Permission needed for the write actions on a page. */
export const ACTION_PERMISSION = {
  orgPolicy: 'orgs.edit',
  aiSettings: 'ai.manage',
  recon: 'recon.run',
  fraud: 'fraud.resolve',
  serviceConfig: 'admins.manage',
  offerConfig: 'admins.manage',
} as const satisfies Record<string, Permission>;

export function canAccessAny(can: (p: Permission) => boolean, anyOf: readonly Permission[]): boolean {
  return anyOf.some((p) => can(p));
}

export function canAccessRoute(can: (p: Permission) => boolean, route: GuardedRoute): boolean {
  return canAccessAny(can, ROUTE_ACCESS[route]);
}

/** Visible reason shown on a disabled action. */
export const NO_PERMISSION_REASON = 'Your role does not have permission for this action.';
