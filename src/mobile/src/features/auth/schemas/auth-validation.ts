import { z } from 'zod';

/**
 * Shared validation rules for authentication forms
 */

// Permissive Vietnamese phone validation (accepts 9 digits without 0, or 10 digits with 0)
export const phoneValidationSchema = z
  .string()
  .regex(/^(0?[35789]\d{8})$/, 'Invalid phone number (e.g. 867587497)');

export const loginSchema = z.object({
  phone: phoneValidationSchema,
  password: z.string().min(1, 'Password is required'),
});

export const registerSchema = z.object({
  fullName: z
    .string()
    .min(2, 'Full name must be at least 2 characters')
    .max(120, 'Full name is too long'),
  phone: phoneValidationSchema,
  password: z
    .string()
    .min(8, 'Password must be at least 8 characters')
    .max(72, 'Password must be at most 72 characters'),
  agree: z.boolean().refine((val) => val === true, {
    message: 'You must accept the terms of use',
  }),
});

export type LoginFormData = z.infer<typeof loginSchema>;
export type RegisterFormData = z.infer<typeof registerSchema>;
