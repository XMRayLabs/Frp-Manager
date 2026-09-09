import test from 'node:test'
import assert from 'node:assert/strict'
import { NeedUpgrade } from '../config/notify.ts'

test('legacy main builds always need upgrade', () => {
  assert.equal(NeedUpgrade({ gitVersion: 'main' }, 'v1.0.1'), true)
  assert.equal(NeedUpgrade({ gitVersion: ' MAIN ' }, undefined), true)
  assert.equal(NeedUpgrade({ gitVersion: 'v1.0.1' }, 'v1.0.1'), false)
  assert.equal(NeedUpgrade({ gitVersion: 'v1.0.2' }, 'v1.0.1'), false)
})
