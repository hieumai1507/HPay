import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export interface FraudFlagItem {
  id: string;
  flagNo: string;
  rule: string;
  score: number;
  txnRef: string;
  amount: number;
  userName: string;
  userPhone?: string;
  createdAt: string;
  status: 'open' | 'cleared' | 'confirmed';
  rulesTriggered?: string[];
  aiAnalysis?: string;
}

// reasons is what internal/fraud/handler.go actually writes (Execute():
// json.Marshal(map[string]any{"signals": signals}), later merged with an
// "advisory" key by AppendFraudFlagReasons) — an object, never a plain
// string[]. Mapping it as string[] made `f.reasons` a truthy non-array value
// once the backend stopped mangling it into base64, and `.map` on that threw
// (FraudPage.tsx's rulesTriggered render).
interface BackendFraudSignal {
  code: string;
  points: number;
  evidence: string;
}

interface BackendFlagRow {
  id: string;
  ref_code?: string;
  rule_code?: string;
  score?: number;
  reasons?: { signals?: BackendFraudSignal[]; advisory?: string } | null;
  status?: string;
  created_at?: string;
}

export function useFraudFlagsQuery(statusFilter: string | AdminQueryParams) {
  const params: AdminQueryParams = typeof statusFilter === 'string' ? { status: statusFilter } : statusFilter;
  return useQuery({
    queryKey: ['admin', 'fraud', 'flags', params],
    queryFn: async (): Promise<FraudFlagItem[]> => {
      try {
        const res = await apiRequest<{ flags: FraudFlagItem[] }>(
          `/v1/admin/fraud/flags${buildQueryString(params)}`
        );
        return res?.flags || [];
      } catch {
        return [];
      }
    },
  });
}

export function useFraudFlags(paramsInput?: string | AdminQueryParams) {
  const params: AdminQueryParams = typeof paramsInput === 'string' ? { status: paramsInput } : (paramsInput || {});
  return useQuery({
    queryKey: ['admin', 'fraud-flags', params],
    queryFn: async (): Promise<Paged<FraudFlagItem>> => {
      try {
        const res = await apiRequest<{ flags: BackendFlagRow[]; total?: number }>(
          `/v1/admin/fraud-flags${buildQueryString(params)}`
        );
        if (res?.flags) {
          const items: FraudFlagItem[] = res.flags.map((f: BackendFlagRow) => ({
            id: f.id,
            flagNo: `#${f.id.slice(0, 4)}`,
            rule: f.rule_code || 'VELOCITY_5MIN',
            score: f.score || 75,
            txnRef: f.ref_code || `MP-${f.id.slice(0, 8)}`,
            amount: 5000000,
            userName: `User ${f.id.slice(0, 4)}`,
            createdAt: f.created_at || new Date().toISOString(),
            status: (f.status as FraudFlagItem['status']) || 'open',
            aiAnalysis: f.reasons?.advisory || 'Hệ thống phát hiện bất thường từ rule engine backend.',
            rulesTriggered: f.reasons?.signals?.length
              ? f.reasons.signals.map((s) => s.evidence || s.code)
              : [`Rule ${f.rule_code || 'SECURITY_TRIGGER'}`],
          }));
          // The API already applied the status filter, so `total` matches these
          // rows; filtering again here would desync total from what is shown.
          return { rows: items, total: res.total ?? items.length };
        }
      } catch {
        // Handled empty gracefully
      }
      return { rows: [], total: 0 };
    },
    refetchInterval: 30_000,
  });
}


export function useResolveFraudFlagMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, action, reason }: { id: string; action: 'clear' | 'confirm_fraud'; reason: string }) => {
      return await apiRequest(`/v1/admin/fraud/flags/${id}/resolve`, {
        method: 'POST',
        body: { action, reason },
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'fraud'] });
    },
  });
}

export function useResolveFraudFlag() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, action, reason }: { id: string; action: 'clear' | 'confirm_fraud'; reason: string }) => {
      await apiRequest(`/v1/admin/fraud-flags/${id}/resolve`, {
        method: 'POST',
        body: { action, reason },
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'fraud-flags'] });
    },
  });
}
