import * as z from 'zod'

export const ZodPortSchema = z.coerce
  .number<number>({ error: 'validation.required' })
  .min(1, { message: 'validation.portRange.min' })
  .max(65535, { message: 'validation.portRange.max' })

export const ZodPortOptionalSchema = z.coerce
  .number<number>({ error: 'validation.required' })
  .min(1, { message: 'validation.portRange.min' })
  .max(65535, { message: 'validation.portRange.max' })

export const ZodIPSchema = z
  .string({ error: 'validation.required' })
  .regex(/^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/, { message: 'validation.ipAddress' })
export const ZodStringSchema = z
  .string({ error: 'validation.required' })
  .min(1, { message: 'validation.required' })

export const ZodStringOptionalSchema = z.string().optional()
export const ZodEmailSchema = z
  .string({ error: 'validation.required' })
  .min(1, { message: 'validation.required' })
  .email({ message: 'auth.email.invalid' })

