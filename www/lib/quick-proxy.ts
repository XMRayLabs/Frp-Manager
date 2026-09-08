import type { TCPProxyConfig } from '../types/proxy'

export type QuickProxyOptions = {
  purpose: 'http' | 'socks5' | 'tcp'
  existing: boolean
  port: string
  samePort: boolean
  remotePort: string
  localIP: string
  username: string
  password: string
  name: string
  suffix: string
}

export function buildQuickProxy(options: QuickProxyOptions): TCPProxyConfig {
  const { purpose, existing, port, samePort, remotePort, localIP, username, password, name, suffix } = options
  const forwarding = purpose === 'tcp' || existing
  const publicPort = forwarding && !samePort ? remotePort : port
  const validPort = (value: string) => /^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 65535
  if (!validPort(publicPort) || (forwarding && (!validPort(port) || !localIP.trim()))) throw new Error('Invalid port or local address')
  if (!forwarding && (!username.trim() || !password)) throw new Error('Proxy credentials are required')
  const config: TCPProxyConfig = { name: name.trim() || `${purpose}-${publicPort}-${suffix}`, type: 'tcp', remotePort: Number(publicPort) }
  if (forwarding) {
    config.localIP = localIP.trim()
    config.localPort = Number(port)
  } else {
    config.plugin = purpose === 'http'
      ? { type: 'http_proxy', httpUser: username.trim(), httpPassword: password }
      : { type: 'socks5', username: username.trim(), password }
  }
  return config
}