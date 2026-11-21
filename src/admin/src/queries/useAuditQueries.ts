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
      try {
        const res = await apiRequest<{ logs: BackendAuditRow[]; total?: number }>(
          `/v1/admin/audit${buildQueryString(params)}`
        );
        if (res?.logs) {
          const rows = res.logs.map((row) => ({
            id: `aud-${row.id}`,
            timestamp: new Date(row.created_at).toLocaleTimeString('vi-VN'),
            actor: row.actor_id ? `${row.actor_type}.${row.actor_id.slice(0, 6)}` : 'system',
            actorRole: row.actor_type,
            action: row.action,
            actionType: getActionTone(row.action),
            resource: row.resource,
            details: row.reason || 'Sự kiện hệ thống',
            ip: row.ip || '—',
          }));
          // Older builds of the endpoint omit `total`; fall back so pageCount
          // degrades to a single page instead of breaking the table.
          return { rows, total: res.total ?? rows.length };
        }
      } catch {
        // Fallback demo items
      }
      const demo: AuditLogItem[] = [
        { id: 'aud-101', timestamp: '14:23:01', actor: 'admin.098812', actorRole: 'super_admin', action: 'POLICY_UPDATE', actionType: 'warning', resource: 'OrgPolicy #3', details: 'Thay đổi hạn mức chi lương', ip: '118.69.182.10' },
        { id: 'aud-102', timestamp: '14:15:44', actor: 'system', actorRole: 'system', action: 'RECON_MATCH', actionType: 'success', resource: 'ReconJob #992', details: 'Khớp 100% 1,420 giao dịch Bank', ip: '127.0.0.1' },
        { id: 'aud-103', timestamp: '13:58:12', actor: 'user.091234', actorRole: 'customer', action: 'TRANSFER_P2P', actionType: 'info', resource: 'Txn #TX8812', details: 'Chuyển tiền P2P 500,000đ', ip: '14.225.21.19' },
      ];
      return { rows: demo, total: demo.length };
    },
  });
}

function getActionTone(action: string): 'success' | 'danger' | 'warning' | 'info' {
  if (action.includes('REJECT') || action.includes('FLAG') || action.includes('FAIL')) return 'danger';
  if (action.includes('UPDATE') || action.includes('WARN')) return 'warning';
  if (action.includes('APPROVE') || action.includes('MATCH') || action.includes('SUCCESS')) return 'success';
  return 'info';
}
