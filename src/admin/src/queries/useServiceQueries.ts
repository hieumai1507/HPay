import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { idempotencyHeaders } from '@/lib/idempotency';
import type { ActionStepConfig, InputFieldRule } from '@/features/services/ServicePipelineModal';
import { type AdminQueryParams, type Paged, buildQueryString, decodeJsonColumn } from './common';

export type { ActionStepConfig, InputFieldRule };
export { decodeJsonColumn };

export interface ServiceItem {
  id: string;
  code: string;
  name: string;
  fullName?: string;
  category: 'transfer' | 'payment' | 'wallet' | 'org';
  transactionType?: string;
  actionPriority: 'before_gl' | 'after_gl';
  status: 'active' | 'maintenance' | 'disabled';
  channels: string[];
  offerCount?: number;
  serviceNote?: string;
  confirmation?: boolean;
  serviceTarget?: string;
  amountFormula?: Record<string, unknown>;
  actions?: ActionStepConfig[];
  inputFields?: InputFieldRule[];
}

/** The row exactly as the API sends it: snake_case, JSON columns as base64 text, nullable text as null. */
export interface ServiceRow {
  id: string;
  code: string;
  name: string;
  full_name?: string | null;
  service_note?: string | null;
  category: ServiceItem['category'];
  transaction_type?: string;
  action_priority: ServiceItem['actionPriority'];
  status: ServiceItem['status'];
  confirmation?: boolean;
  service_target?: string | null;
  channels?: string | string[] | null;
  amount_formula?: string | Record<string, unknown> | null;
  actions?: string | ActionStepConfig[] | null;
  input_fields?: string | InputFieldRule[] | null;
}

export function mapServiceRow(r: ServiceRow): ServiceItem {
  const actions = decodeJsonColumn<ActionStepConfig[]>(r.actions, []);
  const inputFields = decodeJsonColumn<InputFieldRule[]>(r.input_fields, []);
  return {
    id: r.id,
    code: r.code,
    name: r.name,
    fullName: r.full_name ?? undefined,
    category: r.category,
    transactionType: r.transaction_type,
    actionPriority: r.action_priority,
    status: r.status,
    channels: decodeJsonColumn<string[]>(r.channels, []),
    serviceNote: r.service_note ?? undefined,
    confirmation: r.confirmation,
    serviceTarget: r.service_target ?? undefined,
    amountFormula: decodeJsonColumn<Record<string, unknown>>(r.amount_formula, {}),
    actions: Array.isArray(actions) ? actions : [],
    inputFields: Array.isArray(inputFields) ? inputFields : [],
  };
}

export function useServices(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'services', params],
    queryFn: async (): Promise<Paged<ServiceItem>> => {
      const res = await apiRequest<{ services: ServiceRow[] | null; total?: number }>(
        `/v1/admin/services${buildQueryString(params)}`
      );
      const rows = (res?.services ?? []).map(mapServiceRow);
      return { rows, total: res?.total ?? rows.length };
    },
  });
}

/** PUT /v1/admin/services/{id} body {status: active|maintenance|disabled} -> the updated service. */
export function useUpdateServiceStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: ServiceItem['status'] }) =>
      apiRequest<ServiceRow>(`/v1/admin/services/${id}`, {
        method: 'PUT',
        body: { status },
        headers: idempotencyHeaders(),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'services'] });
    },
  });
}

// Backend gap: there is no POST /v1/admin/services route (docs/reference/api.md).
// The hook that called it was removed; ServiceCatalogPage shows the "create"
// action as disabled with a visible reason until the backend adds the route.
