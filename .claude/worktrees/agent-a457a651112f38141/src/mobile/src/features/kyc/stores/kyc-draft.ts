import * as SecureStore from 'expo-secure-store';
import { create } from 'zustand';

const DRAFT_ID_CARD_KEY = 'mp_kyc_draft_id_card';
const DRAFT_SELFIE_KEY = 'mp_kyc_draft_selfie';

interface KycDraftState {
  idCardUri: string | null;
  selfieUri: string | null;
  loading: boolean;
  setIdCard: (uri: string | null) => void;
  setSelfie: (uri: string | null) => void;
  saveDraft: () => Promise<void>;
  loadDraft: () => Promise<void>;
  clearDraft: () => Promise<void>;
}

export const useKycDraftStore = create<KycDraftState>((set, get) => ({
  idCardUri: null,
  selfieUri: null,
  loading: true,
  setIdCard: (uri) => set({ idCardUri: uri }),
  setSelfie: (uri) => set({ selfieUri: uri }),
  saveDraft: async () => {
    const { idCardUri, selfieUri } = get();
    try {
      if (idCardUri) {
        await SecureStore.setItemAsync(DRAFT_ID_CARD_KEY, idCardUri);
      } else {
        await SecureStore.deleteItemAsync(DRAFT_ID_CARD_KEY);
      }
      if (selfieUri) {
        await SecureStore.setItemAsync(DRAFT_SELFIE_KEY, selfieUri);
      } else {
        await SecureStore.deleteItemAsync(DRAFT_SELFIE_KEY);
      }
    } catch {
      // no-op
    }
  },
  loadDraft: async () => {
    set({ loading: true });
    try {
      const idCardUri = await SecureStore.getItemAsync(DRAFT_ID_CARD_KEY);
      const selfieUri = await SecureStore.getItemAsync(DRAFT_SELFIE_KEY);
      set({ idCardUri, selfieUri, loading: false });
    } catch {
      set({ loading: false });
    }
  },
  clearDraft: async () => {
    try {
      await SecureStore.deleteItemAsync(DRAFT_ID_CARD_KEY);
      await SecureStore.deleteItemAsync(DRAFT_SELFIE_KEY);
    } catch {
      // no-op
    }
    set({ idCardUri: null, selfieUri: null });
  },
}));
