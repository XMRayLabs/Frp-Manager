'use client'

import { useEffect, useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'next/navigation'
import { getClient } from '@/api/client'
import { RespCode, Server } from '@/lib/pb/common'
import { ClientConfig } from '@/types/client'
import { TypedProxyConfig } from '@/types/proxy'
import { ClientSelector } from '../base/client-selector'
import { ServerSelector } from '../base/server-selector'
import { SuggestiveInput } from '../base/suggestive-input'
import { Button } from '../ui/button'
import { FRPCForm } from './frpc_form'
import { FRPCEditor } from './frpc_editor'
import { ProxyConfigList } from '../proxy/proxy_config_list'
import { ClientQuickSetup } from './client_quick_setup'

export interface FRPCFormCardProps { clientID?: string; serverID?: string }

export function FRPCFormCard({ clientID: defaultClientID, serverID: defaultServerID }: FRPCFormCardProps) {
  const params = useSearchParams()
  const [clientID, setClientID] = useState<string>()
  const [serverID, setServerID] = useState<string>()
  const [mode, setMode] = useState<'tunnels' | 'connection' | 'json'>('tunnels')
  const [clientProxyConfigs, setClientProxyConfigs] = useState<TypedProxyConfig[]>([])
  const [frpsUrl, setFrpsUrl] = useState('')
  const [selectedServer, setSelectedServer] = useState<Server>()
  const paramClientID = params.get('clientID')
  useEffect(() => { setClientID(defaultClientID || paramClientID || undefined); setServerID(defaultServerID) }, [defaultClientID, defaultServerID, paramClientID])
  const query = useQuery({
    queryKey: ['getClient', clientID, serverID],
    queryFn: async () => {
      const result = await getClient({ clientId: clientID, serverId: serverID })
      if (result.status?.code !== RespCode.SUCCESS) throw new Error(result.status?.message || '读取配置失败')
      return result
    },
    enabled: !!clientID && !!serverID && mode !== 'tunnels',
    retry: false,
    refetchOnWindowFocus: false,
  })
  const parsed = useMemo(() => {
    try { const config = JSON.parse(query.data?.client?.config || '{}'); if (!config || typeof config !== 'object' || Array.isArray(config)) throw new Error('Invalid config'); return { config: config as ClientConfig, error: '' } }
    catch { return { config: {} as ClientConfig, error: '现有配置格式有误，请使用 JSON 编辑修复。' } }
  }, [query.data])
  useEffect(() => {
    setClientProxyConfigs(parsed.config.proxies || [])
    setFrpsUrl(query.data?.client?.frpsUrl || '')
  }, [parsed.config, query.data])
  const formProps = { clientID: clientID!, serverID: serverID!, client: query.data?.client, clientConfig: parsed.config, refetchClient: query.refetch, clientProxyConfigs, setClientProxyConfigs, frpsUrl }
  return (
    <div className="space-y-5">
      <div className="grid gap-4 rounded-xl border bg-card p-4 sm:grid-cols-2">
        <div className="space-y-2"><p className="text-sm font-medium">当前客户端</p><ClientSelector clientID={clientID} setClientID={(id) => { setClientID(id); setServerID(undefined) }} /></div>
        <div className="space-y-2"><p className="text-sm font-medium">{mode === 'tunnels' ? '所属服务端（可选筛选）' : '要修改连接的服务端'}</p><div className="flex gap-2"><ServerSelector serverID={serverID} setServerID={setServerID} setServer={setSelectedServer} />{mode === 'tunnels' && serverID && <Button variant="outline" onClick={() => setServerID(undefined)}>全部</Button>}</div></div>
      </div>
      <div className="flex flex-wrap gap-2" aria-label="客户端配置方式">
        <Button variant={mode === 'tunnels' ? 'default' : 'outline'} onClick={() => setMode('tunnels')}>隧道配置</Button>
        <Button variant={mode === 'connection' ? 'default' : 'outline'} onClick={() => setMode('connection')}>连接与批量配置</Button>
        <Button variant={mode === 'json' ? 'default' : 'outline'} onClick={() => setMode('json')}>JSON 编辑</Button>
      </div>
      {!clientID ? <p className="text-sm text-muted-foreground">请选择客户端。</p> : mode === 'tunnels' ? <>
        <ClientQuickSetup key={`${clientID}-${serverID}`} clientID={clientID} serverID={serverID} />
        <h2 className="text-sm font-medium">已添加的隧道</h2>
        <ProxyConfigList ProxyConfigs={[]} ClientID={clientID} ServerID={serverID} />
      </> : !serverID ? <p className="rounded-lg border p-4 text-sm text-muted-foreground">先选择服务端，再读取和修改该客户端与它之间的连接配置。添加 HTTP、SOCKS5 或 TCP 隧道请使用“隧道配置”。</p> : query.isPending ? <p>正在读取配置…</p> : query.isError ? <div role="alert">{query.error.message}<Button variant="link" onClick={() => query.refetch()}>重试</Button></div> : <div className="rounded-xl border bg-card p-5 space-y-4">
        <details className="rounded-lg border p-3"><summary className="cursor-pointer text-sm">自定义连接地址（通常无需修改）</summary><div className="mt-3"><SuggestiveInput value={frpsUrl} onChange={setFrpsUrl} suggestions={selectedServer?.frpsUrls || []} /></div></details>
        {parsed.error && <p role="alert" className="text-destructive">{parsed.error}</p>}
        {mode === 'connection' && !parsed.error && <FRPCForm key={`${clientID}-${serverID}`} {...formProps} />}
        {mode === 'json' && <FRPCEditor key={`${clientID}-${serverID}`} {...formProps} />}
      </div>}
    </div>
  )
}
