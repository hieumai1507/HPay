import { useQuery } from '@tanstack/react-query';
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
      const res = await apiRequest<{ offers: OfferPackage[] | null; total?: number }>(
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
      const res = await apiRequest<{ offer: OfferPackage; services: OfferServiceConfig[] | null }>(`/v1/admin/offers/${id}`);
      return {
        ...res.offer,
        services: res.services || [],
      };
    },
    enabled: !!id && id !== 'new',
  });
}

// Backend gap: only GET /v1/admin/offers and GET /v1/admin/offers/{id} exist.
// useSaveOffer (PUT/POST /v1/admin/offers/{id}) was removed because those
// routes return 404/405; OfferDetailPage disables "Save" with a visible reason.
