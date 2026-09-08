import test from 'node:test'
import assert from 'node:assert/strict'
import { automaticProxyName } from '../lib/proxy-name.ts'
const name = (config, client = 'admin.c.home', server = 'admin.s.hk', owner = 'admin') => automaticProxyName(owner, client, server, config, 'unique123')
test('public port precedes protocol and current owner prefixes are removed', () => {
  assert.equal(name({ type: 'tcp', remotePort: 9000, localPort: 8080 }), 'admin.home.hk.9000.tcp')
  assert.equal(name({ type: 'udp', remotePort: 9000 }), 'admin.home.hk.9000.udp')
})
test('built-in proxy type is distinguishable and credentials are never included', () => {
  assert.equal(name({ type: 'tcp', remotePort: 8080, plugin: { type: 'http_proxy', httpPassword: 'secret' } }), 'admin.home.hk.8080.http-proxy')
  assert.equal(name({ type: 'tcp', remotePort: 1080, plugin: { type: 'socks5', password: 'secret' } }), 'admin.home.hk.1080.socks5')
})
test('shared node owners remain to avoid collisions', () => {
  assert.notEqual(name({ type: 'tcp', remotePort: 80 }, 'alice.c.home'), name({ type: 'tcp', remotePort: 80 }, 'bob.c.home'))
  assert.equal(name({ type: 'tcp', remotePort: 80 }, 'admin.c.home', 'alice.s.hk'), 'admin.home.alice.s.hk.80.tcp')
})
test('domain/private tunnels without public ports have a generated unique suffix', () => {
  assert.equal(name({ type: 'http', localPort: 80 }), 'admin.home.hk.unique123.http')
  assert.equal(name({ type: 'stcp', localPort: 22 }), 'admin.home.hk.unique123.stcp')
})