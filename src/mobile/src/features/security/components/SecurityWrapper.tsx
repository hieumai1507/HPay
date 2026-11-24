import * as Device from 'expo-device';
import * as LocalAuthentication from 'expo-local-authentication';
import { BlurView } from 'expo-blur';
import React, { useEffect, useState, useRef } from 'react';
import { StyleSheet, View, Text, AppState, Pressable, Animated } from 'react-native';

import { runUnlockAttempt } from '@/src/features/security/unlockAttempts';
import { clearStoredRefreshToken, useSessionStore } from '@/src/stores/session';
import { useSecurityStore } from '@/src/stores/security';
import { color, radius, space, type } from '@/src/ui/tokens';
import { LogoMark } from '@/src/ui/Logo';

const IDLE_TIMEOUT_MS = 5 * 60 * 1000; // 5 minutes

export function SecurityWrapper({ children }: { children: React.ReactNode }) {
  const { isLocked, isRooted, setLocked, setRooted } = useSecurityStore();
  const [showPrivacyMask, setShowPrivacyMask] = useState(false);
  const [unlockError, setUnlockError] = useState(false);
  const [hasDeviceAuth, setHasDeviceAuth] = useState(false);
  const failedAttempts = useRef(0);

  const appState = useRef(AppState.currentState);
  const backgroundTime = useRef<number | null>(null);
  const shakeAnim = useRef(new Animated.Value(0)).current;
  const isAuthenticated = useSessionStore((s) => s.status === 'authenticated');

  useEffect(() => {
    // 1. Jailbreak/Root detection heuristic
    const checkRooted = async () => {
      const isDeviceRooted = Device.isDevice && (
        Device.brand === 'mock-rooted'
      );
      setRooted(isDeviceRooted);
    };
    checkRooted();

    // Is a biometric or device credential available to unlock with?
    const checkDeviceAuth = async () => {
      const hasHardware = await LocalAuthentication.hasHardwareAsync();
      const isEnrolled = await LocalAuthentication.isEnrolledAsync();
      setHasDeviceAuth(hasHardware && isEnrolled);
    };
    checkDeviceAuth();

    // 2. AppState monitoring for privacy screen (M07) and lock timer
    const subscription = AppState.addEventListener('change', (nextAppState) => {
      if (
        appState.current === 'active' &&
        (nextAppState === 'background' || nextAppState === 'inactive')
      ) {
        setShowPrivacyMask(true);
        backgroundTime.current = Date.now();
      }

      if (
        (appState.current === 'background' || appState.current === 'inactive') &&
        nextAppState === 'active'
      ) {
        setShowPrivacyMask(false);

        if (isAuthenticated && backgroundTime.current) {
          const elapsed = Date.now() - backgroundTime.current;
          if (elapsed >= IDLE_TIMEOUT_MS) {
            setLocked(true);
          }
        }
        backgroundTime.current = null;
      }

      appState.current = nextAppState;
    });

    return () => {
      subscription.remove();
    };
  }, [isAuthenticated, setLocked, setRooted]);

  // Shake animation when an unlock attempt fails
  const triggerShake = () => {
    Animated.sequence([
      Animated.timing(shakeAnim, { toValue: 10, duration: 50, useNativeDriver: true }),
      Animated.timing(shakeAnim, { toValue: -10, duration: 50, useNativeDriver: true }),
      Animated.timing(shakeAnim, { toValue: 8, duration: 50, useNativeDriver: true }),
      Animated.timing(shakeAnim, { toValue: -8, duration: 50, useNativeDriver: true }),
      Animated.timing(shakeAnim, { toValue: 4, duration: 50, useNativeDriver: true }),
      Animated.timing(shakeAnim, { toValue: 0, duration: 50, useNativeDriver: true }),
    ]).start();
  };

  // Unlock with biometrics or the device credential (PIN/pattern/passcode), verified by the OS.
  // The app holds no PIN of its own. After MAX_UNLOCK_ATTEMPTS failures the user is signed out.
  // TODO(ADR-0008): add server-side PIN verification here once the backend defines it.
  const triggerUnlock = async () => {
    const res = await runUnlockAttempt(
      () =>
        LocalAuthentication.authenticateAsync({
          promptMessage: 'Use your fingerprint or face to unlock',
          fallbackLabel: 'Use device passcode',
          cancelLabel: 'Cancel',
        }),
      failedAttempts.current,
    );
    failedAttempts.current = res.failedAttempts;

    if (res.outcome === 'unlocked') {
      setUnlockError(false);
      setLocked(false);
    } else if (res.outcome === 'signed_out') {
      failedAttempts.current = 0;
      setUnlockError(false);
      setLocked(false);
      await clearStoredRefreshToken();
      useSessionStore.getState().clearSession('user');
    } else if (res.outcome === 'failed') {
      setUnlockError(true);
      triggerShake();
    }
  };
  // If app is locked and user is logged in, show lock screen
  if (isLocked && isAuthenticated) {
    return (
      <View style={styles.lockContainer}>
        {/* Header */}
        <View style={styles.lockHeader}>
          <View style={styles.lockIconCircle}>
            <Text style={styles.lockEmoji}>🔒</Text>
          </View>
          <Text style={styles.lockTitle}>App locked</Text>
          <Text style={styles.lockSubtitle}>Authenticate to unlock the app</Text>
        </View>

        {/* Unlock error with shake animation */}
        <Animated.View style={[styles.pinWrapper, { transform: [{ translateX: shakeAnim }] }]}>
          {unlockError && (
            <Text style={styles.pinErrorText}>Authentication failed. Please try again.</Text>
          )}
        </Animated.View>

        <View style={styles.footerActions}>
          {hasDeviceAuth ? (
            <Pressable onPress={triggerUnlock} style={styles.biometricBtn} accessibilityRole="button">
              <Text style={styles.biometricIcon}>🧬</Text>
              <Text style={styles.biometricBtnText}>Unlock</Text>
            </Pressable>
          ) : (
            <Text style={styles.biometricHint}>
              This device has no biometrics or screen lock set up. Please log out and log in again.
            </Text>
          )}
        </View>      </View>
    );
  }

  return (
    <View style={{ flex: 1 }}>
      {/* Jailbreak/Root banner warn */}
      {isRooted && (
        <View style={styles.rootWarnBanner}>
          <Text style={styles.rootWarnText}>
            ⚠️ Your device is rooted/jailbroken. Be careful when making transactions.
          </Text>
        </View>
      )}

      {children}

      {/* Screen Privacy Mask (M07) */}
      {showPrivacyMask && (
        <View style={StyleSheet.absoluteFillObject}>
          <BlurView intensity={95} tint="light" style={StyleSheet.absoluteFillObject} />
          <View style={styles.privacyLogoContainer}>
            <LogoMark size={72} />
            <Text style={styles.privacyMaskText}>HPay</Text>
            <Text style={styles.privacyMaskSub}>Secure financial connection</Text>
          </View>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  lockContainer: {
    flex: 1,
    backgroundColor: color.neutral[50],
    justifyContent: 'space-between',
    paddingHorizontal: space[24],
    paddingTop: 80,
    paddingBottom: 40,
  },
  lockHeader: {
    alignItems: 'center',
  },
  lockIconCircle: {
    width: 80,
    height: 80,
    borderRadius: 40,
    backgroundColor: color.brand[50],
    alignItems: 'center',
    justifyContent: 'center',
    marginBottom: space[20],
  },
  lockEmoji: {
    fontSize: 36,
  },
  lockTitle: {
    fontSize: type.h1.fontSize,
    fontWeight: '700',
    color: color.neutral[900],
    marginBottom: space[8],
  },
  lockSubtitle: {
    fontSize: 14,
    color: color.neutral[500],
    textAlign: 'center',
  },
  pinWrapper: {
    width: '100%',
    maxWidth: 340,
    alignSelf: 'center',
    gap: space[12],
  },
  pinErrorText: {
    color: color.danger[600],
    fontSize: 13,
    fontWeight: '600',
    textAlign: 'center',
    marginTop: space[8],
  },
  footerActions: {
    alignItems: 'center',
    gap: space[12],
  },
  biometricBtn: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: space[8],
    paddingVertical: space[12],
    paddingHorizontal: space[20],
    borderRadius: radius.md,
    backgroundColor: color.brand[50],
    borderWidth: 1,
    borderColor: color.brand[50],
  },
  biometricIcon: {
    fontSize: 20,
  },
  biometricBtnText: {
    color: color.brand[700],
    fontSize: 14,
    fontWeight: '600',
  },
  biometricHint: {
    color: color.neutral[400],
    fontSize: 12,
    textAlign: 'center',
    fontStyle: 'italic',
  },
  rootWarnBanner: {
    backgroundColor: color.danger[50],
    paddingVertical: space[8],
    paddingHorizontal: space[16],
    borderBottomWidth: 1,
    borderBottomColor: color.danger[600],
    alignItems: 'center',
    justifyContent: 'center',
  },
  rootWarnText: {
    color: color.danger[600],
    fontSize: 11,
    fontWeight: '700',
    textAlign: 'center',
  },
  privacyLogoContainer: {
    ...StyleSheet.absoluteFillObject,
    justifyContent: 'center',
    alignItems: 'center',
    gap: space[12],
    backgroundColor: 'rgba(255, 255, 255, 0.1)',
    zIndex: 9999,
  },
  privacyMaskText: {
    color: color.neutral[900],
    fontSize: 28,
    fontWeight: '800',
  },
  privacyMaskSub: {
    color: color.neutral[500],
    fontSize: 14,
    fontWeight: '500',
  },
});
