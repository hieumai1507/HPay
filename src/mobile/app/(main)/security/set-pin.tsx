import { ScrollView, Text, TextInput, View } from 'react-native';
import { useRouter } from 'expo-router';

import { NewPinFields } from '@/src/features/security/components/NewPinFields';
import { useSetPinForm } from '@/src/features/security/useSetPinForm';
import { Button } from '@/src/ui';

export default function SetPinScreen() {
  const router = useRouter();
  const f = useSetPinForm();

  if (f.done) {
    return (
      <View className="flex-1 bg-slate-50 p-5 justify-center gap-4">
        <Text className="text-xl font-bold text-slate-900">PIN set</Text>
        <Text className="text-sm text-slate-600">You can now confirm payments with your PIN.</Text>
        <Button label="Done" accessibilityLabel="Done" onPress={() => router.back()} />
      </View>
    );
  }

  return (
    <ScrollView className="flex-1 bg-slate-50" contentContainerStyle={{ padding: 20, gap: 16, paddingTop: 56 }} keyboardShouldPersistTaps="handled">
      <Text className="text-xl font-bold text-slate-900">Set your PIN</Text>
      <Text className="text-sm text-slate-600">
        You will use this 6-digit PIN to confirm payments. Enter your password to continue.
      </Text>
      <Text className="text-sm font-semibold text-slate-700">Password</Text>
      <TextInput
        accessibilityLabel="Password"
        secureTextEntry
        autoCapitalize="none"
        autoCorrect={false}
        value={f.password}
        onChangeText={f.setPassword}
        className="h-12 rounded-xl border border-slate-300 bg-white px-3 text-base"
      />
      <NewPinFields
        pin={f.pin} onPin={f.setPin} repeat={f.repeat} onRepeat={f.setRepeat}
        hint={f.pinHint} mismatch={f.mismatch}
      />
      {f.error ? <Text accessibilityRole="alert" className="text-sm text-rose-600">{f.error}</Text> : null}
      <Button label="Set PIN" accessibilityLabel="Set PIN" loading={f.isPending} disabled={!f.canSubmit} onPress={f.submit} />
      <Button variant="secondary" label="Cancel" accessibilityLabel="Cancel" onPress={() => router.back()} />
    </ScrollView>
  );
}