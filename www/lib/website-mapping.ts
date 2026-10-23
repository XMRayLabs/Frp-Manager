import type { HTTPProxyConfig } from '../types/proxy'
import type { WebsiteAccount } from '../types/plugin'
import { websitePlugin } from './website-auth.ts'

export const websiteMappingAnnotation = 'frp-manager.website'
const labelPattern = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/
export function validWebsiteLabel(value: string): boolean { return labelPattern.test(value) }
export function websiteSubdomain(name: string, owner: string): string {
  const account = owner.toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/^-+|-+$/g, '')
  return `${name.trim().toLowerCase()}-${account}`
}
export function websiteEntry(base: string | undefined, config: string | undefined): { baseURL: string; domain: string } | undefined {
  if (!base) return undefined
  try {
    const url = new URL(base)
    const server = JSON.parse(config || '{}')
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || (url.pathname !== '/' && url.pathname !== '') || !Number.isInteger(server.vhostHTTPPort) || server.vhostHTTPPort < 1 || server.vhostHTTPPort > 65535 || !url.hostname.includes('.') || url.hostname.split('.').some(label => !labelPattern.test(label)) || url.hostname !== server.subDomainHost?.toLowerCase().replace(/\.$/, '')) return undefined
    return { baseURL: url.origin, domain: url.hostname }
  } catch { return undefined }
}
export function websiteURL(base: string, subdomain: string): string {
  const url = new URL(base); url.hostname = `${subdomain}.${url.hostname}`; return url.origin
}
export function buildWebsiteProxy(options: { name: string; owner: string; localIP: string; port: string; suffix: string; baseURL: string; serverConfig: string; accounts?: WebsiteAccount[] }): HTTPProxyConfig {
  const entry = websiteEntry(options.baseURL, options.serverConfig)
  const subdomain = websiteSubdomain(options.name, options.owner)
  if (!entry || !validWebsiteLabel(options.name.trim().toLowerCase()) || !validWebsiteLabel(subdomain) || !/^\d+$/.test(options.port) || Number(options.port) < 1 || Number(options.port) > 65535 || !options.localIP.trim()) throw new Error('请填写有效的网站名称、本地地址和端口，并选择已启用网站入口的服务端')
  return { name: `website-${subdomain}-${options.suffix}`, type: 'http', localIP: options.localIP.trim(), localPort: Number(options.port), subdomain, annotations: { [websiteMappingAnnotation]: 'true' }, ...(options.accounts ? { plugin: websitePlugin(options.localIP, Number(options.port), options.accounts) } : {}) }
}
