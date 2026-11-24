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

/** The rows exactly as the API sends them: snake_case, nullable fields, no matrix on the list. */
interface OfferRow {
  id: string;
  code: string;
  name: string;
  role: OfferPackage['role'];
  level: number;
  is_active: boolean;
  primary_pocket_max: number;
  max_bank_accounts: number;
  updated_by?: string | null;
  updated_at?: string | null;
  authorized_services_count?: number | null;
}

interface OfferServiceRow {
  service_id: string;
  service_code: string;
  service_name: string;
  is_enabled: boolean;
  max_amount_per_txn: number;
  daily_limit: number;
  monthly_limit: number;
  interval_seconds: number;
  auth_matrix?: { min_amount: number; max_amount?: number | null; auth_method: AuthMatrixRule['authMethod'] }[] | null;
}

export function mapOfferRow(r: OfferRow): OfferPackage {
  return {
    id: r.id,
    code: r.code,
    name: r.name,
    role: r.role,
    level: r.level,
    isActive: r.is_active,
    updatedBy: r.updated_by ?? '',
    updatedAt: r.updated_at ?? '',
    primaryPocketMax: r.primary_pocket_max,
    maxBankAccounts: r.max_bank_accounts,
    authorizedServicesCount: r.authorized_services_count ?? 0,
    services: [],
  };
}

export function mapOfferService(s: OfferServiceRow): OfferServiceConfig {
  return {
    serviceId: s.service_id,
    serviceCode: s.service_code,
    serviceName: s.service_name,
    isEnabled: s.is_enabled,
    maxAmountPerTxn: s.max_amount_per_txn,
    dailyLimit: s.daily_limit,
    monthlyLimit: s.monthly_limit,
    intervalSec: s.interval_seconds,
    authMatrix: (s.auth_matrix ?? []).map((m) => ({
      minAmount: m.min_amount,
      ...(m.max_amount === null || m.max_amount === undefined ? {} : { maxAmount: m.max_amount }),
      authMethod: m.auth_method,
    })),
  };
}

export function useOffers(params?: AdminQueryParams) {
  return useQuery({
    queryKey: ['admin', 'offers', params],
    queryFn: async (): Promise<Paged<OfferPackage>> => {
      const res = await apiRequest<{ offers: OfferRow[] | null; total?: number }>(
        `/v1/admin/offers${buildQueryString(params)}`
      );
      const rows = (res?.offers ?? []).map(mapOfferRow);
      return { rows, total: res?.total ?? rows.length };
    },
  });
}

export function useOfferDetail(id: string) {
  return useQuery({
    queryKey: ['admin', 'offer', id],
    queryFn: async (): Promise<OfferPackage> => {
      const res = await apiRequest<{ offer: OfferRow; services: OfferServiceRow[] | null }>(`/v1/admin/offers/${id}`);
      return { ...mapOfferRow(res.offer), services: (res.services ?? []).map(mapOfferService) };
    },
    enabled: !!id && id !== 'new',
  });
}
// Backend gap: only GET /v1/admin/offers and GET /v1/admin/offers/{id} exist.
// useSaveOffer (PUT/POST /v1/admin/offers/{id}) was removed because those
// routes return 404/405; OfferDetailPage disables "Save" with a visible reason.
