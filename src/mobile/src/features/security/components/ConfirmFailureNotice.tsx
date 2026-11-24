import { router } from 'expo-router';
import { Text, View } from 'react-native';

import { Button } from '@/src/ui';

import type { ConfirmFailure } from '../confirmErrors';

/** Message plus, when the failure has one, the single button that fixes its cause. */
export function ConfirmFailureNotice({ failure }: { failure: ConfirmFailure | null }) {
  if (!failure || failure.kind === 'cancelled' || failure.kind === 'restart') return null;
  const route = failure.actionRoute;
  return (
    <View className="gap-2" accessibilityRole="alert">
      <Text className="text-xs color-rose-600 text-center">{failure.message}</Text>
      {failure.actionLabel && route ? (
        <Button
          variant="secondary"
          label={failure.actionLabel}
          accessibilityLabel={failure.actionLabel}
          onPress={() => router.push(route as never)}
        />
      ) : null}
    </View>
  );
}