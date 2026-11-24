import { router, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { parseAmountToBigInt } from '@/src/lib/money';
import { AmountDisplay, Button, color, space, type } from '@/src/ui';

/** Topup is synchronous — reaching this screen means the money is already credited. */
export default function TopupResultScreen() {
  const { refCode, amount } = useLocalSearchParams<{ refCode: string; amount: string }>();
  const value = parseAmountToBigInt(amount ?? '') ?? 0n;

  return (
    <SafeAreaView style={styles.safe}>
      <View style={styles.body}>
        <Text style={styles.check}>✓</Text>
        <Text style={styles.title}>Top up successful</Text>
        <AmountDisplay valueMinorUnits={value} sign="in" size="display" />
        <Text style={styles.ref}>Transaction ID: {refCode}</Text>
      </View>
      <View style={styles.footer}>
        <Button label="Back to home" onPress={() => router.dismissTo('/(main)/home')} />
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  safe: { flex: 1, backgroundColor: '#fff' },
  body: { flex: 1, alignItems: 'center', justifyContent: 'center', gap: space[16], padding: space[24] },
  check: {
    fontSize: 40,
    color: '#fff',
    backgroundColor: color.success[600],
    width: 72,
    height: 72,
    borderRadius: 36,
    textAlign: 'center',
    lineHeight: 72,
    overflow: 'hidden',
  },
  title: { fontSize: type.h1.fontSize, fontWeight: '700', color: color.neutral[900] },
  ref: { fontSize: type.caption.fontSize, color: color.neutral[500], fontFamily: 'monospace' },
  footer: { padding: space[16] },
});
