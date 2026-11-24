import { z } from 'zod';

/**
 * Shared validation rules for authentication forms
 */

// Permissive Vietnamese phone validation (accepts 9 digits without 0, or 10 digits with 0)
export const phoneValidationSchema = z
  .string()
  .regex(/^(0?[35789]\d{8})$/, 'Số điện thoại không hợp lệ (ví dụ: 867587497)');

export const loginSchema = z.object({
  phone: phoneValidationSchema,
  password: z.string().min(1, 'Mật khẩu không được để trống'),
});

export const registerSchema = z.object({
  fullName: z
    .string()
    .min(2, 'Họ tên phải có ít nhất 2 ký tự')
    .max(120, 'Họ tên quá dài'),
  phone: phoneValidationSchema,
  password: z
    .string()
    .min(8, 'Mật khẩu phải có ít nhất 8 ký tự')
    .max(72, 'Mật khẩu không được quá 72 ký tự'),
  agree: z.boolean().refine((val) => val === true, {
    message: 'Bạn phải đồng ý với điều khoản sử dụng',
  }),
});

export type LoginFormData = z.infer<typeof loginSchema>;
export type RegisterFormData = z.infer<typeof registerSchema>;
