import { useRouter } from 'expo-router';
import React, { useState, useEffect, useRef } from 'react';
import { StyleSheet, Text, View, Pressable, Animated } from 'react-native';

import { Button } from '@/src/ui/Button';
import { LogoMark } from '@/src/ui/Logo';
import { color, radius, space, type } from '@/src/ui/tokens';

const SLIDES = [
  {
    icon: '💠',
    title: 'Transfer money in\none tap',
    subtitle: 'A secure Wallet with multi-level Approval\nfor individuals and businesses',
    amount: '$12,450.00',
    cardHolder: '••4821',
  },
  {
    icon: '🏢',
    title: 'A Wallet built\nfor business',
    subtitle: 'Professional Maker-Checker workflow,\nflexible multi-level payment Approval',
    amount: '$85,000.00',
    cardHolder: 'DONG A GROUP',
  },
  {
    icon: '🛡️',
    title: 'Smart security\nand safe',
    subtitle: 'Identity verification powered by AI\nand 24/7 monitoring of unusual Transactions',
    amount: '$400.00',
    cardHolder: 'DAILY LIMIT',
  },
];

export default function OnboardingScreen() {
  const router = useRouter();
  const [currentSlide, setCurrentSlide] = useState(0);
  const touchStartX = useRef(0);
  const fadeAnim = useRef(new Animated.Value(1)).current;

  const transitionToSlide = (targetIndex: number) => {
    Animated.timing(fadeAnim, {
      toValue: 0,
      duration: 180,
      useNativeDriver: true,
    }).start(() => {
      setCurrentSlide(targetIndex);
      Animated.timing(fadeAnim, {
        toValue: 1,
        duration: 220,
        useNativeDriver: true,
      }).start();
    });
  };

  useEffect(() => {
    const timer = setInterval(() => {
      const nextIndex = currentSlide < SLIDES.length - 1 ? currentSlide + 1 : 0;
      transitionToSlide(nextIndex);
    }, 4500);
    return () => clearInterval(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentSlide]);

  const handleTouchStart = (e: any) => {
    touchStartX.current = e.nativeEvent.pageX;
  };

  const handleTouchEnd = (e: any) => {
    const deltaX = e.nativeEvent.pageX - touchStartX.current;
    if (deltaX < -50) {
      // Swipe left -> next slide
      const nextIndex = currentSlide < SLIDES.length - 1 ? currentSlide + 1 : 0;
      transitionToSlide(nextIndex);
    } else if (deltaX > 50) {
      // Swipe right -> prev slide
      const nextIndex = currentSlide > 0 ? currentSlide - 1 : SLIDES.length - 1;
      transitionToSlide(nextIndex);
    }
  };

  const slide = SLIDES[currentSlide];

  return (
    <View style={styles.container}>
      <View style={styles.statusbarSpacer} />

      {/* Hero Visual Card (Swipeable) */}
      <View 
        style={styles.cardContainer}
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
      >
        <Animated.View style={[styles.animationWrapper, { opacity: fadeAnim }]}>
          <View style={styles.heroCard}>
            {currentSlide === 0 ? (
              <LogoMark size={28} tint="#FFFFFF" />
            ) : (
              <Text style={styles.heroIcon}>{slide.icon}</Text>
            )}
            <View style={styles.heroTextContainer}>
              <Text style={styles.heroLabel}>Wallet balance</Text>
              <Text style={styles.heroAmount}>{slide.amount}</Text>
            </View>
            <View style={styles.heroFooter}>
              <Text style={styles.heroHolder}>{slide.cardHolder}</Text>
              <Text style={styles.heroBrandName}>HPay</Text>
            </View>
          </View>

          {/* Small floating alert mockup for slide 1 */}
          {currentSlide === 0 && (
            <View style={styles.floatingAlert}>
              <View style={styles.alertCheck}>
                <Text style={styles.alertCheckText}>✓</Text>
              </View>
              <View>
                <Text style={styles.alertTitle}>Received $20.00</Text>
                <Text style={styles.alertSub}>from Minh Anh · just now</Text>
              </View>
            </View>
          )}
        </Animated.View>
      </View>

      {/* Content description */}
      <View style={styles.content}>
        <Animated.View style={[styles.textWrapper, { opacity: fadeAnim }]}>
          <Text style={styles.title}>{slide.title}</Text>
          <Text style={styles.subtitle}>{slide.subtitle}</Text>
        </Animated.View>

        {/* Indicator dots */}
        <View style={styles.dotsRow}>
          {SLIDES.map((_, i) => (
            <Pressable
              key={i}
              onPress={() => transitionToSlide(i)}
              style={[
                styles.dot,
                currentSlide === i ? styles.dotActive : null,
              ]}
            />
          ))}
        </View>
      </View>

      {/* Buttons */}
      <View style={styles.footer}>
        <Button
          label={currentSlide === SLIDES.length - 1 ? 'Get started' : 'Sign up'}
          onPress={() => router.push('/(auth)/register')}
          style={styles.button}
        />
        <Button
          label="I already have an account"
          variant="ghost"
          onPress={() => router.push('/(auth)/login')}
          style={styles.button}
        />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  animationWrapper: {
    flex: 1,
    width: '100%',
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
  },
  textWrapper: {
    width: '100%',
    alignItems: 'center',
  },
  container: {
    flex: 1,
    backgroundColor: color.neutral[50],
    justifyContent: 'space-between',
    paddingHorizontal: space[24],
    paddingBottom: space[40],
  },
  statusbarSpacer: {
    height: 48,
  },
  cardContainer: {
    height: 340,
    borderRadius: 32,
    backgroundColor: '#E6FAFD', // linear-gradient mockup (brand-50)
    alignItems: 'center',
    justifyContent: 'center',
    position: 'relative',
    overflow: 'hidden',
  },
  heroCard: {
    width: 250,
    height: 156,
    borderRadius: radius.lg,
    backgroundColor: color.brand[600],
    padding: space[16],
    justifyContent: 'space-between',
    transform: [{ rotate: '-6deg' }],
    shadowColor: color.brand[600],
    shadowOffset: { width: 0, height: 12 },
    shadowOpacity: 0.3,
    shadowRadius: 24,
    elevation: 8,
  },
  heroIcon: {
    fontSize: 24,
    color: '#FFFFFF',
  },
  heroTextContainer: {
    marginTop: space[8],
  },
  heroLabel: {
    fontSize: 12,
    fontWeight: '600',
    color: 'rgba(255, 255, 255, 0.8)',
  },
  heroAmount: {
    fontSize: 24,
    fontWeight: '700',
    color: '#FFFFFF',
  },
  heroFooter: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  heroHolder: {
    fontSize: 12,
    fontWeight: '600',
    color: 'rgba(255, 255, 255, 0.85)',
  },
  heroBrandName: {
    fontSize: 12,
    fontWeight: '700',
    color: '#FFFFFF',
  },
  floatingAlert: {
    position: 'absolute',
    bottom: 26,
    right: 26,
    backgroundColor: '#FFFFFF',
    borderRadius: radius.md,
    paddingVertical: space[8],
    paddingHorizontal: space[12],
    shadowColor: '#101828',
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.1,
    shadowRadius: 16,
    elevation: 4,
    flexDirection: 'row',
    alignItems: 'center',
    gap: space[8],
    transform: [{ rotate: '4deg' }],
  },
  alertCheck: {
    width: 24,
    height: 24,
    borderRadius: radius.full,
    backgroundColor: color.success[50],
    alignItems: 'center',
    justifyContent: 'center',
  },
  alertCheckText: {
    color: color.success[600],
    fontWeight: '700',
    fontSize: 12,
  },
  alertTitle: {
    fontSize: 12,
    fontWeight: '600',
    color: color.neutral[900],
  },
  alertSub: {
    fontSize: 10,
    color: color.neutral[500],
  },
  content: {
    alignItems: 'center',
    marginTop: space[24],
  },
  title: {
    fontSize: 26,
    lineHeight: 34,
    fontWeight: '700',
    color: color.neutral[900],
    textAlign: 'center',
    marginBottom: space[12],
  },
  subtitle: {
    fontSize: type.body.fontSize,
    lineHeight: type.body.lineHeight,
    color: color.neutral[500],
    textAlign: 'center',
    marginBottom: space[20],
  },
  dotsRow: {
    flexDirection: 'row',
    gap: 6,
  },
  dot: {
    width: 6,
    height: 6,
    borderRadius: radius.full,
    backgroundColor: color.neutral[300],
  },
  dotActive: {
    width: 24,
    backgroundColor: color.brand[600],
  },
  footer: {
    gap: space[12],
    marginTop: space[24],
  },
  button: {
    width: '100%',
  },
});
