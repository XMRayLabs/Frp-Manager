import type { TypedClientPluginOptions, WebsiteAccount } from '../types/plugin.ts'

export function websiteAccountsValid(accounts: WebsiteAccount[]): boolean {
  const encoder = new TextEncoder()
  const names = new Set<string>()
  return accounts.length > 0 && accounts.length <= 32 && accounts.every(account => {
    const name = account.username
    if (!name || name !== name.trim() || encoder.encode(name).length > 64 || /[:\x00-\x1f\x7f]/.test(name) || names.has(name)) return false
    names.add(name)
    const length = encoder.encode(account.password || '').length
    return length > 0 ? length >= 8 && length <= 72 : !!account.passwordHash
  })
}
export function websitePlugin(localIP: string, localPort: number, accounts: WebsiteAccount[]): TypedClientPluginOptions {
  if (!Number.isInteger(localPort) || localPort < 1 || localPort > 65535 || !localIP.trim()) throw new Error('请填写有效的本机地址和端口')
  if (!websiteAccountsValid(accounts)) throw new Error('请填写不重复的用户名和 8 至 72 字节的密码')
  const host = localIP.trim().replace(/^\[|\]$/g, '')
  return { type: 'fm_http_auth', localAddr: `${host.includes(':') ? `[${host}]` : host}:${localPort}`, accounts: accounts.map(account => ({ ...account })) }
}
export function websiteTarget(localAddr: string): {localIP: string; localPort: number} | undefined {
  const match = /^(?:\[([^\]]+)\]|([^:]+)):(\d+)$/.exec(localAddr)
  if (!match) return undefined
  return { localIP: match[1] || match[2], localPort: Number(match[3]) }
}
