import { useMutation, useQuery } from '@tanstack/react-query';
import { apiRequest } from '../api/client';
import { queryClient } from '../api/query-client';
import type { components } from '../api/schema';

export type KYCStatusResponse = {
  kyc_status: components['schemas']['User']['kyc_status'];
  latest_request?: {
    id: string;
    status: string;
    ai_score?: number;
    ai_reasons?: string;
    created_at: string;
  };
};

export function useKYCStatus() {
  return useQuery({
    queryKey: ['kyc', 'status'],
    queryFn: () => apiRequest<KYCStatusResponse>('/v1/kyc/status'),
  });
}

export function useSubmitKYC() {
  return useMutation({
    mutationFn: (input: { idCardUrl: string; selfieUrl: string; fullName: string; idNumber: string }) =>
      apiRequest<{ status: string }>('/v1/kyc/submit', {
        method: 'POST',
        body: {
          id_card_url: input.idCardUrl,
          selfie_url: input.selfieUrl,
          full_name: input.fullName,
          id_number: input.idNumber,
        },
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['kyc', 'status'] });
      queryClient.invalidateQueries({ queryKey: ['users', 'me'] });
    },
  });
}
