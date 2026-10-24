import test from 'node:test'
import assert from 'node:assert/strict'
import { z } from 'zod'
import { zodResolver } from '@hookform/resolvers/zod'
import { ZodPortSchema, ZodStringSchema, ZodEmailSchema } from '../lib/form-schemas.ts'

test('secure Zod version preserves numeric form coercion and port limits', () => {
  assert.equal(ZodPortSchema.parse('9080'), 9080)
  for (const value of ['', '0', '65536', 'abc']) assert.equal(ZodPortSchema.safeParse(value).success, false)
  assert.equal(ZodPortSchema.safeParse('0').error.issues[0].message, 'validation.portRange.min')
  assert.equal(ZodStringSchema.safeParse('').success, false)
  assert.equal(ZodEmailSchema.safeParse('invalid').success, false)
  assert.equal(ZodEmailSchema.parse('ig01@example.com'), 'ig01@example.com')
})

test('form resolver returns converted values and field errors with Zod 4', async () => {
  const schema = z.object({ port: ZodPortSchema, name: ZodStringSchema })
  const resolve = zodResolver(schema)
  const options = { fields: {}, shouldUseNativeValidation: false }
  assert.deepEqual(await resolve({ port: '9080', name: 'site' }, {}, options), { values: { port: 9080, name: 'site' }, errors: {} })
  const result = await resolve({ port: '65536', name: '' }, {}, options)
  assert.equal(result.errors.port.message, 'validation.portRange.max')
  assert.equal(result.errors.name.message, 'validation.required')
})
