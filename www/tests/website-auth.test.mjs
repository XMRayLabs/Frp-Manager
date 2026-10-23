import test from 'node:test'
import assert from 'node:assert/strict'
import { websiteAccountsValid, websitePlugin, websiteTarget } from '../lib/website-auth.ts'

test('editing retains existing hashes and allows adding and deleting accounts', () => {
  const old = { username: 'old', passwordHash: '$2a$10$existing-hash' }
  const added = { username: 'new', password: 'new-secret' }
  assert.equal(websiteAccountsValid([old, added]), true)
  assert.equal(websiteAccountsValid([added]), true)
  assert.equal(websiteAccountsValid([]), false)
  assert.deepEqual(websitePlugin('127.0.0.1', 8080, [old]).accounts, [old])
  assert.equal(websiteAccountsValid([{ ...old, password: 'short' }]), false)
})
test('validates distinct users and UTF-8 password byte limits', () => {
  assert.equal(websiteAccountsValid([{ username: 'x:y', password: 'long-secret' }]), false)
  assert.equal(websiteAccountsValid([{ username: ' x', password: 'long-secret' }]), false)
  assert.equal(websiteAccountsValid([{ username: 'x', password: '密'.repeat(25) }]), false)
  assert.equal(websiteAccountsValid([{ username: 'x', password: '密'.repeat(24) }]), true)
  assert.equal(websiteAccountsValid(Array.from({ length: 33 }, (_, i) => ({ username: `u${i}`, password: 'long-secret' }))), false)
})
test('IPv4, domain and IPv6 destinations can be edited without changing ports', () => {
  assert.deepEqual(websiteTarget('127.0.0.1:8080'), { localIP: '127.0.0.1', localPort: 8080 })
  assert.deepEqual(websiteTarget('[::1]:8080'), { localIP: '::1', localPort: 8080 })
  assert.deepEqual(websiteTarget('internal.example:80'), { localIP: 'internal.example', localPort: 80 })
})
