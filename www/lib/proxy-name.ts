import type { TypedProxyConfig } from '../types/proxy'

// Keep foreign owners in shared node IDs; remove only the current user's prefix.
function nodeName(id: string, owner: string, kind: string): string {
  const prefix = `${owner}.${kind}.`
  return id.startsWith(prefix) ? id.slice(prefix.length) : id
}

export function automaticProxyName(owner: string, clientId: string, serverId: string, config: TypedProxyConfig, fallback: string): string {
  const client = nodeName(clientId, owner, 'c')
  const server = nodeName(serverId, owner, 's')
  const protocol = config.plugin?.type === 'http_proxy' ? 'http-proxy' : config.plugin?.type === 'socks5' ? 'socks5' : config.type
  const endpoint = 'remotePort' in config && config.remotePort ? String(config.remotePort) : fallback
  return `${owner}.${client}.${server}.${endpoint}.${protocol}`
}