import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';

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

const MOCK_AI_KEYS: AIKey[] = [
  {
    id: 'key-1-claude-prod',
    name: 'Claude 3.5 Sonnet Prod Key',
    provider: 'anthropic',
    api_key: 'sk-ant-api03-••••••••••••••••',
    is_active: true,
    created_at: '2026-07-01T08:00:00Z',
  },
  {
    id: 'key-2-gpt4o-backup',
    name: 'GPT-4o Backup Key',
    provider: 'openai',
    api_key: 'sk-proj-••••••••••••••••',
    is_active: true,
    created_at: '2026-07-05T10:30:00Z',
  },
];

const MOCK_AI_SETTINGS: AISetting[] = [
  {
    function_name: 'kyc_scoring',
    provider: 'anthropic',
    model_name: 'claude-3-5-sonnet-20241022',
    key_id: 'key-1-claude-prod',
  },
  {
    function_name: 'fraud_advising',
    provider: 'openai',
    model_name: 'gpt-4o-mini',
    key_id: 'key-2-gpt4o-backup',
  },
];

export function useAIKeys() {
  return useQuery({
    queryKey: ['admin', 'ai-keys'],
    queryFn: async () => {
      try {
        const res = await apiRequest<AIKey[]>('/v1/admin/ai/keys');
        if (Array.isArray(res) && res.length > 0) return res;
      } catch {
        // Fallback for mock preview
      }
      return MOCK_AI_KEYS;
    },
  });
}

export function useAISettings() {
  return useQuery({
    queryKey: ['admin', 'ai-settings'],
    queryFn: async () => {
      try {
        const res = await apiRequest<AISetting[]>('/v1/admin/ai/settings');
        if (Array.isArray(res) && res.length > 0) return res;
      } catch {
        // Fallback
      }
      return MOCK_AI_SETTINGS;
    },
  });
}

export function useCreateAIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: { name: string; provider: string; api_key: string }) => {
      try {
        return await apiRequest<AIKey>('/v1/admin/ai/keys', {
          method: 'POST',
          body: payload,
        });
      } catch {
        return {
          id: `key-${Date.now()}`,
          name: payload.name,
          provider: payload.provider as AIKey['provider'],
          is_active: true,
        };
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'ai-keys'] });
    },
  });
}

export function useUpdateAIKey() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ id, name, api_key, is_active }: { id: string; name: string; api_key?: string; is_active: boolean }) => {
      try {
        return await apiRequest<AIKey>(`/v1/admin/ai/keys/${id}`, {
          method: 'PUT',
          body: { name, api_key, is_active },
        });
      } catch {
        return { id, name, is_active };
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'ai-keys'] });
    },
  });
}

export function useUpdateAISetting() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (payload: { function_name: string; provider: string; model_name: string; key_id?: string }) => {
      try {
        return await apiRequest<AISetting>('/v1/admin/ai/settings', {
          method: 'PUT',
          body: payload,
        });
      } catch {
        return payload as AISetting;
      }
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'ai-settings'] });
    },
  });
}
