import { useMutation, useQuery } from '@tanstack/react-query';

import { apiRequest } from '../api/client';
import { queryClient } from '../api/query-client';

export interface TotpStatus {
  enabled: boolean;
  recovery_codes_left: number;
}

export interface TotpEnrolResult {
  status: string;
  /** Shown once. Keep in component state only. */
  secret: string;
  otpauth_uri: string;
}

export interface TotpConfirmResult {
  status: string;
  /** Shown once. Keep in component state only. */
  recovery_codes: string[];
}

export const totpKeys = { status: ['me', 'totp'] as const };

export function useTotpStatus() {
  return useQuery({
    queryKey: totpKeys.status,
    queryFn: () => apiRequest<TotpStatus>('/v1/me/totp'),
  });
}

/**
 * The enrol and confirm results carry the secret and the recovery codes. gcTime 0 drops the
 * mutation result from the query cache as soon as nothing observes it (C-07, C-11).
 */
export function useTotpEnrolStart() {
  return useMutation({
    gcTime: 0,
    mutationFn: (password: string) =>
      apiRequest<TotpEnrolResult>('/v1/me/totp/enroll', { method: 'POST', body: { password } }),
  });
}

export function useTotpEnrolConfirm() {
  return useMutation({
    gcTime: 0,
    mutationFn: (code: string) =>
      apiRequest<TotpConfirmResult>('/v1/me/totp/confirm', { method: 'POST', body: { code } }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: totpKeys.status });
    },
  });
}