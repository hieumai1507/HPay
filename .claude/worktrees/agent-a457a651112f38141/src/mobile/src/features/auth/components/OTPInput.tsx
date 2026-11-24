import React, { useRef, useState, useEffect } from 'react';
import { StyleSheet, TextInput, View } from 'react-native';

import { color, radius, space } from '@/src/ui/tokens';

interface OTPInputProps {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  secureTextEntry?: boolean;
}

export function OTPInput({ value, onChange, disabled = false, secureTextEntry = false }: OTPInputProps) {
  const inputsRef = useRef<(TextInput | null)[]>([]);
  const [digits, setDigits] = useState<string[]>(Array(6).fill(''));

  // Keep internal digits state in sync with external value
  useEffect(() => {
    const newDigits = Array(6).fill('');
    for (let i = 0; i < Math.min(value.length, 6); i++) {
      newDigits[i] = value[i];
    }
    setDigits(newDigits);
  }, [value]);

  const handleChangeText = (text: string, index: number) => {
    if (disabled) return;

    // Handle paste of whole string or multiple digits
    const cleanedText = text.replace(/[^0-9]/g, '');
    if (cleanedText.length > 1) {
      const pasteData = cleanedText.slice(0, 6);
      const newDigits = Array(6).fill('');
      for (let i = 0; i < pasteData.length; i++) {
        newDigits[i] = pasteData[i];
      }
      setDigits(newDigits);
      const combined = newDigits.join('');
      onChange(combined);

      // Focus last character's box or the next empty box
      const targetIndex = Math.min(pasteData.length, 5);
      inputsRef.current[targetIndex]?.focus();
      return;
    }

    // Single character input
    const newDigits = [...digits];
    newDigits[index] = cleanedText;
    setDigits(newDigits);
    onChange(newDigits.join(''));

    if (cleanedText.length > 0 && index < 5) {
      inputsRef.current[index + 1]?.focus();
    }
  };

  const handleKeyPress = (e: any, index: number) => {
    if (disabled) return;

    if (e.nativeEvent.key === 'Backspace') {
      if (digits[index] === '' && index > 0) {
        // Move back and clear that cell
        const newDigits = [...digits];
        newDigits[index - 1] = '';
        setDigits(newDigits);
        onChange(newDigits.join(''));
        inputsRef.current[index - 1]?.focus();
      } else {
        // Clear current cell
        const newDigits = [...digits];
        newDigits[index] = '';
        setDigits(newDigits);
        onChange(newDigits.join(''));
      }
    }
  };

  return (
    <View style={styles.container}>
      {Array(6)
        .fill(0)
        .map((_, i) => {
          const isActive = value.length === i;
          return (
            <TextInput
              key={i}
              ref={(ref) => {
                inputsRef.current[i] = ref;
              }}
              value={digits[i]}
              onChangeText={(text) => handleChangeText(text, i)}
              onKeyPress={(e) => handleKeyPress(e, i)}
              keyboardType="number-pad"
              maxLength={2} // Allows double character paste handling
              editable={!disabled}
              secureTextEntry={secureTextEntry}
              selectTextOnFocus
              style={[
                styles.box,
                isActive && styles.boxActive,
                digits[i] !== '' && styles.boxFilled,
                disabled && styles.boxDisabled,
              ]}
              textAlign="center"
              accessibilityLabel={`OTP digit ${i + 1}`}
            />
          );
        })}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    gap: space[8],
    width: '100%',
    maxWidth: 340,
    alignSelf: 'center',
  },
  box: {
    width: 48,
    height: 56,
    borderWidth: 1.5,
    borderColor: color.neutral[300],
    borderRadius: radius.sm,
    fontSize: 22,
    fontWeight: '600',
    color: color.neutral[900],
    backgroundColor: '#FFFFFF',
  },
  boxActive: {
    borderColor: color.brand[600],
    shadowColor: color.brand[600],
    shadowOffset: { width: 0, height: 0 },
    shadowOpacity: 0.15,
    shadowRadius: 4,
    elevation: 2,
  },
  boxFilled: {
    borderColor: color.neutral[800],
  },
  boxDisabled: {
    backgroundColor: color.neutral[100],
    color: color.neutral[400],
  },
});
