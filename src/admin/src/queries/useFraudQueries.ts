import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { idempotencyHeaders } from '@/lib/idempotency';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export interface FraudFlagItem {
  id: string;
  flagNo: string;
  rule: string;
  score: number;
  txnRef: string;
  /** The flags API does not return an amount; null renders as an em dash (the old mapper invented 5,000,000). */
  amount: number | null;
  /** The flags API does not return the user; null renders as an em dash. */
  userName: string | null;
  userPhone?: string;
  createdAt: string;
  status: 'open' | 'cleared' | 'confirmed';
  rulesTriggered?: string[];
  aiAnalysis?: string;
}

// reasons is what internal/fraud/handler.go actually writes (Execute():
// json.Marshal(map[string]any{"signals": signals}), later merged with an
// "advisory" key by AppendFraudFlagReasons) — an object, never a plain
// string[].
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

export function useFraudFlags(paramsInput?: string | AdminQueryParams, enabled = true) {
  const params: AdminQueryParams = typeof paramsInput === 'string' ? { status: paramsInput } : (paramsInput || {});
  return useQuery({
    queryKey: ['admin', 'fraud-flags', params],
    enabled,
    // Errors propagate (the page shows them); an empty list is a real answer.
    queryFn: async (): Promise<Paged<FraudFlagItem>> => {
      const res = await apiRequest<{ flags: BackendFlagRow[] | null; total?: number }>(
        `/v1/admin/fraud-flags${buildQueryString(params)}`
      );
      const items: FraudFlagItem[] = (res?.flags ?? []).map((f: BackendFlagRow) => ({
        id: f.id,
        flagNo: `#${f.id.slice(0, 4)}`,
        rule: f.rule_code ?? '—',
        score: typeof f.score === 'number' ? f.score : 0,
        txnRef: f.ref_code ?? '—',
        amount: null,
        userName: null,
        createdAt: f.created_at ?? '',
        status: (f.status as FraudFlagItem['status']) || 'open',
        aiAnalysis: f.reasons?.advisory,
        rulesTriggered: f.reasons?.signals?.length
          ? f.reasons.signals.map((s) => s.evidence || s.code)
          : f.rule_code
            ? [`Rule ${f.rule_code}`]
            : [],
      }));
      // The API already applied the status filter, so `total` matches these
      // rows; filtering again here would desync total from what is shown.
      return { rows: items, total: res?.total ?? items.length };
    },
    refetchInterval: 30_000,
  });
}

export function useResolveFraudFlag() {
  const qc = useQueryClient();
  return useMutation({
    // POST /v1/admin/fraud-flags/{id}/resolve body {action: clear|confirm_fraud, reason<=255} -> 200 {status:"resolved"}.
    // One Idempotency-Key per mutate() call (per click).
    mutationFn: async ({ id, action, reason }: { id: string; action: 'clear' | 'confirm_fraud'; reason: string }) => {
      await apiRequest(`/v1/admin/fraud-flags/${id}/resolve`, {
        method: 'POST',
        body: { action, reason },
        headers: idempotencyHeaders(),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'fraud-flags'] });
    },
  });
}
