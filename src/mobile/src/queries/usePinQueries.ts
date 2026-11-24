import { useMutation } from '@tanstack/react-query';

import { apiRequest } from '../api/client';

/** PUT /v1/me/pin: set the first PIN. Needs the account password (step-up). */
export function useSetPin() {
  return useMutation({
    mutationFn: (input: { password: string; newPin: string }) =>
      apiRequest<{ status: 'pin_set' }>('/v1/me/pin', {
        method: 'PUT',
        body: { password: input.password, new_pin: input.newPin },
      }),
  });
}

export interface PinResetRequestResult {
  status: 'code_sent';
  expires_in: number;
  /** Only present when the API runs in dev; never display it otherwise. */
  otp_dev?: string;
}

/** POST /v1/me/pin/reset/request: send a one-time code after checking the password. */
export function useRequestPinReset() {
  return useMutation({
    mutationFn: (input: { password: string }) =>
      apiRequest<PinResetRequestResult>('/v1/me/pin/reset/request', {
        method: 'POST',
        body: { password: input.password },
      }),
  });
}

/** POST /v1/me/pin/reset/confirm: finish the reset with the code and a new PIN. */
export function useConfirmPinReset() {
  return useMutation({
    mutationFn: (input: { code: string; newPin: string }) =>
      apiRequest<{ status: 'pin_reset' }>('/v1/me/pin/reset/confirm', {
        method: 'POST',
        body: { code: input.code, new_pin: input.newPin },
      }),
  });
}