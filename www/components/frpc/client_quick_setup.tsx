'use client'

import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useStore } from '@nanostores/react'
import { nanoid } from 'nanoid'
import { toast } from 'sonner'
import { $userInfo } from '@/store/user'
import { $proxyTableRefetchTrigger } from '@/store/refetch-trigger'
import { createProxyConfig } from '@/api/proxy'
import { buildQuickProxy } from '@/lib/quick-proxy'
import { automaticProxyName } from '@/lib/proxy-name'
import { ObjToUint8Array } from '@/lib/utils'
import { ServerSelector } from '../base/server-selector'
import { ProxyConfigMutateDialog } from '../proxy/mutate_proxy_config'
import { Button } from '../ui/button'
import { Input } from '../ui/input'

type Purpose = 'http' | 'socks5' | 'tcp'
const purposes: { value: Purpose; label: string }[] = [
  { value: 'http', label: 'HTTP 代理' },
  { value: 'socks5', label: 'SOCKS5 代理' },
  { value: 'tcp', label: 'TCP 映射' },
]

export function ClientQuickSetup({ clientID, serverID }: { clientID: string; serverID?: string }) {
  const [purpose, setPurpose] = useState<Purpose>()
  const [pending, setPending] = useState(false)
  return <section className="rounded-xl border bg-card p-4 space-y-4" aria-label="快捷添加">
    <div className="flex flex-wrap items-center gap-3">
      <h2 className="text-sm font-medium mr-1">快捷添加</h2>
      {purposes.map(item => <Button key={item.value} type="button" variant={purpose === item.value ? 'default' : 'outline'} disabled={pending} aria-pressed={purpose === item.value} onClick={() => setPurpose(item.value)}>{item.label}</Button>)}
      <ProxyConfigMutateDialog fullConfiguration defaultClientID={clientID} defaultServerID={serverID} triggerLabel="其他设置" />
    </div>
    {purpose && <CompactTunnelForm key={purpose} purpose={purpose} clientID={clientID} initialServerID={serverID} onPendingChange={setPending} onClose={() => setPurpose(undefined)} />}
  </section>
}

function CompactTunnelForm({ purpose, clientID, initialServerID, onPendingChange, onClose }: { purpose: Purpose; clientID: string; initialServerID?: string; onPendingChange: (pending: boolean) => void; onClose: () => void }) {
  const user = useStore($userInfo)
  const [serverID, setServerID] = useState(initialServerID)
  const [port, setPort] = useState(purpose === 'socks5' ? '1080' : '8080')
  const [username, setUsername] = useState('user')
  const [password, setPassword] = useState('')
  const [showPassword, setShowPassword] = useState(false)
  const [samePort, setSamePort] = useState(true)
  const [remotePort, setRemotePort] = useState('8080')
  const [localIP, setLocalIP] = useState('127.0.0.1')
  const [suffix] = useState(() => nanoid(6))
  const tcp = purpose === 'tcp'
  const validPort = (value: string) => /^\d+$/.test(value) && Number(value) > 0 && Number(value) <= 65535
  const valid = !!user?.userName && !!serverID && validPort(port) && (tcp ? !!localIP.trim() && (samePort || validPort(remotePort)) : !!username.trim() && !!password)
  const mutation = useMutation({
    mutationFn: async () => {
      if (!valid) throw new Error('请填写服务端、有效端口及认证信息')
      const config = buildQuickProxy({ purpose, existing: false, port, samePort, remotePort, localIP, username, password, name: '', suffix })
      config.name = automaticProxyName(user!.userName!, clientID, serverID!, config, suffix)
      return createProxyConfig({ clientId: clientID, serverId: serverID!, config: ObjToUint8Array({ proxies: [config] }), overwrite: false })
    },
    onSuccess: () => { $proxyTableRefetchTrigger.set(Math.random()); toast.success('已添加，可在下方隧道列表查看'); onClose() },
    onError: (error: Error) => toast.error(error.message || '添加失败，请重试'),
    onSettled: () => onPendingChange(false),
  })
  return <form className="border-t pt-4 space-y-4" aria-label={`${purpose.toUpperCase()} 快捷添加`} onSubmit={event => { event.preventDefault(); if (valid && !mutation.isPending) { onPendingChange(true); mutation.mutate() } }}>
    <fieldset disabled={mutation.isPending} className="space-y-4">
      {!initialServerID && <div className="max-w-sm space-y-2"><p className="text-sm font-medium">使用服务端</p><ServerSelector serverID={serverID} setServerID={setServerID} /></div>}
      <div className={`grid gap-3 ${tcp ? 'sm:max-w-sm' : 'sm:grid-cols-3'}`}>
        <label className="grid gap-2 text-sm">{tcp ? '本机端口' : '服务器端口'}<Input type="number" min={1} max={65535} required value={port} onChange={e => setPort(e.target.value)} /></label>
        {!tcp && <>
          <label className="grid gap-2 text-sm">用户名<Input required autoComplete="off" value={username} onChange={e => setUsername(e.target.value)} /></label>
          <label className="grid gap-2 text-sm">密码<Input required type={showPassword ? 'text' : 'password'} autoComplete="new-password" value={password} onChange={e => setPassword(e.target.value)} /></label>
        </>}
      </div>
      {tcp ? <>
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={samePort} onChange={e => { setSamePort(e.target.checked); if (!e.target.checked) setRemotePort(port) }} />服务器端口与本机相同</label>
        {!samePort && <label className="grid max-w-sm gap-2 text-sm">服务器端口<Input type="number" min={1} max={65535} required value={remotePort} onChange={e => setRemotePort(e.target.value)} /></label>}
        <details className="text-sm"><summary className="cursor-pointer text-muted-foreground">目标地址：{localIP}</summary><label className="mt-3 grid max-w-sm gap-2">本机或局域网地址<Input value={localIP} onChange={e => setLocalIP(e.target.value)} /></label></details>
        <p className="text-xs text-muted-foreground">适用于网站或其他 TCP 服务，登录验证由原服务提供。</p>
      </> : <div className="flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
        <span>使用客户端内置代理，无需安装额外程序。</span>
        <label className="flex items-center gap-2"><input type="checkbox" checked={showPassword} onChange={e => setShowPassword(e.target.checked)} />显示密码</label>
        <button type="button" className="text-primary hover:underline" onClick={() => { setPassword(nanoid(20)); setShowPassword(true) }}>生成密码</button>
      </div>}
    </fieldset>
    <div className="flex gap-2"><Button type="submit" disabled={!valid || mutation.isPending}>{mutation.isPending ? '正在添加…' : '添加'}</Button><Button type="button" variant="ghost" disabled={mutation.isPending} onClick={onClose}>取消</Button></div>
  </form>
}
