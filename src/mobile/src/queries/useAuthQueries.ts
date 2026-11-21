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

export interface VerifyOtpPayload {
  otpId?: string;
  phone?: string;
  code: string;
}

export interface VerifyOtpResponse {
  session_token: string;
  user_id: string;
  phone: string;
  role?: string;
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

export function useVerifyOtp() {
  return useMutation({
    mutationFn: (payload: VerifyOtpPayload) =>
      apiRequest<VerifyOtpResponse>('/v1/auth/verify-otp', {
        method: 'POST',
        body: {
          otp_id: payload.otpId,
          phone: payload.phone,
          code: payload.code,
        },
      }),
  });
}
