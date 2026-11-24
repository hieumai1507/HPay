import { describe, expect, it } from 'vitest';

import { mapOfferRow, mapOfferService } from './useOffersQueries';

// AUD-001 / AUD-002: the pages read camelCase fields but the API sends snake_case, so every limit
// showed "undefined" and every plan showed "Draft". These pin the mapping.
describe('mapOfferRow', () => {
  it('maps the list row, keeping is_active and the service count', () => {
    const o = mapOfferRow({
      id: 'b0',
      code: 'OFFER_CUST_L1',
      name: 'Personal (KYC tier 1)',
      role: 'customer',
      level: 1,
      is_active: true,
      primary_pocket_max: 250_000_000,
      max_bank_accounts: 2,
      updated_by: 'policy-2026-11-21',
      updated_at: '2026-11-24T12:31:08Z',
      authorized_services_count: 5,
    });
    expect(o.isActive).toBe(true);
    expect(o.authorizedServicesCount).toBe(5);
    expect(o.updatedBy).toBe('policy-2026-11-21');
    expect(o.primaryPocketMax).toBe(250_000_000);
  });

  it('never returns undefined or null for optional fields', () => {
    const o = mapOfferRow({ id: 'x', code: 'C', name: 'N', role: 'customer', level: 0, is_active: false, primary_pocket_max: 0, max_bank_accounts: 1 });
    expect(o.updatedBy).toBe('');
    expect(o.authorizedServicesCount).toBe(0);
    expect(o.isActive).toBe(false);
    expect(Object.values(o).some((v) => v === undefined || v === null)).toBe(false);
  });
});

describe('mapOfferService', () => {
  const row = {
    service_id: 's1',
    service_code: 'P2P_TRANSFER',
    service_name: 'P2P transfer',
    is_enabled: true,
    max_amount_per_txn: 250_000,
    daily_limit: 125_000_000,
    monthly_limit: 625_000_000,
    interval_seconds: 0,
  };

  it('maps limits and the authentication tiers, in order, with a null upper bound meaning none', () => {
    const s = mapOfferService({
      ...row,
      auth_matrix: [
        { min_amount: 0, max_amount: 8_000, auth_method: 'pin' },
        { min_amount: 8_000, max_amount: 200_000, auth_method: 'otp' },
        { min_amount: 200_000, max_amount: null, auth_method: '2fa' },
      ],
    });
    expect(s.serviceCode).toBe('P2P_TRANSFER');
    expect(s.dailyLimit).toBe(125_000_000);
    expect(s.authMatrix.map((m) => m.authMethod)).toEqual(['pin', 'otp', '2fa']);
    expect(s.authMatrix[0]!.maxAmount).toBe(8_000);
    expect(s.authMatrix[2]!.maxAmount).toBeUndefined();
  });

  it('survives a missing matrix', () => {
    expect(mapOfferService(row).authMatrix).toEqual([]);
    expect(mapOfferService({ ...row, auth_matrix: null }).authMatrix).toEqual([]);
  });
});