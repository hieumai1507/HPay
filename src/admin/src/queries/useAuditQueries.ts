import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export interface AuditLogItem {
  id: string;
  timestamp: string;
  actor: string;
  actorRole: string;
  action: string;
  actionType: 'success' | 'danger' | 'warning' | 'info';
  resource: string;
  details: string;
  ip: string;
}

export interface BackendAuditRow {
  id: number;
  actor_id: string;
  actor_type: string;
  action: string;
  resource: string;
  reason?: string;
  ip?: string;
  created_at: string;
}

export function useAuditQueries(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'audit', params],
    // Server-side paginated: `total` drives pageCount, so it must be the full
    // server count for the current filter, not the rows on this page.
    queryFn: async (): Promise<Paged<AuditLogItem>> => {
      const res = await apiRequest<{ logs: BackendAuditRow[] | null; total?: number }>(
        `/v1/admin/audit${buildQueryString(params)}`
      );
      const rows = (res?.logs ?? []).map((row) => ({
        id: `aud-${row.id}`,
        timestamp: new Date(row.created_at).toLocaleTimeString('en-GB'),
        actor: row.actor_id ? `${row.actor_type}.${row.actor_id.slice(0, 6)}` : 'system',
        actorRole: row.actor_type,
        action: row.action,
        actionType: getActionTone(row.action),
        resource: row.resource,
        details: row.reason || 'System event',
        ip: row.ip || '—',
      }));
      // Older builds of the endpoint omit `total`; fall back so pageCount degrades to a single
      // page instead of breaking the table.
      return { rows, total: res?.total ?? rows.length };
    },
  });
}
function getActionTone(action: string): 'success' | 'danger' | 'warning' | 'info' {
  if (action.includes('REJECT') || action.includes('FLAG') || action.includes('FAIL')) return 'danger';
  if (action.includes('UPDATE') || action.includes('WARN')) return 'warning';
  if (action.includes('APPROVE') || action.includes('MATCH') || action.includes('SUCCESS')) return 'success';
  return 'info';
}
