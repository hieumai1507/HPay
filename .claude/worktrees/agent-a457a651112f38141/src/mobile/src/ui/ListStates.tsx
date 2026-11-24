import { ActivityIndicator, Text, View } from 'react-native';

import { Button } from './Button';

export function Skeleton({ rows = 5 }: { rows?: number }) {
  return (
    <View className="p-4 gap-4" accessibilityLabel="Đang tải">
      {Array.from({ length: rows }, (_, i) => (
        <View key={i} className="flex-row items-center gap-3">
          <View className="w-10 h-10 rounded-full bg-slate-200 dark:bg-slate-700" />
          <View className="flex-1 gap-2">
            <View className="h-3 rounded-md bg-slate-200 dark:bg-slate-700 w-3/5" />
            <View className="h-3 rounded-md bg-slate-200 dark:bg-slate-700 w-1/3" />
          </View>
        </View>
      ))}
    </View>
  );
}

export function EmptyState({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <View className="items-center justify-center p-8 gap-3">
      <Text className="text-4xl">🗒️</Text>
      <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200 text-center">{title}</Text>
      {subtitle ? <Text className="text-xs text-slate-400 text-center">{subtitle}</Text> : null}
    </View>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <View className="items-center justify-center p-8 gap-3">
      <Text className="text-4xl">⚠️</Text>
      <Text className="text-sm font-semibold text-slate-700 dark:text-slate-200 text-center">{message}</Text>
      {onRetry ? <Button variant="secondary" label="Thử lại" onPress={onRetry} /> : null}
    </View>
  );
}

export function LoadingFooter() {
  return (
    <View className="py-4 items-center">
      <ActivityIndicator color="#0E90A3" />
    </View>
  );
}
