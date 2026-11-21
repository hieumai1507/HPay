import { zodResolver } from '@hookform/resolvers/zod';
import { useRouter } from 'expo-router';
import React, { useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import {
  StyleSheet,
  Text,
  TextInput,
  View,
  Pressable,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import { Button } from '@/src/ui/Button';
import { color, radius, space, type } from '@/src/ui/tokens';
import { registerSchema, type RegisterFormData } from '@/src/features/auth/schemas/auth-validation';

function calculatePasswordStrength(password: string): {
  score: number;
  label: string;
  color: string;
} {
  if (!password) return { score: 0, label: 'Trống', color: color.neutral[300] };
  let score = 0;
  if (password.length >= 8) score++;
  if (/[A-Z]/.test(password) && /[a-z]/.test(password)) score++;
  if (/[0-9]/.test(password)) score++;
  if (/[^A-Za-z0-9]/.test(password)) score++;

  switch (score) {
    case 1:
      return { score, label: 'Yếu', color: color.danger[600] };
    case 2:
      return { score, label: 'Trung bình', color: color.warning[600] };
    case 3:
      return { score, label: 'Khá', color: color.brand[600] };
    case 4:
      return { score, label: 'Mạnh', color: color.success[600] };
    default:
      return { score: 0, label: 'Yếu', color: color.danger[600] };
  }
}

export default function RegisterScreen() {
  const router = useRouter();
  const [secureText, setSecureText] = useState(true);

  const {
    control,
    handleSubmit,
    watch,
    formState: { errors },
  } = useForm<RegisterFormData>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      fullName: '',
      phone: '',
      password: '',
      agree: false,
    },
  });

  const passwordVal = watch('password');
  const strength = calculatePasswordStrength(passwordVal);

  const onSubmit = (data: RegisterFormData) => {
    // Normalize phone number (prepend 0 if not present)
    const formattedPhone = data.phone.startsWith('0') ? data.phone : '0' + data.phone;
    // Navigate to OTP screen and pass register info as route params
    router.push({
      pathname: '/(auth)/otp',
      params: {
        phone: formattedPhone,
        password: data.password,
        fullName: data.fullName,
        mode: 'register',
      },
    });
  };

  return (
    <KeyboardAvoidingView
      style={{ flex: 1, backgroundColor: color.neutral[50] }}
      behavior={Platform.OS === 'ios' ? 'padding' : 'height'}
    >
      <ScrollView contentContainerStyle={styles.scrollContainer} keyboardShouldPersistTaps="handled">
        {/* Navbar back button */}
        <Pressable onPress={() => router.back()} style={styles.backButton} accessibilityLabel="Back">
          <Text style={styles.backIcon}>←</Text>
        </Pressable>

        {/* Title */}
        <View style={styles.header}>
          <Text style={styles.title}>Tạo tài khoản</Text>
          <Text style={styles.subtitle}>Chỉ mất 2 phút, miễn phí trọn đời</Text>
        </View>

        {/* Form Fields */}
        <View style={styles.form}>
          {/* Full Name */}
          <View style={styles.fieldContainer}>
            <Text style={styles.fieldLabel}>Họ và tên</Text>
            <Controller
              control={control}
              name="fullName"
              render={({ field: { onChange, onBlur, value } }) => (
                <TextInput
                  placeholder="Nhập họ và tên của bạn"
                  placeholderTextColor={color.neutral[400]}
                  autoCapitalize="words"
                  onBlur={onBlur}
                  onChangeText={onChange}
                  value={value}
                  style={[styles.input, errors.fullName && styles.inputError]}
                />
              )}
            />
            {errors.fullName && <Text style={styles.errorText}>{errors.fullName.message}</Text>}
          </View>

          {/* Phone */}
          <View style={styles.fieldContainer}>
            <Text style={styles.fieldLabel}>Số điện thoại</Text>
            <View style={[styles.phoneInputWrapper, errors.phone && styles.inputError]}>
              <Text style={styles.phonePrefix}>+84</Text>
              <View style={styles.phoneDivider} />
              <Controller
                control={control}
                name="phone"
                render={({ field: { onChange, onBlur, value } }) => (
                  <TextInput
                    placeholder="912345678"
                    placeholderTextColor={color.neutral[400]}
                    keyboardType="phone-pad"
                    maxLength={10}
                    onBlur={onBlur}
                    onChangeText={onChange}
                    value={value}
                    style={styles.phoneInput}
                  />
                )}
              />
            </View>
            {errors.phone && <Text style={styles.errorText}>{errors.phone.message}</Text>}
          </View>

          {/* Password */}
          <View style={styles.fieldContainer}>
            <Text style={styles.fieldLabel}>Mật khẩu</Text>
            <View style={[styles.passwordInputWrapper, errors.password && styles.inputError]}>
              <Controller
                control={control}
                name="password"
                render={({ field: { onChange, onBlur, value } }) => (
                  <TextInput
                    placeholder="Nhập mật khẩu (tối thiểu 8 ký tự)"
                    placeholderTextColor={color.neutral[400]}
                    secureTextEntry={secureText}
                    onBlur={onBlur}
                    onChangeText={onChange}
                    value={value}
                    style={styles.passwordInput}
                  />
                )}
              />
              <Pressable
                onPress={() => setSecureText(!secureText)}
                style={styles.eyeButton}
                accessibilityLabel="Toggle password visibility"
              >
                <Text style={styles.eyeIcon}>{secureText ? '👁️' : '🙈'}</Text>
              </Pressable>
            </View>
            {errors.password && <Text style={styles.errorText}>{errors.password.message}</Text>}

            {/* Password Strength Meter */}
            {passwordVal.length > 0 && (
              <View style={styles.strengthWrapper}>
                <View style={styles.strengthBars}>
                  {Array(4)
                    .fill(0)
                    .map((_, i) => (
                      <View
                        key={i}
                        style={[
                          styles.strengthBar,
                          {
                            backgroundColor:
                              i < strength.score ? strength.color : color.neutral[200],
                          },
                        ]}
                      />
                    ))}
                </View>
                <Text style={[styles.strengthLabel, { color: strength.color }]}>
                  {strength.label}
                </Text>
              </View>
            )}
          </View>

          {/* Agree Terms Checkbox */}
          <Controller
            control={control}
            name="agree"
            render={({ field: { onChange, value } }) => (
              <Pressable
                onPress={() => onChange(!value)}
                style={styles.checkboxContainer}
                accessibilityRole="checkbox"
                accessibilityState={{ checked: value }}
              >
                <View style={[styles.checkbox, value && styles.checkboxChecked]}>
                  {value && <Text style={styles.checkboxTick}>✓</Text>}
                </View>
                <Text style={styles.checkboxLabel}>
                  Tôi đồng ý với{' '}
                  <Text style={styles.linkText}>Điều khoản sử dụng</Text> và{' '}
                  <Text style={styles.linkText}>Chính sách bảo mật</Text> của HPay
                </Text>
              </Pressable>
            )}
          />
          {errors.agree && <Text style={styles.errorText}>{errors.agree.message}</Text>}
        </View>

        {/* Footer Actions */}
        <View style={styles.footer}>
          <Button label="Tiếp tục" onPress={handleSubmit(onSubmit)} style={styles.submitButton} />
          <Pressable onPress={() => router.push('/(auth)/login')} style={styles.loginLink}>
            <Text style={styles.footerText}>
              Đã có tài khoản? <Text style={styles.linkBold}>Đăng nhập</Text>
            </Text>
          </Pressable>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  scrollContainer: {
    flexGrow: 1,
    paddingHorizontal: space[24],
    paddingBottom: space[40],
    justifyContent: 'space-between',
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
    marginTop: 20,
    marginBottom: space[24],
  },
  backIcon: {
    fontSize: 20,
    color: color.neutral[900],
    fontWeight: '700',
  },
  header: {
    marginBottom: space[32],
  },
  title: {
    fontSize: type.h1.fontSize,
    lineHeight: type.h1.lineHeight,
    fontWeight: '700',
    color: color.neutral[900],
    marginBottom: space[8],
  },
  subtitle: {
    fontSize: type.body.fontSize,
    color: color.neutral[500],
  },
  form: {
    gap: space[20],
  },
  fieldContainer: {
    gap: space[8],
  },
  fieldLabel: {
    fontSize: 14,
    fontWeight: '600',
    color: color.neutral[700],
  },
  input: {
    height: 48,
    borderRadius: radius.sm,
    borderWidth: 1.5,
    borderColor: color.neutral[300],
    paddingHorizontal: space[16],
    fontSize: type.body.fontSize,
    color: color.neutral[900],
    backgroundColor: '#FFFFFF',
  },
  inputError: {
    borderColor: color.danger[600],
  },
  phoneInputWrapper: {
    height: 48,
    flexDirection: 'row',
    alignItems: 'center',
    borderRadius: radius.sm,
    borderWidth: 1.5,
    borderColor: color.neutral[300],
    backgroundColor: '#FFFFFF',
    paddingHorizontal: space[16],
  },
  phonePrefix: {
    fontSize: type.body.fontSize,
    fontWeight: '600',
    color: color.neutral[700],
  },
  phoneDivider: {
    width: 1,
    height: 20,
    backgroundColor: color.neutral[200],
    marginHorizontal: space[12],
  },
  phoneInput: {
    flex: 1,
    height: '100%',
    fontSize: type.body.fontSize,
    color: color.neutral[900],
  },
  passwordInputWrapper: {
    height: 48,
    flexDirection: 'row',
    alignItems: 'center',
    borderRadius: radius.sm,
    borderWidth: 1.5,
    borderColor: color.neutral[300],
    backgroundColor: '#FFFFFF',
    paddingLeft: space[16],
  },
  passwordInput: {
    flex: 1,
    height: '100%',
    fontSize: type.body.fontSize,
    color: color.neutral[900],
  },
  eyeButton: {
    padding: space[12],
    justifyContent: 'center',
    alignItems: 'center',
  },
  eyeIcon: {
    fontSize: 16,
  },
  strengthWrapper: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginTop: space[8],
  },
  strengthBars: {
    flexDirection: 'row',
    gap: 6,
    flex: 1,
    marginRight: space[12],
  },
  strengthBar: {
    height: 4,
    borderRadius: radius.full,
    flex: 1,
  },
  strengthLabel: {
    fontSize: 12,
    fontWeight: '700',
  },
  checkboxContainer: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: 10,
    marginTop: space[8],
  },
  checkbox: {
    width: 20,
    height: 20,
    borderRadius: 6,
    borderWidth: 1.5,
    borderColor: color.neutral[300],
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#FFFFFF',
  },
  checkboxChecked: {
    borderColor: color.brand[600],
    backgroundColor: color.brand[600],
  },
  checkboxTick: {
    color: '#FFFFFF',
    fontSize: 12,
    fontWeight: '700',
  },
  checkboxLabel: {
    flex: 1,
    fontSize: 13,
    lineHeight: 18,
    color: color.neutral[600],
  },
  linkText: {
    color: color.brand[600],
    fontWeight: '600',
  },
  errorText: {
    fontSize: 12,
    color: color.danger[600],
    fontWeight: '500',
  },
  footer: {
    marginTop: space[40],
    gap: space[16],
  },
  submitButton: {
    width: '100%',
  },
  loginLink: {
    alignItems: 'center',
    paddingVertical: space[8],
  },
  footerText: {
    fontSize: 13,
    color: color.neutral[600],
  },
  linkBold: {
    color: color.brand[600],
    fontWeight: '700',
  },
});
