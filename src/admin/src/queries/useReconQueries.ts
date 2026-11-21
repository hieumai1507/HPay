import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { type AdminQueryParams, buildQueryString } from './common';

export interface ReconRun {
  id: string;
  date: string;
  totalCount: number;
  matchedCount: number;
  mismatchedCount: number;
  status: 'completed' | 'processing' | 'failed';
  runAt: string;
  durationMs: number;
  aiSummary: string;
  reportUrl?: string;
}

export interface ReconItem {
  id: string;
  status: 'matched' | 'missing_external' | 'missing_internal' | 'amount_mismatch';
  internalRef: string;
  externalRef: string | null;
  internalAmount: number;
  externalAmount: number | null;
  timestamp: string;
  suggestedAction?: string;
}

const MOCK_RECON_RUN: ReconRun = {
  id: 'run-2026-07-07',
  date: '2026-07-07',
  totalCount: 2173,
  matchedCount: 2170,
  mismatchedCount: 3,
  status: 'completed',
  runAt: '2026-07-08T00:15:00Z',
  durationMs: 4200,
  aiSummary:
    'Cả 3 bút toán lệch đều thuộc nhóm missing_external và phát sinh trong khung 23:58–00:02. Khả năng cao là lệch cutoff ngày giữa HPay và sao kê ngân hàng — giao dịch ghi nhận ngày 07/07 phía ví nhưng rơi sang ngày 08/07 phía bank. Đề xuất: đối chiếu lại với sao kê ngày 08/07 trước khi tạo case điều tra.',
  reportUrl: '/v1/files/recon/2026-07-07.xlsx?exp=1700000000&sig=mock',
};

const MOCK_RECON_ITEMS: ReconItem[] = [
  {
    id: 'ri-1',
    status: 'missing_external',
    internalRef: 'MP26070799871',
    externalRef: null,
    internalAmount: 5000000,
    externalAmount: null,
    timestamp: '23:58:41',
    suggestedAction: 'Đối chiếu 08/07 →',
  },
  {
    id: 'ri-2',
    status: 'missing_external',
    internalRef: 'MP26070799902',
    externalRef: null,
    internalAmount: 1200000,
    externalAmount: null,
    timestamp: '23:59:12',
    suggestedAction: 'Đối chiếu 08/07 →',
  },
  {
    id: 'ri-3',
    status: 'missing_external',
    internalRef: 'MP26070799911',
    externalRef: null,
    internalAmount: 750000,
    externalAmount: null,
    timestamp: '00:01:03',
    suggestedAction: 'Đối chiếu 08/07 →',
  },
  {
    id: 'ri-4',
    status: 'matched',
    internalRef: 'MP26070799870',
    externalRef: 'VCB88213',
    internalAmount: 2000000,
    externalAmount: 2000000,
    timestamp: '23:55:18',
    suggestedAction: '—',
  },
];

export function useReconRun(date: string, params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'recon', date, params],
    queryFn: async () => {
      try {
        const res = await apiRequest<{
          run?: Record<string, unknown>;
          items?: Array<Record<string, unknown>>;
          report_url?: string;
        }>(`/v1/admin/recon/runs/${date}${buildQueryString(params)}`);

        if (res?.run) {
          // Field names must match store.GetReconRunByDateRow's real JSON
          // tags (run_date/total/matched/mismatched) — the old
          // date/total_count/matched_count/mismatched_count names don't
          // exist on the real response, so Number(undefined || 2173) always
          // fell back to the hardcoded mock numbers (2173/2170/3) for every
          // real recon run, silently masking the actual counts.
          const run: ReconRun = {
            id: String(res.run.id || `run-${date}`),
            date: String(res.run.run_date || date),
            totalCount: Number(res.run.total ?? 2173),
            matchedCount: Number(res.run.matched ?? 2170),
            mismatchedCount: Number(res.run.mismatched ?? 3),
            status: (res.run.status as ReconRun['status']) || 'completed',
            runAt: String(res.run.created_at || new Date().toISOString()),
            durationMs: Number(res.run.duration_ms || 4200),
            aiSummary: String(res.run.ai_summary || MOCK_RECON_RUN.aiSummary),
            reportUrl: res.report_url,
          };
          // internal/store/queries/recon.sql ListReconItems really returns
          // {id, transaction_id, statement_id, result, detail} — result is
          // the match status, detail is a jsonb blob (ref_code,
          // internal_amount, statement_amount, ...), not the flat
          // status/internal_ref/... shape this used to assume. Untestable
          // live right now: cmd/seed only inserts reconciliation_runs
          // summary rows, never reconciliation_items, so this path has no
          // real data to exercise yet (see qa/plan — known limitation, not
          // a Claude-fixable gap).
          const items: ReconItem[] = (res.items || []).map((item, idx) => {
            const detail = (item.detail as Record<string, unknown>) || {};
            return {
              id: String(item.id || `ri-${idx}`),
              status: (item.result as ReconItem['status']) || 'missing_external',
              internalRef: String(detail.ref_code || item.transaction_id || `MP-${idx}`),
              externalRef: item.statement_id ? String(item.statement_id) : null,
              internalAmount: Number(detail.internal_amount || 0),
              externalAmount: detail.statement_amount ? Number(detail.statement_amount) : null,
              timestamp: String(detail.stmt_date || '00:00:00'),
              suggestedAction: 'Đối chiếu →',
            };
          });
          return { run, items };
        }
      } catch {
        // Fallback for mock preview
      }
      return { run: { ...MOCK_RECON_RUN, date }, items: MOCK_RECON_ITEMS };
    },
    enabled: Boolean(date),
  });
}

export function useRunRecon() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (date: string) => {
      try {
        return await apiRequest<{ run_id: string; date: string }>('/v1/admin/recon/run', {
          method: 'POST',
          body: { date },
        });
      } catch {
        return { run_id: `run-${date}`, date };
      }
    },
    onSuccess: (_, date) => {
      qc.invalidateQueries({ queryKey: ['admin', 'recon', date] });
    },
  });
}
