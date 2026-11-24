import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { Camera, CameraView } from 'expo-camera';
import { useRouter } from 'expo-router';
import React, { useState, useEffect, useRef } from 'react';
import { StyleSheet, Text, View, Pressable, Image, Alert, ActivityIndicator } from 'react-native';

import { KYCPhotoFrame } from '@/src/features/kyc/components/KYCPhotoFrame';
import { validateKycPhoto } from '@/src/features/kyc/helpers/photo-validator';
import { useKycDraftStore } from '@/src/features/kyc/stores/kyc-draft';
import { Button } from '@/src/ui/Button';
import { color, radius, space } from '@/src/ui/tokens';

export default function CaptureIdScreen() {
  useSecureScreen();
  const router = useRouter();
  const [hasPermission, setHasPermission] = useState<boolean | null>(null);
  const [photoUri, setPhotoUri] = useState<string | null>(null);
  const [validating, setValidating] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [flashlight, setFlashlight] = useState<'off' | 'on'>('off');
  
  const cameraRef = useRef<any>(null);
  const setIdCard = useKycDraftStore((s) => s.setIdCard);
  const saveDraft = useKycDraftStore((s) => s.saveDraft);
  const draftIdCardUri = useKycDraftStore((s) => s.idCardUri);

  useEffect(() => {
    (async () => {
      const { status } = await Camera.requestCameraPermissionsAsync();
      setHasPermission(status === 'granted');
    })();
  }, []);

  // Use draft if available
  useEffect(() => {
    if (draftIdCardUri && !photoUri) {
      setPhotoUri(draftIdCardUri);
    }
  }, [draftIdCardUri, photoUri]);

  const handleCapture = async () => {
    if (cameraRef.current) {
      try {
        const photo = await cameraRef.current.takePictureAsync({ quality: 0.85 });
        await processCapturedPhoto(photo.uri);
      } catch {
        Alert.alert('Error', 'Could not take the photo. Please try again.');
      }
    } else {
      // Mock photo for simulator/web/tests
      await processCapturedPhoto('file:///mock-id-card-front.jpg');
    }
  };

  const processCapturedPhoto = async (uri: string) => {
    setValidating(true);
    setValidationError(null);
    try {
      const result = await validateKycPhoto(uri);
      if (!result.isValid) {
        setValidationError(result.error ?? 'Invalid photo.');
      } else {
        setPhotoUri(uri);
      }
    } catch {
      setValidationError('Something went wrong while checking the photo.');
    } finally {
      setValidating(false);
    }
  };

  const handleUsePhoto = async () => {
    if (photoUri) {
      setIdCard(photoUri);
      await saveDraft();
      router.push('/(kyc)/capture-selfie');
    }
  };

  const handleRetake = () => {
    setPhotoUri(null);
    setValidationError(null);
  };

  // Mock triggers for testing M10 edge cases in Expo Go / simulator
  const triggerMockError = async (type: 'dark' | 'blurry') => {
    await processCapturedPhoto(`file:///mock-id-card-${type}.jpg`);
  };

  if (hasPermission === null) {
    return (
      <View style={styles.centerContainer}>
        <ActivityIndicator size="large" color={color.brand[600]} />
        <Text style={styles.loadingText}>Requesting camera permission...</Text>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      {/* Navbar */}
      <View style={styles.navbar}>
        <Pressable onPress={() => router.back()} style={styles.closeButton} accessibilityLabel="Close">
          <Text style={styles.closeIcon}>✕</Text>
        </Pressable>
        <Text style={styles.navTitle}>Front of ID card</Text>
        <View style={{ width: 40 }} />
      </View>

      {!photoUri ? (
        // Camera View Mode
        <View style={styles.cameraContainer}>
          {hasPermission ? (
            <CameraView
              style={StyleSheet.absoluteFillObject}
              ref={cameraRef}
              facing="back"
              flash={flashlight}
            />
          ) : (
            <View style={styles.cameraMock}>
              <Text style={styles.cameraMockText}>[ CAMERA SIMULATION ]</Text>
              <Text style={styles.cameraMockSub}>No camera permission, or running on an emulator</Text>
              
              {/* Dev tools to test M10 heuristics */}
              <View style={styles.devButtons}>
                <Pressable onPress={() => triggerMockError('blurry')} style={styles.devButton}>
                  <Text style={styles.devButtonText}>Test blurry ⚠️</Text>
                </Pressable>
                <Pressable onPress={() => triggerMockError('dark')} style={styles.devButton}>
                  <Text style={styles.devButtonText}>Test dark ⚠️</Text>
                </Pressable>
              </View>
            </View>
          )}

          {/* Guide Overlay Frame */}
          <KYCPhotoFrame type="id-front" error={validationError} />

          {/* Bottom Controls */}
          <View style={styles.controlsContainer}>
            <Pressable onPress={() => triggerMockError('blurry')} style={styles.toolButton}>
              <Text style={styles.toolButtonText}>Library</Text>
            </Pressable>

            <Pressable onPress={handleCapture} style={styles.shutterButton} accessibilityLabel="Shutter">
              <View style={styles.shutterInner} />
            </Pressable>

            <Pressable
              onPress={() => setFlashlight(flashlight === 'off' ? 'on' : 'off')}
              style={styles.toolButton}
            >
              <Text style={styles.toolButtonText}>Light {flashlight === 'on' ? '⚡' : '✕'}</Text>
            </Pressable>
          </View>
        </View>
      ) : (
        // Photo Preview Mode
        <View style={styles.previewContainer}>
          <Image source={{ uri: photoUri }} style={styles.previewImage} resizeMode="contain" />
          
          <View style={styles.previewFooter}>
            <Text style={styles.previewGuideText}>
              Make sure the details on the ID card are sharp, with no glare or anything covering them.
            </Text>
            <View style={styles.previewButtons}>
              <Button label="Retake" variant="secondary" onPress={handleRetake} style={styles.previewBtn} />
              <Button label="Use this photo" onPress={handleUsePhoto} style={styles.previewBtn} />
            </View>
          </View>
        </View>
      )}

      {validating && (
        <View style={styles.loadingOverlay}>
          <ActivityIndicator size="large" color="#FFFFFF" />
          <Text style={styles.loadingOverlayText}>Checking photo quality...</Text>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0A0F1A',
  },
  centerContainer: {
    flex: 1,
    backgroundColor: '#0A0F1A',
    justifyContent: 'center',
    alignItems: 'center',
    gap: space[12],
  },
  loadingText: {
    color: '#FFFFFF',
    fontSize: 14,
  },
  navbar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingHorizontal: space[24],
    marginTop: 20,
    height: 48,
    zIndex: 10,
  },
  closeButton: {
    width: 40,
    height: 40,
    borderRadius: radius.full,
    backgroundColor: 'rgba(255, 255, 255, 0.12)',
    alignItems: 'center',
    justifyContent: 'center',
  },
  closeIcon: {
    color: '#FFFFFF',
    fontSize: 18,
    fontWeight: '700',
  },
  navTitle: {
    color: '#FFFFFF',
    fontSize: 16,
    fontWeight: '600',
  },
  cameraContainer: {
    flex: 1,
    position: 'relative',
  },
  cameraMock: {
    flex: 1,
    backgroundColor: '#1E293B',
    alignItems: 'center',
    justifyContent: 'center',
    gap: space[8],
  },
  cameraMockText: {
    color: '#94A3B8',
    fontSize: 18,
    fontWeight: '700',
    letterSpacing: 2,
  },
  cameraMockSub: {
    color: '#64748B',
    fontSize: 12,
  },
  devButtons: {
    flexDirection: 'row',
    gap: space[12],
    marginTop: space[24],
  },
  devButton: {
    backgroundColor: 'rgba(255, 255, 255, 0.15)',
    paddingVertical: space[8],
    paddingHorizontal: space[16],
    borderRadius: radius.sm,
  },
  devButtonText: {
    color: '#FFFFFF',
    fontSize: 12,
    fontWeight: '600',
  },
  controlsContainer: {
    position: 'absolute',
    bottom: 40,
    left: 0,
    right: 0,
    flexDirection: 'row',
    justifyContent: 'center',
    alignItems: 'center',
    gap: 44,
  },
  shutterButton: {
    width: 74,
    height: 74,
    borderRadius: radius.full,
    borderWidth: 4,
    borderColor: '#FFFFFF',
    alignItems: 'center',
    justifyContent: 'center',
  },
  shutterInner: {
    width: 58,
    height: 58,
    borderRadius: radius.full,
    backgroundColor: '#FFFFFF',
  },
  toolButton: {
    paddingVertical: space[8],
    paddingHorizontal: space[12],
  },
  toolButtonText: {
    color: '#94A3B8',
    fontSize: 13,
    fontWeight: '600',
  },
  previewContainer: {
    flex: 1,
    justifyContent: 'space-between',
    paddingBottom: space[40],
  },
  previewImage: {
    flex: 1,
    width: '100%',
  },
  previewFooter: {
    paddingHorizontal: space[24],
    gap: space[20],
  },
  previewGuideText: {
    color: '#94A3B8',
    fontSize: 13,
    lineHeight: 18,
    textAlign: 'center',
  },
  previewButtons: {
    flexDirection: 'row',
    gap: space[12],
  },
  previewBtn: {
    flex: 1,
  },
  loadingOverlay: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: 'rgba(0,0,0,0.7)',
    justifyContent: 'center',
    alignItems: 'center',
    gap: space[12],
  },
  loadingOverlayText: {
    color: '#FFFFFF',
    fontSize: 14,
    fontWeight: '600',
  },
});
