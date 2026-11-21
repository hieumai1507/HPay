import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { type AdminQueryParams, type Paged, buildQueryString } from './common';

export interface AuthMatrixRule {
  minAmount: number;
  maxAmount?: number;
  authMethod: 'none' | 'pin' | 'otp' | '2fa';
}

export interface OfferServiceConfig {
  serviceId: string;
  serviceCode: string;
  serviceName: string;
  isEnabled: boolean;
  maxAmountPerTxn: number;
  dailyLimit: number;
  monthlyLimit: number;
  intervalSec: number;
  authMatrix: AuthMatrixRule[];
}

export interface OfferPackage {
  id: string;
  code: string;
  name: string;
  role: 'customer' | 'agent' | 'merchant' | 'biller' | 'operator';
  level: number;
  isActive: boolean;
  updatedBy: string;
  updatedAt: string;
  primaryPocketMax: number;
  maxBankAccounts: number;
  authorizedServicesCount: number;
  services: OfferServiceConfig[];
}

export function useOffers(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'offers', params],
    queryFn: async (): Promise<Paged<OfferPackage>> => {
      const res = await apiRequest<{ offers: OfferPackage[]; total?: number }>(
        `/v1/admin/offers${buildQueryString(params)}`
      );
      const rows = res?.offers ?? [];
      return { rows, total: res?.total ?? rows.length };
    },
  });
}

export function useOfferDetail(id: string) {
  return useQuery({
    queryKey: ['admin', 'offer', id],
    queryFn: async () => {
      const res = await apiRequest<{ offer: OfferPackage; services: OfferServiceConfig[] }>(`/v1/admin/offers/${id}`);
      return {
        ...res.offer,
        services: res.services || [],
      };
    },
    enabled: !!id && id !== 'new',
  });
}

export function useSaveOffer() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (offer: Partial<OfferPackage>) => {
      return await apiRequest<OfferPackage>(`/v1/admin/offers/${offer.id || ''}`, {
        method: offer.id ? 'PUT' : 'POST',
        body: offer,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['admin', 'offers'] });
    },
  });
}
