import { useSecureScreen } from '@/src/features/security/useSecureScreen';
import { Camera, CameraView } from 'expo-camera';
import { useRouter } from 'expo-router';
import React, { useState, useEffect, useRef } from 'react';
import { StyleSheet, Text, View, Pressable, Image, Alert, ActivityIndicator } from 'react-native';

import { API_BASE_URL } from '@/src/api/client';
import { KYCPhotoFrame } from '@/src/features/kyc/components/KYCPhotoFrame';
import { validateKycPhoto } from '@/src/features/kyc/helpers/photo-validator';
import { useKycDraftStore } from '@/src/features/kyc/stores/kyc-draft';
import { useSessionStore } from '@/src/stores/session';
import { Button } from '@/src/ui/Button';
import { color, radius, space } from '@/src/ui/tokens';

export default function CaptureSelfieScreen() {
  useSecureScreen();
  const router = useRouter();
  const [hasPermission, setHasPermission] = useState<boolean | null>(null);
  const [photoUri, setPhotoUri] = useState<string | null>(null);
  const [validating, setValidating] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [networkError, setNetworkError] = useState(false);

  const cameraRef = useRef<any>(null);
  const { idCardUri, selfieUri, setSelfie, saveDraft, clearDraft } = useKycDraftStore();
  const user = useSessionStore((s) => s.user);

  useEffect(() => {
    (async () => {
      const { status } = await Camera.requestCameraPermissionsAsync();
      setHasPermission(status === 'granted');
    })();
  }, []);

  // Use draft if available
  useEffect(() => {
    if (selfieUri && !photoUri) {
      setPhotoUri(selfieUri);
    }
  }, [selfieUri, photoUri]);

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
      await processCapturedPhoto('file:///mock-selfie.jpg');
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

  const uploadKycData = async (cardUri: string, selfUri: string) => {
    setUploading(true);
    setNetworkError(false);
    
    try {
      const formData = new FormData();
      
      // Append files. In react-native, files in FormData are represented as objects with uri, type, name.
      formData.append('id_card', {
        uri: cardUri,
        name: 'id_card.jpg',
        type: 'image/jpeg',
      } as any);

      formData.append('selfie', {
        uri: selfUri,
        name: 'selfie.jpg',
        type: 'image/jpeg',
      } as any);

      // We use rawRequest directly to support FormData header override (which requires browser/native to auto-generate the Boundary)
      // Since rawRequest stringifies options.body if Content-Type is not overridden, we handle it custom here.
      const accessToken = useSessionStore.getState().accessToken;

      const headers: Record<string, string> = {
        Accept: 'application/json',
      };
      if (accessToken) {
        headers.Authorization = `Bearer ${accessToken}`;
      }

      // Perform upload
      const response = await fetch(`${API_BASE_URL}/v1/kyc/submit`, {
        method: 'POST',
        headers,
        body: formData,
      });

      if (!response.ok) {
        // Handle API level error
        let errCode = 'ERR_UNKNOWN';
        try {
          const errBody = await response.json();
          errCode = errBody?.code ?? 'ERR_UNKNOWN';
        } catch {
          // no-op
        }
        throw new Error(errCode);
      }

      // Success! Clear drafts and refresh user state, then go to Status
      await clearDraft();
      
      // Update session store user's kyc_status to pending locally to reflect immediate state change
      if (user) {
        useSessionStore.getState().setSession(accessToken!, {
          ...user,
          kyc_status: 'pending',
        });
      }

      router.replace('/(kyc)/status');
    } catch (err: any) {
      if (err.message && err.message.startsWith('ERR_')) {
        // App error
        if (err.message === 'ERR_KYC_ALREADY_PENDING' || err.message === 'ERR_KYC_ALREADY_VERIFIED') {
          await clearDraft();
          router.replace('/(kyc)/status');
        } else {
          Alert.alert('Verification error', 'Could not submit your application: ' + err.message);
        }
      } else {
        // Network error (M11)
        setNetworkError(true);
      }
    } finally {
      setUploading(false);
    }
  };

  const handleUsePhoto = async () => {
    if (photoUri) {
      setSelfie(photoUri);
      await saveDraft();
      
      // ID Card front must be available
      const cardUri = idCardUri || (useKycDraftStore.getState().idCardUri);
      if (!cardUri) {
        Alert.alert('Error', 'Could not find the front photo of your ID card. Please go back to the previous step.', [
          { text: 'Back', onPress: () => router.back() }
        ]);
        return;
      }
      
      await uploadKycData(cardUri, photoUri);
    }
  };

  const handleRetake = () => {
    setPhotoUri(null);
    setValidationError(null);
  };

  const handleRetryUpload = async () => {
    const cardUri = idCardUri || (useKycDraftStore.getState().idCardUri);
    const selfUri = photoUri;
    if (cardUri && selfUri) {
      await uploadKycData(cardUri, selfUri);
    }
  };

  // Mock triggers for testing M10/M11 edge cases in simulator/unit tests
  const triggerMockError = async (type: 'dark' | 'blurry') => {
    await processCapturedPhoto(`file:///mock-selfie-${type}.jpg`);
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
        <Pressable onPress={() => router.back()} style={styles.closeButton} accessibilityLabel="Back">
          <Text style={styles.closeIcon}>←</Text>
        </Pressable>
        <Text style={styles.navTitle}>Take a selfie</Text>
        <View style={{ width: 40 }} />
      </View>

      {!photoUri ? (
        // Camera View Mode
        <View style={styles.cameraContainer}>
          {hasPermission ? (
            <CameraView
              style={StyleSheet.absoluteFillObject}
              ref={cameraRef}
              facing="front"
            />
          ) : (
            <View style={styles.cameraMock}>
              <Text style={styles.cameraMockText}>[ SELFIE MOCK ]</Text>
              <Text style={styles.cameraMockSub}>No camera permission, or running on an emulator</Text>

              {/* Dev tools to test M10 heuristics */}
              <View style={styles.devButtons}>
                <Pressable onPress={() => triggerMockError('blurry')} style={styles.devButton}>
                  <Text style={styles.devButtonText}>Test blur ⚠️</Text>
                </Pressable>
                <Pressable onPress={() => triggerMockError('dark')} style={styles.devButton}>
                  <Text style={styles.devButtonText}>Test dark ⚠️</Text>
                </Pressable>
              </View>
            </View>
          )}

          {/* Guide Overlay Frame (Oval) */}
          <KYCPhotoFrame type="selfie" error={validationError} />

          {/* Bottom Controls */}
          <View style={styles.controlsContainer}>
            <View style={{ width: 60 }} />

            <Pressable onPress={handleCapture} style={styles.shutterButton} accessibilityLabel="Shutter">
              <View style={styles.shutterInner} />
            </Pressable>

            <View style={{ width: 60 }} />
          </View>
        </View>
      ) : (
        // Photo Preview Mode
        <View style={styles.previewContainer}>
          <Image source={{ uri: photoUri }} style={styles.previewImage} resizeMode="contain" />
          
          <View style={styles.previewFooter}>
            {networkError ? (
              <View style={styles.errorBannerContainer}>
                <Text style={styles.errorBannerText}>
                  ⚠️ No network connection. KYC draft saved locally.
                </Text>
                <View style={styles.previewButtons}>
                  <Button label="Retake" variant="secondary" onPress={handleRetake} style={styles.previewBtn} />
                  <Button label="Upload again (Retry)" onPress={handleRetryUpload} style={styles.previewBtn} />
                </View>
              </View>
            ) : (
              <>
                <Text style={styles.previewGuideText}>
                  Make sure your face fits in the oval, is sharp and well lit.
                </Text>
                <View style={styles.previewButtons}>
                  <Button label="Retake" variant="secondary" onPress={handleRetake} style={styles.previewBtn} />
                  <Button label="Use this photo" onPress={handleUsePhoto} style={styles.previewBtn} />
                </View>
              </>
            )}
          </View>
        </View>
      )}

      {validating && (
        <View style={styles.loadingOverlay}>
          <ActivityIndicator size="large" color="#FFFFFF" />
          <Text style={styles.loadingOverlayText}>Checking photo quality...</Text>
        </View>
      )}

      {uploading && (
        <View style={styles.loadingOverlay}>
          <ActivityIndicator size="large" color="#FFFFFF" />
          <Text style={styles.loadingOverlayText}>Submitting your KYC application...</Text>
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
  errorBannerContainer: {
    gap: space[16],
    width: '100%',
  },
  errorBannerText: {
    color: color.danger[600],
    fontSize: 14,
    fontWeight: '600',
    textAlign: 'center',
    backgroundColor: color.danger[50],
    paddingVertical: 10,
    paddingHorizontal: space[16],
    borderRadius: radius.sm,
    overflow: 'hidden',
  },
  loadingOverlay: {
    ...StyleSheet.absoluteFillObject,
    backgroundColor: 'rgba(0,0,0,0.7)',
    justifyContent: 'center',
    alignItems: 'center',
    gap: space[12],
    zIndex: 100,
  },
  loadingOverlayText: {
    color: '#FFFFFF',
    fontSize: 14,
    fontWeight: '600',
  },
});
