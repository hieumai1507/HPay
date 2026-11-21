import { Ionicons } from '@expo/vector-icons';
import { useEffect } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withRepeat, withTiming } from 'react-native-reanimated';

import { color, space, type } from './tokens';

export type TimelineStepState = 'done' | 'active' | 'failed' | 'waiting';

interface PulsingRingProps {
  ringColor: string;
}

function PulsingRing({ ringColor }: PulsingRingProps) {
  const scale = useSharedValue(1);
  const opacity = useSharedValue(0.6);

  useEffect(() => {
    scale.value = withRepeat(withTiming(1.6, { duration: 1200 }), -1, false);
    opacity.value = withRepeat(withTiming(0, { duration: 1200 }), -1, false);
  }, [scale, opacity]);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
    opacity: opacity.value,
  }));

  return <Animated.View style={[styles.pulseRing, { backgroundColor: ringColor }, animatedStyle]} />;
}

export interface TimelineStepProps {
  label: string;
  state: TimelineStepState;
  /** Hide the connector after the last step. */
  isLast?: boolean;
  meta?: string;
}

/** One row of the vertical workflow trail (txn detail public view + transfer processing). */
export function TimelineStep({ label, state, isLast, meta }: TimelineStepProps) {
  let stepColor: string = color.neutral[300];
  let isDone = false;
  let isFailed = false;
  let isActive = false;

  if (state === 'done') {
    stepColor = color.success[600];
    isDone = true;
  } else if (state === 'failed') {
    stepColor = color.danger[600];
    isFailed = true;
  } else if (state === 'active') {
    stepColor = color.brand[600];
    isActive = true;
  }

  return (
    <View style={styles.row}>
      <View style={styles.rail}>
        <View style={styles.dotContainer}>
          {isActive && <PulsingRing ringColor={stepColor} />}
          <View style={[styles.dot, { backgroundColor: stepColor }]}>
            {isDone && <Ionicons name="checkmark" size={13} color="#fff" />}
            {isFailed && <Ionicons name="close" size={13} color="#fff" />}
            {isActive && <View style={styles.activeCore} />}
          </View>
        </View>
        {!isLast && (
          <View style={[styles.connector, isDone && { backgroundColor: color.success[600] }]} />
        )}
      </View>
      <View style={styles.content}>
        <Text style={[styles.label, state === 'waiting' && styles.labelWaiting]}>{label}</Text>
        {meta ? <Text style={styles.meta}>{meta}</Text> : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', gap: space[12] },
  rail: { alignItems: 'center', width: 24 },
  dotContainer: {
    width: 24,
    height: 24,
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
  },
  dot: {
    width: 20,
    height: 20,
    borderRadius: 10,
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 2,
  },
  activeCore: {
    width: 8,
    height: 8,
    borderRadius: 4,
    backgroundColor: '#fff',
  },
  pulseRing: {
    position: 'absolute',
    width: 20,
    height: 20,
    borderRadius: 10,
    zIndex: 1,
  },
  connector: { flex: 1, width: 2, backgroundColor: color.neutral[200], minHeight: 24 },
  content: { flex: 1, paddingBottom: space[20] },
  label: { fontSize: type.body.fontSize, fontWeight: '600', color: color.neutral[900] },
  labelWaiting: { color: color.neutral[400], fontWeight: '400' },
  meta: { fontSize: type.caption.fontSize, color: color.neutral[500], marginTop: 2 },
});

