import { router, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { getErrorMessage } from '@/src/lib/errors';
import { parseAmountToBigInt } from '@/src/lib/money';
import { AmountDisplay, Button, color, space, type } from '@/src/ui';

type Variant = 'success' | 'failed' | 'processing' | 'pending_approval';

const VARIANT_UI: Record<Variant, { icon: string; iconBg: string; title: string }> = {
  success: { icon: '✓', iconBg: color.success[600], title: 'Transfer successful' },
  failed: { icon: '✕', iconBg: color.danger[600], title: 'Transaction failed' },
  // M02: timeout is NOT a failure — the workflow may still complete.
  processing: { icon: '⏳', iconBg: color.info[600], title: 'Transaction is being processed' },
  pending_approval: { icon: '🕐', iconBg: color.warning[600], title: 'Awaiting organization Approval' },
};

export default function TransferResultScreen() {
  const params = useLocalSearchParams<{
    refCode: string;
    amount: string;
    variant: Variant;
    failureCode?: string;
  }>();
  const variant: Variant = params.variant ?? 'processing';
  const ui = VARIANT_UI[variant];
  const amount = parseAmountToBigInt(params.amount ?? '') ?? 0n;

  return (
    <SafeAreaView style={styles.safe}>
      <View style={styles.body}>
        <Text style={[styles.icon, { backgroundColor: ui.iconBg }]}>{ui.icon}</Text>
        <Text style={styles.title}>{ui.title}</Text>
        {variant === 'success' && <AmountDisplay valueMinorUnits={amount} sign="out" size="display" />}
        {variant === 'failed' && (
          <Text style={styles.reason}>{getErrorMessage(params.failureCode || 'ERR_TRANSFER_FAILED')}</Text>
        )}
        {variant === 'processing' && (
          <Text style={styles.reason}>
            The result will be updated in History. You will not be charged twice.
          </Text>
        )}
        <Text style={styles.ref}>Transaction ID: {params.refCode}</Text>
      </View>

      <View style={styles.footer}>
        {variant === 'failed' && (
          <Button
            variant="secondary"
            label="Try again"
            // Back to the confirm-adjacent entry point; a NEW flow mints a NEW
            // idempotency key — the failed attempt is never replayed (M03).
            onPress={() => router.replace('/(main)/transfer/receiver')}
          />
        )}
        {(variant === 'processing' || variant === 'pending_approval') && (
          <Button
            variant="secondary"
            label="View in History"
            onPress={() => router.dismissTo('/(main)/history')}
          />
        )}
        <Button label="Back to home" onPress={() => router.dismissTo('/(main)/home')} />
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: '#fff' },
  body: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: space[16], padding: space[24] },
  icon: {
    fontSize: 36,
    color: '#fff',
    width: 72,
    height: 72,
    borderRadius: 36,
    textAlign: 'center',
    lineHeight: 72,
    overflow: 'hidden',
  },
  title: { fontSize: type.h1.fontSize, fontWeight: '700', color: color.neutral[900], textAlign: 'center' },
  reason: { fontSize: type.body.fontSize, color: color.neutral[600], textAlign: 'center' },
  ref: { fontSize: type.caption.fontSize, color: color.neutral[500], fontFamily: 'monospace' },
  footer: { padding: space[16], gap: space[8] },
});
