import test from 'node:test'
import assert from 'node:assert/strict'
import { buildWebsiteProxy, websiteEntry, websiteSubdomain, websiteURL, validWebsiteLabel } from '../lib/website-mapping.ts'
const config = JSON.stringify({ subDomainHost: 'frp.example.com', vhostHTTPPort: 9080 })
const options = { name: 'qwen', owner: 'ig01', localIP: '127.0.0.1', port: '3000', suffix: 'unique', baseURL: 'https://frp.example.com', serverConfig: config }
test('website entry is optional and requires configured domain and HTTP port', () => {
  assert.equal(websiteEntry(undefined, config), undefined)
  assert.equal(websiteEntry('', config), undefined)
  for (const bad of ['https://other.example.com', 'https://frp.example.com/web', 'https://u:p@frp.example.com', 'https://frp.example.com?x=1']) assert.equal(websiteEntry(bad, config), undefined)
  for (const port of [0, -1, 65536, '9080']) assert.equal(websiteEntry(options.baseURL, JSON.stringify({subDomainHost:'frp.example.com',vhostHTTPPort:port})), undefined)
  assert.deepEqual(websiteEntry(options.baseURL, config), { baseURL: options.baseURL, domain:'frp.example.com' })
})
test('one public entry produces independent names without allocating public ports', () => {
  const p = buildWebsiteProxy(options)
  assert.equal(p.type, 'http'); assert.equal(p.subdomain, 'qwen-ig01'); assert.equal(p.localPort, 3000)
  assert.equal('remotePort' in p, false); assert.equal(p.plugin, undefined)
  assert.equal(p.annotations['frp-manager.website'], 'true')
  assert.equal(websiteURL(options.baseURL, p.subdomain), 'https://qwen-ig01.frp.example.com')
  assert.equal(websiteURL('https://frp.example.com:8443', p.subdomain), 'https://qwen-ig01.frp.example.com:8443')
  assert.notEqual(websiteSubdomain('qwen','ig01'), websiteSubdomain('qwen','ig02'))
})
test('invalid names, empty local target and out-of-range ports cannot be submitted', () => {
  for (const name of ['-qwen','qwen-','qwen.test','qwen/../../api','a'.repeat(64),'中文','']) assert.equal(validWebsiteLabel(name), false)
  for (const change of [{name:'qwen.test'},{name:'a'.repeat(60)},{port:'0'},{port:'65536'},{localIP:''},{baseURL:''}]) assert.throws(() => buildWebsiteProxy({...options,...change}))
})
test('website password is independently optional and supports multiple accounts', () => {
  const p = buildWebsiteProxy({...options, accounts:[{username:'alice',password:'first-secret'},{username:'bob',password:'second-secret'}]})
  assert.equal(p.type,'http'); assert.equal(p.plugin.type,'fm_http_auth'); assert.equal(p.plugin.accounts.length,2)
  assert.equal(p.plugin.localAddr,'127.0.0.1:3000')
})
