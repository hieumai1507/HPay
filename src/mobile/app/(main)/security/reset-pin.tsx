import { ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';

import { NewPinFields } from '@/src/features/security/components/NewPinFields';
import { SET_PIN_ROUTE } from '@/src/features/security/pinErrors';
import { useResetPin } from '@/src/features/security/useResetPin';
import { OTPInput } from '@/src/features/auth/components/OTPInput';
import { Button } from '@/src/ui';

export default function ResetPinScreen() {
  const router = useRouter();
  const f = useResetPin();

  if (f.step === 'done') {
    return (
      <View className="flex-1 bg-slate-50 p-5 justify-center gap-4">
        <Text className="text-xl font-bold text-slate-900">PIN reset</Text>
        <Text className="text-sm text-slate-600">
          Your new PIN is active. For 24 hours, PIN payments are limited to $100.00.
        </Text>
        <Button label="Done" accessibilityLabel="Done" onPress={() => router.back()} />
      </View>
    );
  }

  const error = f.error ? (
    <Text accessibilityRole="alert" className="text-sm text-rose-600">{f.error}</Text>
  ) : null;

  if (f.step === 'password') {
    return (
      <ScrollView className="flex-1 bg-slate-50" contentContainerStyle={{ padding: 20, gap: 16, paddingTop: 56 }} keyboardShouldPersistTaps="handled">
        <Text className="text-xl font-bold text-slate-900">Forgot PIN?</Text>
        <Text className="text-sm text-slate-600">Enter your password and we will send you a code.</Text>
        <TextInput
          accessibilityLabel="Password"
          secureTextEntry
          autoCapitalize="none"
          autoCorrect={false}
          value={f.password}
          onChangeText={f.setPassword}
          className="h-12 rounded-xl border border-slate-300 bg-white px-3 text-base"
        />
        {error}
        {f.pinNotSet ? (
          <Button variant="ghost" label="Set PIN" accessibilityLabel="Set PIN" onPress={() => router.replace(SET_PIN_ROUTE)} />
        ) : null}
        {f.cooldown > 0 ? (
          <Text className="text-xs text-slate-500">You can request another code in {f.cooldown}s.</Text>
        ) : null}
        <Button
          label={f.cooldown > 0 ? `Send code (${f.cooldown}s)` : 'Send code'}
          accessibilityLabel="Send code"
          loading={f.isPending}
          disabled={!f.canRequest}
          onPress={f.requestCode}
        />
        <Button variant="secondary" label="Cancel" accessibilityLabel="Cancel" onPress={() => router.back()} />
      </ScrollView>
    );
  }

  return (
    <ScrollView className="flex-1 bg-slate-50" contentContainerStyle={{ padding: 20, gap: 16, paddingTop: 56 }} keyboardShouldPersistTaps="handled">
      <Text className="text-xl font-bold text-slate-900">Enter the code</Text>
      <Text className="text-sm text-slate-600">Enter the 6-digit code we sent you and choose a new PIN.</Text>
      {f.devCode ? <Text className="text-xs text-slate-400">Dev code: {f.devCode}</Text> : null}
      <Text className="text-sm font-semibold text-slate-700">Code</Text>
      <OTPInput value={f.code} onChange={f.setCode} disabled={f.needsNewCode} label="Code digit" />
      <NewPinFields
        pin={f.pin} onPin={f.setPin} repeat={f.repeat} onRepeat={f.setRepeat}
        hint={f.pinHint} mismatch={f.mismatch} disabled={f.needsNewCode}
      />
      {error}
      {f.needsNewCode ? (
        <Button label="Request a new code" accessibilityLabel="Request a new code" onPress={f.startOver} />
      ) : (
        <>
          <Button label="Reset PIN" accessibilityLabel="Reset PIN" loading={f.isPending} disabled={!f.canConfirm} onPress={f.confirm} />
          <Button variant="ghost" label="Request a new code" accessibilityLabel="Request a new code" onPress={f.startOver} />
        </>
      )}
      {f.cooldown > 0 ? <Text className="text-xs text-slate-500">You can request another code in {f.cooldown}s.</Text> : null}
      <Button variant="secondary" label="Cancel" accessibilityLabel="Cancel" onPress={() => router.back()} />
    </ScrollView>
  );
}