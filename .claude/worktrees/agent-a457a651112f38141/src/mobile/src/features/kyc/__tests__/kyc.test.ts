import * as SecureStore from 'expo-secure-store';

import { validateKycPhoto } from '../helpers/photo-validator';
import { useKycDraftStore } from '../stores/kyc-draft';
import { API_BASE_URL } from '@/src/api/client';

const resetSecureStore = () => (SecureStore as unknown as { __reset: () => void }).__reset();

beforeEach(() => {
  resetSecureStore();
  useKycDraftStore.getState().clearDraft();
  global.fetch = jest.fn();
});

describe('TestEdge_M10_KYCImageHeuristic_Validation', () => {
  it('identifies blurry images and rejects with appropriate Vietnameses error message', async () => {
    const result = await validateKycPhoto('file:///mock-id-card-blurry.jpg');
    expect(result.isValid).toBe(false);
    expect(result.error).toContain('mờ');
  });

  it('identifies dark/poorly-lit images and rejects with appropriate Vietnameses error message', async () => {
    const result = await validateKycPhoto('file:///mock-id-card-dark.jpg');
    expect(result.isValid).toBe(false);
    expect(result.error).toContain('tối');
  });

  it('accepts compliant images', async () => {
    const result = await validateKycPhoto('file:///mock-id-card-front.jpg');
    expect(result.isValid).toBe(true);
    expect(result.error).toBeUndefined();
  });
});

describe('TestEdge_M11_KYCUpload_NetworkDropResume', () => {
  it('saves URIs to draft, fails on connection drop, retains draft URIs, and clears them only after successful retry', async () => {
    // 1. Initial State
    const draftStore = useKycDraftStore.getState();
    expect(draftStore.idCardUri).toBeNull();
    expect(draftStore.selfieUri).toBeNull();

    // 2. Set Draft Photos
    useKycDraftStore.setState({
      idCardUri: 'file:///local-path-to-id-card.jpg',
      selfieUri: 'file:///local-path-to-selfie.jpg',
    });
    
    await useKycDraftStore.getState().saveDraft();

    // Verify it is persisted in SecureStore
    await useKycDraftStore.getState().loadDraft();
    expect(useKycDraftStore.getState().idCardUri).toBe('file:///local-path-to-id-card.jpg');
    expect(useKycDraftStore.getState().selfieUri).toBe('file:///local-path-to-selfie.jpg');

    // 3. Simulate upload attempt failing due to connection drop (fetch throws)
    (global.fetch as jest.Mock).mockRejectedValueOnce(new TypeError('Network request failed'));

    let uploadSuccess = false;
    let errorCaught: any = null;

    const performUpload = async () => {
      const card = useKycDraftStore.getState().idCardUri;
      const self = useKycDraftStore.getState().selfieUri;

      const formData = new FormData();
      formData.append('id_card', { uri: card, name: 'id_card.jpg', type: 'image/jpeg' } as any);
      formData.append('selfie', { uri: self, name: 'selfie.jpg', type: 'image/jpeg' } as any);

      try {
        const response = await fetch(`${API_BASE_URL}/v1/kyc/submit`, {
          method: 'POST',
          body: formData,
        });
        if (response.ok) {
          uploadSuccess = true;
          await useKycDraftStore.getState().clearDraft();
        }
      } catch (err) {
        errorCaught = err;
      }
    };

    await performUpload();

    // Verify it failed but draft URIs are still present
    expect(uploadSuccess).toBe(false);
    expect(errorCaught).toBeDefined();
    expect(useKycDraftStore.getState().idCardUri).toBe('file:///local-path-to-id-card.jpg');
    expect(useKycDraftStore.getState().selfieUri).toBe('file:///local-path-to-selfie.jpg');

    // 4. Connection is restored, simulate retry success
    (global.fetch as jest.Mock).mockResolvedValueOnce({
      ok: true,
      status: 202,
      json: async () => ({ status: 'accepted' }),
    });

    await performUpload();

    // Verify upload succeeded and drafts are cleared
    expect(uploadSuccess).toBe(true);
    expect(useKycDraftStore.getState().idCardUri).toBeNull();
    expect(useKycDraftStore.getState().selfieUri).toBeNull();
  });
});
