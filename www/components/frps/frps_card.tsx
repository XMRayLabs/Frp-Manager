import { useEffect, useState } from 'react'
import { getServer } from '@/api/server'
import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'next/navigation'
import { RespCode } from '@/lib/pb/common'
import { FRPSEditor } from './frps_editor'
import FRPSForm from './frps_form'
import { ServerSelector } from '../base/server-selector'
import StringListInput from '../base/list-input'
import { Button } from '../ui/button'

export interface FRPSFormCardProps { serverID?: string }
export function FRPSFormCard({ serverID: defaultServerID }: FRPSFormCardProps = {}) {
  const params = useSearchParams()
  const paramServerID = params.get('serverID')
  const [serverID, setServerID] = useState<string>()
  const [advanced, setAdvanced] = useState(false)
  const [frpsUrls, setFrpsUrls] = useState<string[]>([])
  useEffect(() => { setServerID(defaultServerID || paramServerID || undefined) }, [defaultServerID, paramServerID])
  const query = useQuery({
    queryKey: ['getServer', serverID],
    queryFn: async () => { const result = await getServer({ serverId: serverID }); if (result.status?.code !== RespCode.SUCCESS) throw new Error(result.status?.message || '读取服务端配置失败'); return result },
    enabled: !!serverID, retry: false, refetchOnWindowFocus: false,
  })
  useEffect(() => { setFrpsUrls(query.data?.server?.frpsUrls || []) }, [query.data])
  let parseError = false
  try { const config = JSON.parse(query.data?.server?.config || '{}'); if (!config || typeof config !== 'object' || Array.isArray(config)) parseError = true } catch { parseError = true }
  return <div className="space-y-4">
    <div className="rounded-xl border bg-card p-4 flex flex-wrap items-center gap-3"><span className="text-sm font-medium">当前服务端</span><ServerSelector serverID={serverID} setServerID={setServerID} /><Button variant={!advanced ? 'default' : 'outline'} onClick={() => setAdvanced(false)}>常用配置</Button><Button variant={advanced ? 'default' : 'outline'} onClick={() => setAdvanced(true)}>JSON 编辑</Button></div>
    {!serverID ? <p>请选择服务端。</p> : query.isPending ? <p>正在读取配置…</p> : query.isError ? <div role="alert">{query.error.message}<Button variant="link" onClick={() => query.refetch()}>重试</Button></div> : query.data?.server && <div className="rounded-xl border bg-card p-5 space-y-4">
      <p className="text-sm text-muted-foreground">通常只需设置公网地址和客户端连接端口（默认 7000）。此处不是网站或代理的映射端口，映射端口在隧道中设置。</p>
      <details className="rounded-lg border p-3"><summary className="cursor-pointer text-sm">自定义连接入口（CDN / WebSocket 等）</summary><div className="mt-3"><StringListInput value={frpsUrls} onChange={setFrpsUrls} placeholder="tcp://example.com:7000" /></div></details>
      {!advanced && parseError ? <p role="alert" className="text-destructive">现有配置格式有误，请切换 JSON 编辑修复。</p> : !advanced ? <FRPSForm key={serverID} serverID={serverID} server={query.data.server} frpsUrls={frpsUrls} /> : <FRPSEditor onSaved={query.refetch} key={serverID} serverID={serverID} server={query.data.server} frpsUrls={frpsUrls} />}
    </div>}
  </div>
}
