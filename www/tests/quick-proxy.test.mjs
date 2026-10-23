import test from 'node:test'
import assert from 'node:assert/strict'
import { buildQuickProxy } from '../lib/quick-proxy.ts'
const defaults = { purpose: 'tcp', existing: false, port: '8080', samePort: true, remotePort: '9000', localIP: '127.0.0.1', username: 'user', password: 'secret', name: '', suffix: 'abc123' }
test('TCP same-port mapping does not claim password authentication', () => {
  assert.deepEqual(buildQuickProxy(defaults), { name: 'tcp-8080-abc123', type: 'tcp', remotePort: 8080, localIP: '127.0.0.1', localPort: 8080 })
})
test('different public port keeps the local destination', () => {
  const cfg = buildQuickProxy({ ...defaults, samePort: false, localIP: '192.168.1.20' })
  assert.equal(cfg.remotePort, 9000)
  assert.equal(cfg.localPort, 8080)
  assert.equal(cfg.localIP, '192.168.1.20')
})
test('HTTP and SOCKS use their own plugin credentials and no local backend', () => {
  const http = buildQuickProxy({ ...defaults, purpose: 'http' })
  assert.deepEqual(http.plugin, { type: 'http_proxy', httpUser: 'user', httpPassword: 'secret' })
  assert.equal(http.localPort, undefined)
  const socks = buildQuickProxy({ ...defaults, purpose: 'socks5' })
  assert.deepEqual(socks.plugin, { type: 'socks5', username: 'user', password: 'secret' })
})
test('existing proxy forwards without injecting an unrelated auth plugin', () => {
  const cfg = buildQuickProxy({ ...defaults, purpose: 'socks5', existing: true, password: '' })
  assert.equal(cfg.plugin, undefined)
  assert.equal(cfg.localPort, 8080)
})
test('rejects invalid ports and unauthenticated built-in proxies', () => {
  for (const port of ['', '0', '65536', '1.5', '1e3', '-2']) assert.throws(() => buildQuickProxy({ ...defaults, port }))
  assert.throws(() => buildQuickProxy({ ...defaults, purpose: 'http', password: '' }))
  assert.throws(() => buildQuickProxy({ ...defaults, purpose: 'socks5', username: ' ' }))
})
test('TCP websites preserve ports and use an explicit authentication plugin', () => {
  const accounts = [{ username: 'one', password: 'first-secret' }, { username: 'two', password: 'second-secret' }]
  const cfg = buildQuickProxy({ ...defaults, samePort: false, localIP: '::1', websiteAccounts: accounts })
  assert.equal(cfg.remotePort, 9000)
  assert.deepEqual(cfg.plugin, { type: 'fm_http_auth', localAddr: '[::1]:8080', accounts })
  assert.throws(() => buildQuickProxy({ ...defaults, websiteAccounts: [] }))
  assert.throws(() => buildQuickProxy({ ...defaults, websiteAccounts: [{ username: 'x', password: 'short' }] }))
  assert.throws(() => buildQuickProxy({ ...defaults, websiteAccounts: [{ username: 'x', password: 'first-secret' }, { username: 'x', password: 'second-secret' }] }))
})
