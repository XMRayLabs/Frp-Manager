import test from 'node:test'
import assert from 'node:assert/strict'
import { NeedUpgrade, normalizeClientVersion } from '../config/notify.ts'

test('legacy main builds always need upgrade', () => {
  assert.equal(NeedUpgrade({ gitVersion: 'main' }, 'v1.0.1'), true)
  assert.equal(NeedUpgrade({ gitVersion: ' MAIN ' }, undefined), true)
  assert.equal(NeedUpgrade({ gitVersion: 'v1.0.1' }, 'v1.0.1'), false)
  assert.equal(NeedUpgrade({ gitVersion: 'v1.0.2' }, 'v1.0.1'), false)
})


test('ARMv7 architecture suffix does not change the public version', () => {
  assert.equal(normalizeClientVersion('1.1.0-armv7-test'), '1.1.0')
  assert.equal(normalizeClientVersion('1.1.0-armv7'), '1.1.0')
  assert.equal(normalizeClientVersion('1.1.0-rc.1'), '1.1.0-rc.1')
  assert.equal(NeedUpgrade({ gitVersion: '1.1.0-armv7-test' }, '1.1.0'), false)
  assert.equal(NeedUpgrade({ gitVersion: '1.0.6-armv7-test' }, '1.1.0'), true)
})
