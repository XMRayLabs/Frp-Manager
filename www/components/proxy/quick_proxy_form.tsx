'use client'

import { useId, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { nanoid } from 'nanoid'
import { useStore } from '@nanostores/react'
import { $userInfo } from '@/store/user'
import { automaticProxyName } from '@/lib/proxy-name'
import { createProxyConfig } from '@/api/proxy'
import { ObjToUint8Array } from '@/lib/utils'
import { $proxyTableRefetchTrigger } from '@/store/refetch-trigger'
import { Server } from '@/lib/pb/common'
import { buildQuickProxy } from '@/lib/quick-proxy'
import { ServerSelector } from '../base/server-selector'
import { ClientSelector } from '../base/client-selector'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { toast } from 'sonner'

type Purpose = 'http' | 'socks5' | 'tcp'

export function QuickProxyForm({ onSuccess }: { onSuccess?: () => void }) {
  const { i18n } = useTranslation()
  const text = (zh: string, en: string) => i18n.language.startsWith('zh') ? zh : en
  const id = useId()
  const [purpose, setPurpose] = useState<Purpose>('http')
  const [clientId, setClientId] = useState<string>()
  const [serverId, setServerId] = useState<string>()
  const [server, setServer] = useState<Server>()
  const [existing, setExisting] = useState(false)
  const [port, setPort] = useState('8080')
  const [samePort, setSamePort] = useState(true)
  const [remotePort, setRemotePort] = useState('8080')
  const [localIP, setLocalIP] = useState('127.0.0.1')
  const [username, setUsername] = useState('user')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const user = useStore($userInfo)
  const [suffix] = useState(() => nanoid(6))
  const [created, setCreated] = useState(false)
  const forwarding = purpose === 'tcp' || existing
  const publicPort = forwarding && !samePort ? remotePort : port
  const validPort = (value: string) => /^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 65535
  const valid = !!user?.userName && !!clientId && !!serverId && validPort(publicPort) &&
    (forwarding ? validPort(port) && !!localIP.trim() : !!username.trim() && !!password)
  const host = server?.ip || text('服务器地址', 'server address')
  const address = `${host.includes(':') && !host.startsWith('[') ? `[${host}]` : host}:${publicPort || '?'}`
  const mutation = useMutation({
    mutationFn: () => {
      if (!valid) throw new Error(text('请填写节点、有效端口及认证信息', 'Complete the nodes, valid ports and credentials'))
      const config = buildQuickProxy({ purpose, existing, port, samePort, remotePort, localIP, username, password, name: '', suffix })
      config.name = automaticProxyName(user!.userName!, clientId!, serverId!, config, suffix)
      return createProxyConfig({ clientId: clientId!, serverId: serverId!, config: ObjToUint8Array({ proxies: [config] }), overwrite: false })
    },
    onSuccess: () => {
      setCreated(true)
      $proxyTableRefetchTrigger.set(Math.random())
      toast.success(text('配置已创建', 'Configuration created'))
      onSuccess?.()
    },
    onError: () => toast.error(text('创建失败，请检查节点权限及端口是否已占用', 'Creation failed. Check node permissions and port availability')),
  })
  return (
    <form className="space-y-4" onSubmit={event => { event.preventDefault(); if (valid && !mutation.isPending && !created) mutation.mutate() }}>
      <fieldset disabled={mutation.isPending || created} className="space-y-4 disabled:opacity-70">
        <legend className="sr-only">{text('快捷配置', 'Quick setup')}</legend>
        <div className="grid grid-cols-3 gap-2" aria-label={text('选择用途', 'Choose purpose')}>
          {(['http', 'socks5', 'tcp'] as const).map(value => (
            <Button key={value} type="button" variant={purpose === value ? 'default' : 'outline'} aria-pressed={purpose === value}
              onClick={() => { setPurpose(value); setPort(value === 'socks5' ? '1080' : '8080') }}>
              {value === 'http' ? text('HTTP 代理', 'HTTP proxy') : value === 'socks5' ? text('SOCKS5 代理', 'SOCKS5 proxy') : text('TCP 映射', 'TCP mapping')}
            </Button>
          ))}
        </div>
        <p className="text-sm text-muted-foreground">{purpose === 'tcp'
          ? text('把本机网站或其他 TCP 服务映射到公网，无需域名。', 'Expose a local website or TCP service without a domain.')
          : text('通过服务器连接代理，使用所选客户端的网络访问外部资源。', 'Connect through the server and browse using the selected client’s network.')}</p>
        <div className="grid gap-4 sm:grid-cols-2">
          <div className="space-y-2"><p className="text-sm font-medium">{text('公网服务器', 'Public server')}</p><ServerSelector serverID={serverId} setServerID={setServerId} setServer={setServer} /></div>
          <div className="space-y-2"><p className="text-sm font-medium">{text('本机所在客户端', 'Local client')}</p><ClientSelector clientID={clientId} setClientID={setClientId} /></div>
        </div>
        {purpose !== 'tcp' && <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={existing} onChange={e => setExisting(e.target.checked)} />{text('转发已有的本机代理', 'Forward an existing local proxy')}</label>}
        <label className="block space-y-2" htmlFor={`${id}-port`}><span className="text-sm font-medium">{forwarding ? text('本机端口', 'Local port') : text('服务器代理端口', 'Server proxy port')}</span>
          <Input id={`${id}-port`} type="number" min={1} max={65535} required value={port} onChange={e => setPort(e.target.value)} /></label>
        {forwarding && <>
          <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={samePort} onChange={e => { setSamePort(e.target.checked); if (!e.target.checked) setRemotePort(port) }} />{text('服务器端口与本机端口相同', 'Use the same port on the server')}</label>
          {!samePort && <label className="block space-y-2" htmlFor={`${id}-remote`}><span className="text-sm">{text('服务器端口', 'Server port')}</span><Input id={`${id}-remote`} type="number" min={1} max={65535} required value={remotePort} onChange={e => setRemotePort(e.target.value)} /></label>}
          <p className="text-xs text-muted-foreground">{purpose === 'tcp' ? text('TCP 映射不会添加登录密码；网站登录由原网站负责。', 'TCP mapping does not add a password. Website authentication stays with your website.') : text('账号密码在原代理程序中设置，映射会保留原有认证。', 'Set credentials in your existing proxy; forwarding preserves its authentication.')}</p>
        </>}
        {!forwarding && <div className="space-y-3 rounded-lg border p-3">
          <label className="block space-y-2" htmlFor={`${id}-user`}><span className="text-sm">{text('用户名', 'Username')}</span><Input id={`${id}-user`} required autoComplete="off" value={username} onChange={e => setUsername(e.target.value)} /></label>
          <label className="block space-y-2" htmlFor={`${id}-password`}><span className="text-sm">{text('密码', 'Password')}</span><Input id={`${id}-password`} required type={showPassword ? 'text' : 'password'} autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} /></label>
          <div className="flex items-center justify-between gap-2"><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={showPassword} onChange={e => setShowPassword(e.target.checked)} />{text('显示密码', 'Show password')}</label><Button type="button" variant="outline" size="sm" onClick={() => { setPassword(nanoid(20)); setShowPassword(true) }}>{text('生成密码', 'Generate password')}</Button></div>
          <p className="text-xs text-muted-foreground">{text('客户端内置代理，无需额外安装程序或填写本机端口。', 'Built-in proxy: no extra software or local listening port is required.')}</p>
        </div>}
        {forwarding && <details className="rounded-lg border p-3"><summary className="cursor-pointer text-sm">{text('更多选项', 'More options')}</summary><div className="mt-3 space-y-3">
          {forwarding && <label className="block space-y-2" htmlFor={`${id}-ip`}><span className="text-sm">{text('本机 / 局域网地址', 'Local / LAN address')}</span><Input id={`${id}-ip`} value={localIP} onChange={e => setLocalIP(e.target.value)} /></label>}
        </div></details>}
      </fieldset>
      <div className="rounded-lg bg-muted p-3 text-sm space-y-2" aria-live="polite">
        <p className="font-medium">{created ? text('已创建 · 连接信息', 'Created · Connection details') : text('连接预览', 'Connection preview')}</p>
        <p className="break-all font-mono">{purpose === 'tcp' ? address : `${purpose}://${address}`}</p>
        <p className="text-muted-foreground">{forwarding ? `${address} → ${localIP}:${port}` : `${text('用户名', 'Username')}: ${username} · ${text('密码', 'Password')}: ${showPassword ? password : '••••••••'}`}</p>
        {created && <p>{text('请确保服务器防火墙放行该端口，节点在线后可连接。', 'Allow this port through the server firewall and connect when the nodes are online.')}</p>}
      </div>
      {!created && <Button className="w-full" type="submit" disabled={!valid || mutation.isPending}>{mutation.isPending ? text('正在创建…', 'Creating…') : text('创建配置', 'Create configuration')}</Button>}
    </form>
  )
}