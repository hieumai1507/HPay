import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { MOCKS_ENABLED } from '@/lib/env';
import { idempotencyHeaders } from '@/lib/idempotency';

export interface AIKey {
  id: string;
  name: string;
  provider: 'anthropic' | 'openai';
  api_key?: string;
  is_active: boolean;
  created_at?: string;
}

export interface AISetting {
  function_name: 'kyc_scoring' | 'fraud_advising';
  provider: 'anthropic' | 'openai' | 'mock';
  model_name: string;
  key_id?: string;
}

// Reads: an empty list is a valid answer and is returned as-is. Mock data is
// only reachable in a Vite dev server with VITE_USE_MOCKS=true; every other
// failure propagates so the page can show the real error.
export function useAIKeys() {
  return useQuery({
    queryKey: ['admin', 'ai-keys'],
    queryFn: async (): Promise<AIKey[]> => {
      try {
        return (await apiRequest<AIKey[] | null>('/v1/admin/ai/keys')) ?? [];
      } catch (err) {
        if (import.meta.env.DEV && MOCKS_ENABLED) return (await import('@/mocks/ai')).MOCK_AI_KEYS;
        throw err;
      }
    },
  });
}

export function useAISettings() {
  return useQuery({
    queryKey: ['admin', 'ai-settings'],
    queryFn: async (): Promise<AISetting[]> => {
      try {
        return (await apiRequest<AISetting[] | null>('/v1/admin/ai/settings')) ?? [];
      } catch (err) {
        if (import.meta.env.DEV && MOCKS_ENABLED) return (await import('@/mocks/ai')).MOCK_AI_SETTINGS;
        throw err;
      }
    },
  });
}

// Writes never fabricate a result: a failed call rejects, so the page's own
// catch shows the error instead of "saved".
export function useCreateAIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: { name: string; provider: string; api_key: string }) =>
      apiRequest<AIKey>('/v1/admin/ai/keys', { method: 'POST', body: payload, headers: idempotencyHeaders() }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'ai-keys'] });
    },
  });
}

export function useUpdateAIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, name, api_key, is_active }: { id: string; name: string; api_key?: string; is_active: boolean }) =>
      apiRequest<AIKey>(`/v1/admin/ai/keys/${id}`, {
        method: 'PUT',
        body: { name, api_key, is_active },
        headers: idempotencyHeaders(),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'ai-keys'] });
    },
  });
}

export function useUpdateAISetting() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (payload: { function_name: string; provider: string; model_name: string; key_id?: string }) =>
      apiRequest<AISetting>('/v1/admin/ai/settings', { method: 'PUT', body: payload, headers: idempotencyHeaders() }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'ai-settings'] });
    },
  });
}
