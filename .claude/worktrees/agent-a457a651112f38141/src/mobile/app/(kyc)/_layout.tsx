import { Stack } from 'expo-router';
import { useEffect } from 'react';

import { useKycDraftStore } from '@/src/features/kyc/stores/kyc-draft';

export default function KycLayout() {
  const loadDraft = useKycDraftStore((s) => s.loadDraft);

  // M11: captures are persisted to SecureStore as they are taken, but nothing
  // ever read them back, so a flow interrupted by a crash, a killed app or a
  // failed upload started from scratch. Restoring here — once, at the entry of
  // the flow — lets both capture screens prefill from the draft they already
  // know how to consume. clearDraft() still runs after a successful submit.
  useEffect(() => {
    void loadDraft();
  }, [loadDraft]);

  return <Stack screenOptions={{ headerShown: false }} />;
}
