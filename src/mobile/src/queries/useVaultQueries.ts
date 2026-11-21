import { useMutation, useQuery } from '@tanstack/react-query';

import { apiRequest } from '../api/client';
import { queryClient } from '../api/query-client';
import type { components } from '../api/schema';

import { useTransactionEngine } from './useTransactionQueries';
import { walletKeys } from './useWalletQueries';

export type Vault = components['schemas']['Vault'];

export const vaultKeys = {
  all: ['vaults'] as const,
};

export function useVaults() {
  return useQuery({
    queryKey: vaultKeys.all,
    queryFn: async () => {
      const res = await apiRequest<{ vaults: Vault[] }>('/v1/vaults');
      return res.vaults ?? [];
    },
  });
}

export function useCreateVault() {
  return useMutation({
    mutationFn: (input: { label: string; targetAmount?: number }) =>
      apiRequest<Vault>('/v1/vaults', {
        method: 'POST',
        body: { label: input.label, target_amount: input.targetAmount ?? 0 },
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: vaultKeys.all });
    },
  });
}

/**
 * Round-up (docs 04 §2.2, stretch): the optional ROUND_UP workflow step
 * sweeps a P2P spend's rounding difference into whichever vault has
 * round_up_target=true. At most one vault per user at a time — enabling one
 * auto-disables whichever vault was previously enabled.
 */
export function useSetRoundUpTarget() {
  return useMutation({
    mutationFn: (input: { vaultId: string; enabled: boolean }) =>
      apiRequest<Vault>(`/v1/vaults/${input.vaultId}/round-up-target`, {
        method: 'PATCH',
        body: { enabled: input.enabled },
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: vaultKeys.all });
    },
  });
}

/**
 * Moving money in and out of a vault goes through the same 3-step engine as
 * every other service (service_code=VAULT_SAVINGS), so vaults inherit the
 * offer limits, the auth matrix and idempotency instead of getting their own
 * money path. There is no /vaults/:id/deposit endpoint by design.
 */
export function useVaultMove() {
  const engine = useTransactionEngine();

  const move = async (
    vaultId: string,
    amount: number,
    direction: 'deposit' | 'withdraw',
    idempotencyKey?: string,
  ) => {
    const req = await engine.requestTransaction({
      serviceCode: 'VAULT_SAVINGS',
      amount,
      idempotencyKey,
      extraParams: { vault_id: vaultId, direction },
    });
    const res = await engine.confirmTransaction({
      transactionId: req.transaction_id,
      pin: '123456',
    });
    // Both the vault list and the main wallet balance changed.
    void queryClient.invalidateQueries({ queryKey: vaultKeys.all });
    void queryClient.invalidateQueries({ queryKey: walletKeys.wallets });
    return res;
  };

  return {
    deposit: (vaultId: string, amount: number, key?: string) =>
      move(vaultId, amount, 'deposit', key),
    withdraw: (vaultId: string, amount: number, key?: string) =>
      move(vaultId, amount, 'withdraw', key),
    isMoving: engine.isRequesting || engine.isConfirming,
  };
}
