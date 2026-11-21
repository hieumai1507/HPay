/**
 * Unit tests for useOrgStore — business wallet state management.
 */
import { act } from '@testing-library/react-native';
import { useOrgStore, type ActiveOrg } from '@/src/stores/org';

const mockOrg: ActiveOrg = {
  orgId:        'org-uuid-1',
  name:         'Acme Corp',
  role:         'maker',
  checkerLevel: 0,
  walletId:     'wallet-uuid-1',
  balance:      10_000_000,
};

const checkerOrg: ActiveOrg = {
  orgId:        'org-uuid-2',
  name:         'Sandbox Studio',
  role:         'checker',
  checkerLevel: 1,
  walletId:     'wallet-uuid-2',
  balance:      5_000_000,
};

describe('useOrgStore', () => {
  beforeEach(() => {
    act(() => {
      useOrgStore.getState().exitOrg();
    });
  });

  it('starts in personal mode (activeOrg = null)', () => {
    expect(useOrgStore.getState().activeOrg).toBeNull();
  });

  it('switchOrg sets the active org', () => {
    act(() => useOrgStore.getState().switchOrg(mockOrg));
    expect(useOrgStore.getState().activeOrg).toEqual(mockOrg);
  });

  it('exitOrg clears the active org', () => {
    act(() => useOrgStore.getState().switchOrg(mockOrg));
    act(() => useOrgStore.getState().exitOrg());
    expect(useOrgStore.getState().activeOrg).toBeNull();
  });

  it('switchOrg replaces previous active org', () => {
    act(() => useOrgStore.getState().switchOrg(mockOrg));
    act(() => useOrgStore.getState().switchOrg(checkerOrg));
    expect(useOrgStore.getState().activeOrg?.orgId).toBe('org-uuid-2');
    expect(useOrgStore.getState().activeOrg?.role).toBe('checker');
  });

  it('updateBalance updates only the balance of the active org', () => {
    act(() => useOrgStore.getState().switchOrg(mockOrg));
    act(() => useOrgStore.getState().updateBalance(9_000_000));
    expect(useOrgStore.getState().activeOrg?.balance).toBe(9_000_000);
    expect(useOrgStore.getState().activeOrg?.name).toBe('Acme Corp');
  });

  it('updateBalance is a no-op when not in business mode', () => {
    act(() => useOrgStore.getState().updateBalance(999_999));
    expect(useOrgStore.getState().activeOrg).toBeNull();
  });
});
