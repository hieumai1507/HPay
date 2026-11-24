import { useMutation } from '@tanstack/react-query';
import { apiRequest } from '../api/client';

export interface LoginPayload {
  phone: string;
}

export interface LoginResponse {
  step: 'otp_required' | 'logged_in';
  otp_id?: string;
  expires_in?: number;
  session_token?: string;
}

export function useLogin() {
  return useMutation({
    mutationFn: (payload: LoginPayload) =>
      apiRequest<LoginResponse>('/v1/auth/login', {
        method: 'POST',
        body: { phone: payload.phone },
      }),
  });
}
