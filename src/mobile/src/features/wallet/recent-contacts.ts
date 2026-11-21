import { create } from 'zustand';

export interface RecentContact {
  phone: string; // full phone — typed by the user themselves on this device
  maskedName: string;
}

/**
 * Session-scoped recent receivers, recorded after a successful transfer.
 *
 * NOTE (plan divergence, documented in m3 plan §10): history's
 * counterparty_phone is server-MASKED, so it cannot prefill a new transfer.
 * A tappable long-lived contact list needs a pay-by-counterparty-token
 * contract — deferred to M4. Until then recents live in memory only.
 */
interface RecentContactsState {
  contacts: RecentContact[];
  add: (c: RecentContact) => void;
}

export const useRecentContacts = create<RecentContactsState>((set) => ({
  contacts: [],
  add: (c) =>
    set((s) => ({
      contacts: [c, ...s.contacts.filter((x) => x.phone !== c.phone)].slice(0, 5),
    })),
}));
