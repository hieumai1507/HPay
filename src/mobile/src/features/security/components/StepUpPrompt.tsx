import { useEffect, useState } from 'react';
import { Modal, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { OTPInput } from '@/src/features/auth/components/OTPInput';
import { color, radius, space } from '@/src/ui/tokens';

export type StepUpBand = 'otp' | '2fa';

export interface StepUpPromptProps {
  visible: boolean;
  band: StepUpBand;
  /** Shown only when the API returned it (dev). */
  otpDev?: string | null;
  /** Seconds until a new code may be requested; 0 means the resend button is enabled. */
  cooldown: number;
  resending: boolean;
  /** Message from the previous attempt or a failed resend (server copy via describeError). */
  notice?: string | null;
  onResend: () => void;
  onSubmit: (factors: { pin: string; code: string }) => void;
  onCancel: () => void;
}

const RECOVERY_MAX = 32;

/**
 * PIN plus a second factor in one modal: the SMS code (otp band) or the authenticator code
 * (2fa band, with a recovery-code toggle). It only collects text; the server verifies it.
 */
export function StepUpPrompt(props: StepUpPromptProps) {
  const { visible, band, otpDev, cooldown, resending, notice, onResend, onSubmit, onCancel } = props;
  const [pin, setPin] = useState('');
  const [code, setCode] = useState('');
  const [recovery, setRecovery] = useState(false);

  useEffect(() => {
    if (!visible) {
      setPin('');
      setCode('');
      setRecovery(false);
    }
  }, [visible]);

  const codeOk = recovery ? code.trim().length >= 8 : code.length === 6;
  const complete = pin.length === 6 && codeOk;

  return (
    <Modal visible={visible} transparent animationType="fade" onRequestClose={onCancel}>
      <View style={styles.backdrop}>
        <View style={styles.card}>
          <Text style={styles.title}>Confirm with your PIN and a code</Text>
          {notice ? (
            <Text accessibilityRole="alert" style={styles.error}>
              {notice}
            </Text>
          ) : null}

          <Text style={styles.label}>PIN</Text>
          <OTPInput value={pin} onChange={setPin} secureTextEntry />

          {band === 'otp' ? (
            <>
              <Text style={styles.label}>Code sent to your phone</Text>
              <OTPInput value={code} onChange={setCode} />
              {otpDev ? <Text style={styles.dev}>Dev code: {otpDev}</Text> : null}
              <Pressable
                accessibilityRole="button"
                accessibilityLabel="resend-code"
                accessibilityState={{ disabled: cooldown > 0 || resending }}
                disabled={cooldown > 0 || resending}
                onPress={onResend}
                style={styles.link}
              >
                <Text style={[styles.linkText, (cooldown > 0 || resending) && styles.muted]}>
                  {cooldown > 0 ? `Resend code in ${cooldown}s` : 'Resend code'}
                </Text>
              </Pressable>
            </>
          ) : (
            <>
              <Text style={styles.label}>{recovery ? 'Recovery code' : 'Authenticator code'}</Text>
              {recovery ? (
                <TextInput
                  accessibilityLabel="recovery-code-input"
                  value={code}
                  onChangeText={(t) => setCode(t.trim().slice(0, RECOVERY_MAX))}
                  autoCapitalize="none"
                  autoCorrect={false}
                  autoComplete="off"
                  style={styles.input}
                />
              ) : (
                <OTPInput value={code} onChange={setCode} />
              )}
              <Pressable
                accessibilityRole="button"
                accessibilityLabel="toggle-recovery"
                onPress={() => {
                  setRecovery((r) => !r);
                  setCode('');
                }}
                style={styles.link}
              >
                <Text style={styles.linkText}>
                  {recovery ? 'Use authenticator code' : 'Use a recovery code'}
                </Text>
              </Pressable>
            </>
          )}

          <View style={styles.row}>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="stepup-cancel"
              onPress={onCancel}
              style={[styles.btn, styles.btnSecondary]}
            >
              <Text style={styles.btnSecondaryText}>Cancel</Text>
            </Pressable>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="stepup-confirm"
              accessibilityState={{ disabled: !complete }}
              disabled={!complete}
              onPress={() => onSubmit({ pin, code: recovery ? code.trim() : code })}
              style={[styles.btn, styles.btnPrimary, !complete && styles.btnDisabled]}
            >
              <Text style={styles.btnPrimaryText}>Confirm</Text>
            </Pressable>
          </View>
        </View>
      </View>
    </Modal>
  );
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, backgroundColor: 'rgba(15, 23, 42, 0.5)', justifyContent: 'center', padding: space[24] },
  card: { backgroundColor: '#FFFFFF', borderRadius: radius.md, padding: space[20], gap: space[12] },
  title: { fontSize: 18, fontWeight: '700', color: color.neutral[900], textAlign: 'center' },
  label: { fontSize: 13, fontWeight: '600', color: color.neutral[700] },
  error: { fontSize: 13, color: color.danger[600], textAlign: 'center' },
  dev: { fontSize: 12, color: color.neutral[500], textAlign: 'center' },
  input: {
    borderWidth: 1.5,
    borderColor: color.neutral[300],
    borderRadius: radius.sm,
    paddingHorizontal: space[12],
    paddingVertical: space[12],
    fontSize: 16,
    color: color.neutral[900],
  },
  link: { alignItems: 'center', paddingVertical: space[8], minHeight: 44, justifyContent: 'center' },
  linkText: { color: color.brand[600], fontWeight: '600' },
  muted: { color: color.neutral[400] },
  row: { flexDirection: 'row', gap: space[12] },
  btn: { flex: 1, alignItems: 'center', paddingVertical: space[12], borderRadius: radius.md },
  btnPrimary: { backgroundColor: color.brand[600] },
  btnSecondary: { borderWidth: 1, borderColor: color.neutral[300] },
  btnDisabled: { opacity: 0.4 },
  btnPrimaryText: { color: '#FFFFFF', fontWeight: '700' },
  btnSecondaryText: { color: color.neutral[700], fontWeight: '600' },
});