import React from 'react';
import { StyleSheet, View, Text, Dimensions } from 'react-native';

import { color, radius, space } from '@/src/ui/tokens';

const { width, height } = Dimensions.get('window');

interface KYCPhotoFrameProps {
  type: 'id-front' | 'selfie';
  error?: string | null;
}

export function KYCPhotoFrame({ type, error }: KYCPhotoFrameProps) {
  const isId = type === 'id-front';

  return (
    <View style={StyleSheet.absoluteFillObject} pointerEvents="none">
      {/* Dimmed background masks */}
      <View style={styles.overlayContainer}>
        {/* Top Mask */}
        <View style={[styles.maskDim, { height: isId ? (height - 236) / 2 : (height - 320) / 2 }]} />
        
        <View style={styles.middleRow}>
          {/* Left Mask */}
          <View style={[styles.maskDim, { width: isId ? 28 : (width - 240) / 2 }]} />
          
          {/* Cutout Area with outline */}
          <View
            style={[
              styles.cutout,
              isId ? styles.cutoutId : styles.cutoutSelfie,
              error ? styles.cutoutError : null,
            ]}
          >
            {/* Corner highlights for ID card */}
            {isId && (
              <>
                <View style={[styles.corner, styles.topLeft]} />
                <View style={[styles.corner, styles.topRight]} />
                <View style={[styles.corner, styles.bottomLeft]} />
                <View style={[styles.corner, styles.bottomRight]} />
                <Text style={styles.cutoutText}>CĂN CƯỚC CÔNG DÂN</Text>
              </>
            )}
            
            {!isId && (
              <View style={styles.selfieInnerBorder}>
                <Text style={styles.cutoutText}>KHUÔN MẶT</Text>
              </View>
            )}
          </View>
          
          {/* Right Mask */}
          <View style={[styles.maskDim, { width: isId ? 28 : (width - 240) / 2 }]} />
        </View>

        {/* Bottom Mask */}
        <View style={[styles.maskDim, { flex: 1 }]} />
      </View>

      {/* Error banner */}
      {error && (
        <View style={styles.errorBanner}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  overlayContainer: {
    flex: 1,
  },
  maskDim: {
    backgroundColor: 'rgba(10, 15, 26, 0.7)',
  },
  middleRow: {
    flexDirection: 'row',
  },
  cutout: {
    backgroundColor: 'transparent',
    borderColor: color.brand[600],
    borderWidth: 2,
    position: 'relative',
    justifyContent: 'center',
    alignItems: 'center',
  },
  cutoutId: {
    width: width - 56,
    height: 236,
    borderRadius: 18,
  },
  cutoutSelfie: {
    width: 240,
    height: 320,
    borderRadius: 160, // Circular/Oval
  },
  selfieInnerBorder: {
    width: '96%',
    height: '96%',
    borderRadius: 150,
    borderWidth: 1,
    borderColor: 'rgba(255, 255, 255, 0.3)',
    borderStyle: 'dashed',
    justifyContent: 'center',
    alignItems: 'center',
  },
  cutoutError: {
    borderColor: color.danger[600],
  },
  corner: {
    position: 'absolute',
    width: 24,
    height: 24,
    borderColor: '#FFFFFF',
  },
  topLeft: {
    top: -2,
    left: -2,
    borderTopWidth: 4,
    borderLeftWidth: 4,
    borderTopLeftRadius: 16,
  },
  topRight: {
    top: -2,
    right: -2,
    borderTopWidth: 4,
    borderRightWidth: 4,
    borderTopRightRadius: 16,
  },
  bottomLeft: {
    bottom: -2,
    left: -2,
    borderBottomWidth: 4,
    borderLeftWidth: 4,
    borderBottomLeftRadius: 16,
  },
  bottomRight: {
    bottom: -2,
    right: -2,
    borderBottomWidth: 4,
    borderRightWidth: 4,
    borderBottomRightRadius: 16,
  },
  cutoutText: {
    color: 'rgba(255, 255, 255, 0.5)',
    fontSize: 12,
    fontWeight: '700',
    letterSpacing: 2,
  },
  errorBanner: {
    position: 'absolute',
    top: 120,
    left: space[24],
    right: space[24],
    backgroundColor: color.danger[600],
    borderRadius: radius.sm,
    padding: space[12],
    alignItems: 'center',
    justifyContent: 'center',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.2,
    shadowRadius: 8,
    elevation: 4,
  },
  errorText: {
    color: '#FFFFFF',
    fontSize: 13,
    fontWeight: '600',
    textAlign: 'center',
  },
});
