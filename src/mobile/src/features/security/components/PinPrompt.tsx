import { useEffect, useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import { OTPInput } from '@/src/features/auth/components/OTPInput';
import { color, radius, space } from '@/src/ui/tokens';

export const PIN_LENGTH = 6;

interface PinPromptProps {
  visible: boolean;
  onSubmit: (pin: string) => void;
  onCancel: () => void;
}

/** Modal PIN entry. Collects digits only; it never compares them with anything. */
export function PinPrompt({ visible, onSubmit, onCancel }: PinPromptProps) {
  const [pin, setPin] = useState('');

  useEffect(() => {
    if (!visible) setPin('');
  }, [visible]);

  const complete = pin.length === PIN_LENGTH;

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <View style={styles.backdrop}>
        <View style={styles.card}>
          <Text style={styles.title}>Nhập mã PIN</Text>
          <Text style={styles.subtitle}>Nhập mã PIN 6 số để xác nhận giao dịch</Text>
          <OTPInput value={pin} onChange={setPin} secureTextEntry />
          <View style={styles.row}>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="pin-cancel"
              onPress={onCancel}
              style={[styles.btn, styles.btnSecondary]}
            >
              <Text style={styles.btnSecondaryText}>Hủy</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="pin-confirm"
              accessibilityState={{ disabled: !complete }}
              disabled={!complete}
              onPress={() => onSubmit(pin)}
              style={[styles.btn, styles.btnPrimary, !complete && styles.btnDisabled]}
            >
              <Text style={styles.btnPrimaryText}>Xác nhận</Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(15, 23, 42, 0.5)',
    justifyContent: 'center',
    padding: space[24],
  },
  card: {
    backgroundColor: '#FFFFFF',
    borderRadius: radius.md,
    padding: space[20],
    gap: space[16],
  },
  title: { fontSize: 18, fontWeight: '700', color: color.neutral[900], textAlign: 'center' },
  subtitle: { fontSize: 13, color: color.neutral[500], textAlign: 'center' },
  row: { flexDirection: 'row', gap: space[12] },
  btn: { flex: 1, alignItems: 'center', paddingVertical: space[12], borderRadius: radius.md },
  btnPrimary: { backgroundColor: color.brand[600] },
  btnSecondary: { borderWidth: 1, borderColor: color.neutral[300] },
  btnDisabled: { opacity: 0.4 },
  btnPrimaryText: { color: '#FFFFFF', fontWeight: '700' },
  btnSecondaryText: { color: color.neutral[700], fontWeight: '600' },
});