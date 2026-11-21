import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { idempotencyHeaders } from '@/lib/idempotency';
import type { ActionStepConfig, InputFieldRule } from '@/features/services/ServicePipelineModal';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export type { ActionStepConfig, InputFieldRule };

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

export function useServices(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'services', params],
    queryFn: async (): Promise<Paged<ServiceItem>> => {
      const res = await apiRequest<{ services: ServiceItem[] | null; total?: number }>(
        `/v1/admin/services${buildQueryString(params)}`
      );
      const rows = res?.services ?? [];
      return { rows, total: res?.total ?? rows.length };
    },
  });
}

/** PUT /v1/admin/services/{id} body {status: active|maintenance|disabled} -> the updated service. */
export function useUpdateServiceStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: ServiceItem['status'] }) =>
      apiRequest<ServiceItem>(`/v1/admin/services/${id}`, {
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
