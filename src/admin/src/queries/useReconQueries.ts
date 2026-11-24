import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { ApiError } from '@/api/errors';
import { idempotencyHeaders } from '@/lib/idempotency';
import { toMinorUnits } from '@/lib/money';
import { type AdminQueryParams, buildQueryString } from './common';

/**
 * Every figure is nullable on purpose: a value the backend did not send is
 * `null` and renders as an em dash. Nothing here may be defaulted to a
 * plausible-looking number (the old code fell back to 2173/2170/3 and 4200 ms).
 */
export interface ReconRun {
  id: string;
  date: string;
  totalCount: number | null;
  matchedCount: number | null;
  mismatchedCount: number | null;
  runAt: string | null;
  /** The backend does not report a duration yet; always null for real data. */
  durationMs: number | null;
  aiSummary: string | null;
  reportUrl?: string;
}

export interface ReconItem {
  id: string;
  /** Raw `result` from the backend (matched, missing_external, ...). Unknown values are kept as-is. */
  status: string;
  internalRef: string;
  externalRef: string | null;
  internalAmount: number | null;
  externalAmount: number | null;
  timestamp: string | null;
  suggestedAction?: string;
}

export interface ReconResponse {
  run?: Record<string, unknown> | null;
  items?: Array<Record<string, unknown>> | null;
  report_url?: string;
}

function intOrNull(v: unknown): number | null {
  return typeof v === 'number' && Number.isSafeInteger(v) && v >= 0 ? v : null;
}

function strOrNull(v: unknown): string | null {
  return typeof v === 'string' && v !== '' ? v : null;
}

/**
 * Maps GET /v1/admin/recon/runs/:date. Field names follow
 * store.GetReconRunByDateRow's JSON tags (run_date/total/matched/mismatched)
 * and ListReconItems ({id, transaction_id, statement_id, result, detail}).
 */
export function mapReconResponse(res: ReconResponse, date: string): { run: ReconRun | null; items: ReconItem[] } {
  if (!res?.run) return { run: null, items: [] };
  const r = res.run;
  const run: ReconRun = {
    id: strOrNull(r.id) ?? `run-${date}`,
    date: strOrNull(r.run_date) ?? date,
    totalCount: intOrNull(r.total),
    matchedCount: intOrNull(r.matched),
    mismatchedCount: intOrNull(r.mismatched),
    runAt: strOrNull(r.created_at),
    durationMs: intOrNull(r.duration_ms),
    aiSummary: strOrNull(r.ai_summary),
    reportUrl: res.report_url,
  };
  const items: ReconItem[] = (res.items ?? []).map((item, idx) => {
    const detail = (item.detail as Record<string, unknown> | null) ?? {};
    return {
      id: strOrNull(item.id) ?? `ri-${idx}`,
      status: strOrNull(item.result) ?? 'unknown',
      internalRef: strOrNull(detail.ref_code) ?? strOrNull(item.transaction_id) ?? '—',
      externalRef: strOrNull(item.statement_id),
      internalAmount: toMinorUnits(detail.internal_amount),
      externalAmount: toMinorUnits(detail.statement_amount),
      timestamp: strOrNull(detail.stmt_date),
      suggestedAction: 'Check →',
    };
  });
  return { run, items };
}

export function useReconRun(date: string, params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'recon', date, params],
    queryFn: async () => {
      try {
        const res = await apiRequest<ReconResponse>(`/v1/admin/recon/runs/${date}${buildQueryString(params)}`);
        return mapReconResponse(res, date);
      } catch (err) {
        // "No run for this date yet" is a legitimate empty state, not a failure.
        if (err instanceof ApiError && err.code === 'ERR_RECON_NOT_FOUND') return { run: null, items: [] };
        throw err;
      }
    },
    enabled: Boolean(date),
  });
}

export function useRunRecon() {
  const qc = useQueryClient();
  return useMutation({
    // One Idempotency-Key per click: mutationFn runs once per mutate() call.
    mutationFn: (date: string) =>
      apiRequest<{ run_id: string; date: string }>('/v1/admin/recon/run', {
        method: 'POST',
        body: { date },
        headers: idempotencyHeaders(),
      }),
    onSuccess: (_, date) => {
      qc.invalidateQueries({ queryKey: ['admin', 'recon', date] });
    },
  });
}
