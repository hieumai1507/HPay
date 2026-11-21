import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
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

export function useServicesQuery(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'services', params],
    queryFn: async (): Promise<ServiceItem[]> => {
      try {
        const res = await apiRequest<{ services: ServiceItem[] }>(
          `/v1/admin/services${buildQueryString(params)}`
        );
        return res?.services || [];
      } catch {
        return [];
      }
    },
  });
}

export function useServices(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'services', params],
    queryFn: async (): Promise<Paged<ServiceItem>> => {
      const res = await apiRequest<{ services: ServiceItem[]; total?: number }>(
        `/v1/admin/services${buildQueryString(params)}`
      );
      const rows = res?.services ?? [];
      return { rows, total: res?.total ?? rows.length };
    },
  });
}

export function useUpdateServiceStatusMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: ServiceItem['status'] }) => {
      return await apiRequest(`/v1/admin/services/${id}/status`, {
        method: 'PATCH',
        body: { status },
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'services'] });
    },
  });
}

export function useUpdateServiceStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, status }: { id: string; status: ServiceItem['status'] }) => {
      return await apiRequest<ServiceItem>(`/v1/admin/services/${id}`, {
        method: 'PUT',
        body: { status },
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'services'] });
    },
  });
}

export function useCreateServiceMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: Partial<ServiceItem>) => {
      return await apiRequest('/v1/admin/services', {
        method: 'POST',
        body: payload,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin', 'services'] });
    },
  });
}

export function useCreateService() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: Partial<ServiceItem>) => {
      return await apiRequest<ServiceItem>('/v1/admin/services', {
        method: 'POST',
        body: payload,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'services'] });
    },
  });
}
