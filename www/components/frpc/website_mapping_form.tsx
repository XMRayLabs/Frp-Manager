'use client'

import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { useStore } from '@nanostores/react'
import { nanoid } from 'nanoid'
import { toast } from 'sonner'
import { getServer } from '@/api/server'
import { createProxyConfig } from '@/api/proxy'
import { $userInfo } from '@/store/user'
import { $proxyTableRefetchTrigger } from '@/store/refetch-trigger'
import { automaticProxyName } from '@/lib/proxy-name'
import { buildWebsiteProxy, validWebsiteLabel, websiteEntry, websiteSubdomain, websiteURL } from '@/lib/website-mapping'
import { websiteAccountsValid } from '@/lib/website-auth'
import { ObjToUint8Array } from '@/lib/utils'
import { RespCode } from '@/lib/pb/common'
import type { WebsiteAccount } from '@/types/plugin'
import { ServerSelector } from '../base/server-selector'
import { ClientSelector } from '../base/client-selector'
import { WebsiteAuthFields } from './website_auth_fields'
import { Button } from '../ui/button'
import { Input } from '../ui/input'

export function WebsiteMappingForm({ defaultClientID, defaultServerID, onSuccess, onPendingChange, onClose }: { defaultClientID?: string; defaultServerID?: string; onSuccess?: () => void; onPendingChange?: (pending: boolean) => void; onClose?: () => void }) {
  const user = useStore($userInfo)
  const [clientID, setClientID] = useState(defaultClientID)
  const [serverID, setServerID] = useState(defaultServerID)
  const [name, setName] = useState('')
  const [port, setPort] = useState('3000')
  const [localIP, setLocalIP] = useState('127.0.0.1')
  const [auth, setAuth] = useState(false)
  const [accounts, setAccounts] = useState<WebsiteAccount[]>([])
  const [suffix] = useState(() => nanoid(6))
  const [created, setCreated] = useState('')
  const server = useQuery({
    queryKey: ['getServer', serverID],
    enabled: !!serverID,
    queryFn: async () => {
      const response = await getServer({ serverId: serverID })
      if (response.status?.code !== RespCode.SUCCESS) throw new Error(response.status?.message || '读取服务端失败')
      // Other consumers of this query key also cache the complete API response.
      return response
    },
    retry: false,
  })
  const entry = websiteEntry(server.data?.server?.websiteBaseUrl, server.data?.server?.config)
  const subdomain = websiteSubdomain(name, user?.userName || '')
  const validName = validWebsiteLabel(name.trim().toLowerCase()) && validWebsiteLabel(subdomain)
  const address = entry && validName ? websiteURL(entry.baseURL, subdomain) : ''
  const valid = !!clientID && !!serverID && !!entry && validName && /^\d+$/.test(port) && Number(port) > 0 && Number(port) <= 65535 && !!localIP.trim() && (!auth || websiteAccountsValid(accounts))
  const mutation = useMutation({ mutationFn: async () => {
    if (!valid || !entry || !user?.userName) throw new Error('请完整填写网站映射配置')
    const config = buildWebsiteProxy({ name, owner: user.userName, localIP, port, suffix, baseURL: entry.baseURL, serverConfig: server.data?.server?.config || '{}', accounts: auth ? accounts : undefined })
    config.name = automaticProxyName(user.userName, clientID!, serverID!, config, subdomain)
    await createProxyConfig({ clientId: clientID!, serverId: serverID!, config: ObjToUint8Array({ proxies: [config] }), overwrite: false })
    return address
  }, onSuccess: (url) => { setCreated(url); $proxyTableRefetchTrigger.set(Math.random()); toast.success('网站映射已保存，已请求客户端自动应用'); onSuccess?.() }, onError: (e: Error) => toast.error(e.message), onSettled: () => onPendingChange?.(false) })
  return <form className="space-y-4" aria-label="网站映射快捷添加" onSubmit={e => { e.preventDefault(); if (valid && !created && !mutation.isPending) { onPendingChange?.(true); mutation.mutate() } }}>
    <p className="text-sm text-muted-foreground">可选功能：通过独立子域名访问本地 HTTP 网站。服务端需先配置 DNS、证书与网站入口；每个网站无需另开公网端口。</p>
    <fieldset disabled={mutation.isPending || !!created} className="space-y-4">
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2"><p className="text-sm font-medium">使用服务端</p><ServerSelector serverID={serverID} setServerID={setServerID} /></div>
        {!defaultClientID && <div className="space-y-2"><p className="text-sm font-medium">本机所在客户端</p><ClientSelector clientID={clientID} setClientID={setClientID} /></div>}
      </div>
      {serverID && (server.isPending ? <p className="text-sm">正在读取网站入口…</p> : server.isError ? <p role="alert" className="text-sm text-destructive">{server.error.message}</p> : !entry ? <p role="alert" className="rounded-lg border p-3 text-sm">此服务端未启用网站映射。管理员可在“服务端 → 修改配置 → 网站域名映射（可选）”中配置；你仍可使用 TCP 映射。</p> : null)}
      <div className="grid gap-3 sm:grid-cols-2">
        <label className="grid gap-2 text-sm">网站名称<Input required placeholder="例如 qwen" autoComplete="off" maxLength={63} value={name} onChange={e => setName(e.target.value.toLowerCase())} /><span className="text-xs text-muted-foreground">使用英文字母、数字或连字符，自动追加账号后缀。</span></label>
        <label className="grid gap-2 text-sm">本机网站端口<Input required type="number" min={1} max={65535} value={port} onChange={e => setPort(e.target.value)} /></label>
      </div>
      {name && !validName && <p role="alert" className="text-xs text-destructive">名称必须以字母或数字开头、结尾，追加账号后缀后不超过 63 字符。</p>}
      <details className="text-sm"><summary className="cursor-pointer text-muted-foreground">目标地址：{localIP}</summary><label className="mt-3 grid gap-2">本机或局域网地址<Input required value={localIP} onChange={e => setLocalIP(e.target.value)} /></label></details>
      <WebsiteAuthFields minimumVersion="1.1.7" enabled={auth} onEnabledChange={setAuth} accounts={accounts} onAccountsChange={setAccounts} />
    </fieldset>
    <div className="rounded-lg bg-muted p-3 text-sm space-y-2" aria-live="polite"><p className="font-medium">{created ? '已创建 · 访问地址' : '访问地址预览'}</p><p className="break-all font-mono">{created || address || '选择服务端并填写网站名称后自动生成'}</p>{address && <p className="text-muted-foreground">→ {localIP}:{port}</p>}{created && <><p className="text-xs text-muted-foreground">节点在线且 DNS / HTTPS 入口配置正确后即可访问，无需重启面板。</p><Button type="button" variant="outline" size="sm" onClick={() => { navigator.clipboard.writeText(created).then(() => toast.success('访问地址已复制')).catch(() => toast.error('复制失败，请手动复制访问地址')) }}>复制访问地址</Button></>}</div>
    <div className="flex gap-2">{!created && <Button type="submit" disabled={!valid || mutation.isPending}>{mutation.isPending ? '正在保存…' : '创建网站映射'}</Button>}{onClose && <Button type="button" variant="ghost" disabled={mutation.isPending} onClick={onClose}>{created ? '完成' : '取消'}</Button>}</div>
  </form>
}
