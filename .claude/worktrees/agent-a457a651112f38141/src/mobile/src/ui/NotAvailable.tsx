import { router } from 'expo-router';
import { Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { NOT_AVAILABLE_BODY, NOT_AVAILABLE_TITLE } from '@/src/lib/availability';

import { Button } from './Button';

/** Full-screen state for flows whose backend route is not mounted yet. */
export function NotAvailable({ title = NOT_AVAILABLE_TITLE, body = NOT_AVAILABLE_BODY }: { title?: string; body?: string }) {
  return (
    <SafeAreaView className="flex-1 bg-white dark:bg-slate-900" edges={['top']}>
      <View className="flex-1 items-center justify-center gap-3 p-8">
        <Text className="text-lg font-bold text-slate-900 dark:text-white text-center">{title}</Text>
        <Text className="text-sm text-slate-500 text-center">{body}</Text>
      </View>
      <View className="p-4">
        <Button label="Quay lại" variant="secondary" onPress={() => router.back()} />
      </View>
    </SafeAreaView>
  );
}