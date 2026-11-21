import { useQuery } from '@tanstack/react-query';
import { apiRequest } from '../api/client';

export interface EngineStatusResponse {
  status: string;
  version: string;
  uptime_seconds: number;
}

export function useEngineStatus() {
  return useQuery({
    queryKey: ['engine', 'status'],
    queryFn: () => apiRequest<EngineStatusResponse>('/v1/engine/status'),
    staleTime: 30_000,
  });
}
