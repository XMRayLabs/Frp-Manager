import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { JoinCommandStr } from '@/lib/consts'
import { useStore } from '@nanostores/react'
import { $platformInfo } from '@/store/user'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { signToken } from '@/api/user'
import { toast } from 'sonner'
import { RespCode } from '@/lib/pb/common'

export const ClientJoinButton = ({ role = 'client' }: { role?: 'client' | 'server' }) => {
  const platformInfo = useStore($platformInfo)
  const [joinToken, setJoinToken] = useState<string>()
  const [busy, setBusy] = useState(false)
  const label = role === 'server' ? '服务端' : '客户端'
  const command = platformInfo && joinToken ? JoinCommandStr(platformInfo, joinToken, undefined, undefined, true, role) : ''

  const generate = async () => {
    if (busy) return
    setBusy(true)
    try {
      const response = await signToken({
        expiresIn: BigInt(86400),
        permissions: [
          { method: 'POST', path: `^/api/v1/${role}/get$` },
          { method: 'POST', path: `^/api/v1/${role}/init$` },
        ],
      })
      if (response?.status?.code !== RespCode.SUCCESS || !response.token) throw new Error(response?.status?.message || '生成接入令牌失败')
      setJoinToken(response.token)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '生成接入令牌失败')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Popover onOpenChange={(open) => { if (open && !joinToken) void generate() }}>
      <PopoverTrigger asChild>
        <Button variant="outline" disabled={!platformInfo}>自动接入{label}</Button>
      </PopoverTrigger>
      <PopoverContent className="w-[32rem] max-w-[95vw]">
        <div className="grid gap-4">
          <h4 className="font-medium">{label}自动接入</h4>
          <p className="text-sm text-muted-foreground">
            下载内核后运行下方命令，即可自动注册并连接面板。身份自动保存，重启不重复注册；接入令牌 24 小时有效。
            {role === 'server' && ' 接入后请在列表中检查并修改服务端公网地址。'}
            <a className="text-blue-500 ml-1" href="https://github.com/XMRayLabs/Frp-Manager/releases" target="_blank" rel="noopener noreferrer">下载内核</a>
          </p>
          {command && <pre className="bg-muted p-3 rounded-md font-mono text-sm overflow-x-auto whitespace-pre-wrap break-all">{command}</pre>}
          <Button size="sm" variant="outline" disabled={!command} onClick={async () => {
            try { await navigator.clipboard.writeText(command); toast.success('已复制接入命令') }
            catch { toast.error('复制失败，请手动复制命令') }
          }}>复制接入命令</Button>
          <Button size="sm" variant="outline" disabled={busy || !platformInfo} onClick={generate}>{busy ? '正在生成…' : '重新生成令牌'}</Button>
          <p className="text-xs text-muted-foreground">每次启动内核默认检查更新。该命令在前台运行；需要开机自启时，可使用 install 和 start 命令安装系统服务。</p>
        </div>
      </PopoverContent>
    </Popover>
  )
}
