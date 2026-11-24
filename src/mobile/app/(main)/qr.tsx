import React, { useState, useEffect } from 'react';
import { StyleSheet, Text, View, Pressable, ActivityIndicator, Alert } from 'react-native';
import { useRouter } from 'expo-router';
import { Camera, CameraView } from 'expo-camera';
import { Ionicons } from '@expo/vector-icons';
import { SafeAreaView } from 'react-native-safe-area-context';
import QRCode from 'react-native-qrcode-svg';

import { useMyQR, useResolveQR, useMe } from '@/src/queries';
import { color } from '@/src/ui/tokens';

type QRMode = 'my-qr' | 'scan';

export default function QRScreen() {
  const router = useRouter();
  const [mode, setMode] = useState<QRMode>('my-qr');
  const [hasPermission, setHasPermission] = useState<boolean | null>(null);
  const [scanned, setScanned] = useState(false);

  const meQuery = useMe();
  const myQrQuery = useMyQR();
  const resolveMutation = useResolveQR();

  useEffect(() => {
    if (mode === 'scan') {
      (async () => {
        const { status } = await Camera.requestCameraPermissionsAsync();
        setHasPermission(status === 'granted');
      })();
    }
  }, [mode]);

  const parseQRUrl = (url: string): { payload: string; signature: string } | null => {
    try {
      const urlObj = new URL(url);
      if (urlObj.protocol !== 'hpay:') return null;
      const payload = urlObj.searchParams.get('payload');
      const signature = urlObj.searchParams.get('signature');
      if (payload && signature) {
        return { payload, signature };
      }
    } catch {
      try {
        const parsed = JSON.parse(url);
        if (parsed.payload && parsed.signature) {
          return { payload: parsed.payload, signature: parsed.signature };
        }
      } catch {}
    }
    return null;
  };

  const handleBarcodeScanned = async ({ data }: { data: string }) => {
    if (scanned || resolveMutation.isPending) return;
    setScanned(true);

    const parsed = parseQRUrl(data);
    if (!parsed) {
      Alert.alert('QR code error', 'This QR code is not in HPay format.', [
        { text: 'Scan again', onPress: () => setScanned(false) }
      ]);
      return;
    }

    try {
      const res = await resolveMutation.mutateAsync({
        payload: parsed.payload,
        signature: parsed.signature,
      });

      router.replace({
        pathname: '/(main)/transfer/confirm',
        params: {
          receiverToken: res.receiver_token,
          maskedName: res.name_masked,
          phone: res.phone_masked,
          amount: '',
          note: 'Transfer via QR',
        },
      });
    } catch (err: any) {
      Alert.alert('Could not pay with QR', err?.message || 'The QR code is invalid or has expired.', [
        { text: 'Scan again', onPress: () => setScanned(false) }
      ]);
    }
  };

  const handleMockScan = async () => {
    if (!myQrQuery.data) {
      Alert.alert('Error', 'No QR payload to mock.');
      return;
    }
    const fakeData = `hpay://qr?payload=${myQrQuery.data.payload}&signature=${myQrQuery.data.signature}`;
    await handleBarcodeScanned({ data: fakeData });
  };

  const renderMyQR = () => {
    if (myQrQuery.isLoading || meQuery.isLoading) {
      return (
        <View className="flex-1 items-center justify-center gap-3">
          <ActivityIndicator size="large" color="#0E90A3" />
        </View>
      );
    }

    if (myQrQuery.isError || !myQrQuery.data || !meQuery.data) {
      return (
        <View className="flex-1 items-center justify-center gap-3">
          <Text className="text-sm color-rose-600">Could not load your QR code.</Text>
        </View>
      );
    }

    const { payload, signature } = myQrQuery.data;
    const { user } = meQuery.data;
    const qrString = `hpay://qr?payload=${payload}&signature=${signature}`;

    return (
      <View className="flex-1 items-center justify-center p-6">
        <View className="bg-white dark:bg-slate-800 rounded-2xl p-6 items-center w-full border border-slate-200 dark:border-slate-700 shadow-sm">
          <Text className="text-lg font-bold text-slate-900 dark:text-white mb-1">HPay payment QR code</Text>
          <Text className="text-xs text-slate-400 mb-6">Scan this code to send me money</Text>
          
          <View className="p-4 rounded-xl border border-cyan-100 bg-white mb-5">
            <QRCode
              value={qrString}
              size={220}
              color={color.neutral[900]}
              backgroundColor="#FFFFFF"
            />
          </View>

          <Text className="text-lg font-bold text-slate-900 dark:text-white mb-1">{user.full_name}</Text>
          <Text className="text-sm text-slate-500">{user.phone}</Text>
        </View>
      </View>
    );
  };

  const renderScanQR = () => {
    if (hasPermission === null) {
      return (
        <View className="flex-1 items-center justify-center gap-3">
          <ActivityIndicator size="large" color="#0E90A3" />
          <Text className="text-sm text-slate-500">Requesting camera permission...</Text>
        </View>
      );
    }

    return (
      <View className="flex-1 relative bg-black">
        {hasPermission ? (
          <CameraView
            style={StyleSheet.absoluteFillObject}
            barcodeScannerSettings={{
              barcodeTypes: ['qr'],
            }}
            onBarcodeScanned={scanned ? undefined : handleBarcodeScanned}
          />
        ) : (
          <View className="flex-1 items-center justify-center bg-slate-900 gap-3 p-5">
            <Ionicons name="camera-reverse-outline" size={48} color="#94a3b8" />
            <Text className="text-slate-400 text-base font-bold">[ CAMERA SIMULATOR MODE ]</Text>
            <Text className="text-slate-500 text-xs text-center leading-relaxed">No real camera access on this simulator.</Text>
            
            <Pressable onPress={handleMockScan} className="mt-4 bg-brand px-5 py-2.5 rounded-xl">
              <Text className="text-white text-sm font-bold">Simulate scanning own QR ⚡</Text>
            </Pressable>
          </View>
        )}
        <View className="absolute inset-0 justify-center items-center">
          <View className="w-60 h-60 border-2 border-cyan-400 rounded-2xl bg-transparent" />
          <Text className="text-white text-xs font-semibold mt-6 bg-black/60 px-4 py-2 rounded-full overflow-hidden">Point the camera at a QR code to scan</Text>
        </View>

        {resolveMutation.isPending && (
          <View className="absolute inset-0 bg-black/70 justify-center items-center gap-3 z-20">
            <ActivityIndicator size="large" color="#FFFFFF" />
            <Text className="text-white text-sm font-semibold">Verifying Account details...</Text>
          </View>
        )}
      </View>
    );
  };

  return (
    <SafeAreaView className="flex-1 bg-slate-50 dark:bg-slate-900" edges={['top']}>
      <View className="flex-row items-center justify-between px-4 py-3 bg-white dark:bg-slate-800 border-b border-slate-100 dark:border-slate-700">
        <Pressable onPress={() => router.back()} className="p-1">
          <Ionicons name="close" size={24} color="#1e293b" />
        </Pressable>
        <View className="flex-row bg-slate-100 dark:bg-slate-700 rounded-lg p-0.5 w-44">
          <Pressable
            onPress={() => setMode('my-qr')}
            className={`flex-1 py-1.5 items-center rounded-md ${mode === 'my-qr' ? 'bg-white dark:bg-slate-800 shadow-xs' : ''}`}
          >
            <Text className={`text-xs font-semibold ${mode === 'my-qr' ? 'text-brand' : 'text-slate-500'}`}>
              Receive money
            </Text>
          </Pressable>
          <Pressable
            onPress={() => setMode('scan')}
            className={`flex-1 py-1.5 items-center rounded-md ${mode === 'scan' ? 'bg-white dark:bg-slate-800 shadow-xs' : ''}`}
          >
            <Text className={`text-xs font-semibold ${mode === 'scan' ? 'text-brand' : 'text-slate-500'}`}>
              Scan
            </Text>
          </Pressable>
        </View>
        <View className="w-8" />
      </View>

      <View className="flex-1">
        {mode === 'my-qr' ? renderMyQR() : renderScanQR()}
      </View>
    </SafeAreaView>
  );
}
