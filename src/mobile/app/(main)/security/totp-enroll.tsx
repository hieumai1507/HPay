import * as Clipboard from 'expo-clipboard';
import { router } from 'expo-router';
import { Linking, Pressable, ScrollView, Switch, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { OTPInput } from '@/src/features/auth/components/OTPInput';
import { SECURITY_ROUTES } from '@/src/features/security/routes';
import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { useTotpEnrol } from '@/src/features/security/useTotpEnrol';
import { Button } from '@/src/ui';

export default function TotpEnrolScreen() {
  // The secret and the recovery codes are on screen.
  useSecureScreen();
  const e = useTotpEnrol();

  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-2 border-b border-slate-100 dark:border-slate-800">
        <Pressable
          onPress={() => router.back()}
          hitSlop={12}
          accessibilityRole="button"
          accessibilityLabel="Back"
          disabled={e.step === 'recovery'}
        >
          <Text className="text-xl font-bold text-slate-700 dark:text-slate-200 w-8">←</Text>
        </Pressable>
        <Text className="text-base font-semibold text-slate-900 dark:text-white">Two-step verification</Text>
        <View className="w-8" />
      </View>

      <ScrollView contentContainerStyle={{ padding: 16, gap: 12 }} keyboardShouldPersistTaps="handled">
        {e.step === 'password' && (
          <>
            <Text className="text-sm text-slate-600 dark:text-slate-300">
              Enter your account password to set up an authenticator app.
            </Text>
            <TextInput
              accessibilityLabel="Account password"
              value={e.password}
              onChangeText={e.setPassword}
              secureTextEntry
              autoCapitalize="none"
              autoCorrect={false}
              textContentType="password"
              className="border border-slate-300 rounded-xl px-3 py-3 text-base text-slate-900 dark:text-white"
            />
            <Button
              label="Continue"
              accessibilityLabel="Continue"
              loading={e.isPending}
              disabled={e.password.length === 0}
              onPress={() => void e.start()}
            />
          </>
        )}

        {e.step === 'code' && (
          <>
            <Text className="text-sm text-slate-600 dark:text-slate-300">
              Add this key to your authenticator app, then enter the 6-digit code it shows.
            </Text>
            <Text
              selectable
              accessibilityLabel="Setup key"
              className="text-lg font-mono font-bold text-slate-900 dark:text-white"
            >
              {e.groupedSecret}
            </Text>
            <Button
              variant="secondary"
              label="Copy key"
              accessibilityLabel="Copy key"
              onPress={() => void Clipboard.setStringAsync(e.secret.replace(/\s+/g, ''))}
            />
            <Pressable
              accessibilityRole="link"
              accessibilityLabel="Open in authenticator app"
              onPress={() => void Linking.openURL(e.uri).catch(() => {})}
            >
              <Text className="text-sm font-semibold text-brand">Open in authenticator app</Text>
            </Pressable>
            <OTPInput value={e.code} onChange={e.setCode} />
            <Button
              label="Turn on"
              accessibilityLabel="Turn on"
              loading={e.isPending}
              disabled={e.code.length !== 6}
              onPress={() => void e.confirm()}
            />
          </>
        )}

        {e.step === 'recovery' && (
          <>
            <Text className="text-base font-bold text-slate-900 dark:text-white">Save your recovery codes</Text>
            <Text className="text-sm text-slate-600 dark:text-slate-300">
              Each code works once if you lose your authenticator. They are shown only now.
            </Text>
            <View className="bg-slate-50 dark:bg-slate-800 rounded-xl p-4 gap-2">
              {e.recoveryCodes.map((c) => (
                <Text key={c} selectable className="text-lg font-mono font-bold text-slate-900 dark:text-white">
                  {c}
                </Text>
              ))}
            </View>
            <Button
              variant="secondary"
              label="Copy all codes"
              accessibilityLabel="Copy all codes"
              onPress={() => void Clipboard.setStringAsync(e.recoveryCodes.join('\n'))}
            />
            <View className="flex-row items-center gap-3">
              <Switch
                accessibilityLabel="I saved these codes"
                value={e.saved}
                onValueChange={e.setSaved}
              />
              <Text className="text-sm text-slate-700 dark:text-slate-200 flex-1">I saved these codes</Text>
            </View>
            <Button
              label="Done"
              accessibilityLabel="Done"
              disabled={!e.canFinish}
              onPress={() => router.back()}
            />
          </>
        )}

        {e.error ? (
          <Text accessibilityRole="alert" className="text-xs color-rose-600 text-center">
            {e.error.message}
          </Text>
        ) : null}
        {e.error?.code === 'ERR_PIN_NOT_SET' ? (
          <Button
            variant="secondary"
            label="Set PIN"
            accessibilityLabel="Set PIN"
            onPress={() => router.push(SECURITY_ROUTES.setPin as never)}
          />
        ) : null}
      </ScrollView>
    </SafeAreaView>
  );
}