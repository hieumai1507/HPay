import { useRouter } from 'expo-router';
import React from 'react';
import { StyleSheet, Text, View, Pressable, ScrollView } from 'react-native';

import { useSessionStore } from '@/src/stores/session';
import { Button } from '@/src/ui/Button';
import { color, radius, space } from '@/src/ui/tokens';

export default function KycIntroScreen() {
  const router = useRouter();
  const setKycSkipped = useSessionStore((s) => s.setKycSkipped);

  const handleStart = () => {
    router.push('/(kyc)/capture-id');
  };

  const handleSkip = () => {
    setKycSkipped(true);
    router.replace('/(main)/home');
  };

  return (
    <ScrollView contentContainerStyle={styles.container} keyboardShouldPersistTaps="handled">
      {/* Header bar */}
      <View style={styles.navbar}>
        <Pressable onPress={handleSkip} style={styles.backButton} accessibilityLabel="Back">
          <Text style={styles.backIcon}>←</Text>
        </Pressable>
        <Text style={styles.navTitle}>Identity verification (KYC)</Text>
        <View style={{ width: 40 }} />
      </View>

      {/* Hero Banner Card */}
      <View style={styles.heroCard}>
        <Text style={styles.heroEmoji}>🪪</Text>
        <View style={styles.heroContent}>
          <Text style={styles.heroTitle}>Unlock all features</Text>
          <Text style={styles.heroDesc}>
            Verify once to raise your transaction limit to $400.00/day.
          </Text>
        </View>
      </View>

      {/* Steps List */}
      <View style={styles.stepsContainer}>
        <View style={styles.stepRow}>
          <View style={styles.stepBadge}>
            <Text style={styles.stepBadgeText}>1</Text>
          </View>
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Photograph your ID card</Text>
            <Text style={styles.stepDesc}>Take a sharp photo of the front with all 4 corners visible and no glare.</Text>
          </View>
        </View>

        <View style={styles.stepRow}>
          <View style={styles.stepBadge}>
            <Text style={styles.stepBadgeText}>2</Text>
          </View>
          <View style={styles.stepContent}>
            <Text style={styles.stepTitle}>Take a selfie</Text>
            <Text style={styles.stepDesc}>Keep your face inside the oval frame.</Text>
          </View>
        </View>
      </View>

      {/* Heuristics & Tips list */}
      <View style={styles.tipsContainer}>
        <View style={styles.tipRow}>
          <Text style={styles.tipCheck}>✓</Text>
          <Text style={styles.tipText}>Find a well-lit spot and avoid backlight.</Text>
        </View>
        <View style={styles.tipRow}>
          <Text style={styles.tipCheck}>✓</Text>
          <Text style={styles.tipText}>Do not wear sunglasses or a mask for the selfie.</Text>
        </View>
        <View style={styles.tipRow}>
          <Text style={styles.tipCheck}>✓</Text>
          <Text style={styles.tipText}>Make sure the ID photo has no glare and no cut-off corners.</Text>
        </View>
      </View>

      {/* Footer Buttons */}
      <View style={styles.footer}>
        <Button label="Start verification" onPress={handleStart} style={styles.button} />
        <Button label="Later" variant="ghost" onPress={handleSkip} style={styles.button} />
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flexGrow: 1,
    backgroundColor: color.neutral[50],
    paddingHorizontal: space[24],
    paddingBottom: space[40],
  },
  navbar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: 20,
    marginBottom: space[24],
    height: 40,
  },
  backButton: {
    width: 40,
    height: 40,
    borderRadius: radius.full,
    backgroundColor: '#FFFFFF',
    borderWidth: 1,
    borderColor: color.neutral[200],
    alignItems: 'center',
    justifyContent: 'center',
  },
  backIcon: {
    fontSize: 20,
    color: color.neutral[900],
    fontWeight: '700',
  },
  navTitle: {
    fontSize: 16,
    fontWeight: '600',
    color: color.neutral[900],
  },
  heroCard: {
    flexDirection: 'row',
    backgroundColor: color.brand[50],
    borderRadius: radius.lg,
    padding: space[20],
    alignItems: 'center',
    gap: space[16],
    marginBottom: space[24],
  },
  heroEmoji: {
    fontSize: 34,
  },
  heroContent: {
    flex: 1,
    gap: space[4],
  },
  heroTitle: {
    fontSize: 16,
    fontWeight: '700',
    color: color.neutral[900],
  },
  heroDesc: {
    fontSize: 12,
    lineHeight: 18,
    color: color.neutral[700],
  },
  stepsContainer: {
    gap: space[16],
    marginBottom: space[32],
  },
  stepRow: {
    flexDirection: 'row',
    backgroundColor: '#FFFFFF',
    borderRadius: radius.md,
    padding: space[16],
    alignItems: 'center',
    gap: space[16],
    borderWidth: 1,
    borderColor: color.neutral[200],
  },
  stepBadge: {
    width: 28,
    height: 28,
    borderRadius: radius.full,
    backgroundColor: color.brand[50],
    alignItems: 'center',
    justifyContent: 'center',
  },
  stepBadgeText: {
    color: color.brand[600],
    fontSize: 14,
    fontWeight: '700',
  },
  stepContent: {
    flex: 1,
    gap: 2,
  },
  stepTitle: {
    fontSize: 14,
    fontWeight: '600',
    color: color.neutral[900],
  },
  stepDesc: {
    fontSize: 12,
    color: color.neutral[500],
    lineHeight: 16,
  },
  tipsContainer: {
    gap: space[12],
    marginBottom: space[40],
  },
  tipRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
  },
  tipCheck: {
    color: color.success[600],
    fontSize: 14,
    fontWeight: '700',
  },
  tipText: {
    fontSize: 13,
    color: color.neutral[700],
  },
  footer: {
    marginTop: 'auto',
    gap: space[12],
  },
  button: {
    width: '100%',
  },
});
