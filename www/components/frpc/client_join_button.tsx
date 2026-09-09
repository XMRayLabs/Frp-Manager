import { Button } from '@/components/ui/button'
import { useState } from 'react'
import { JoinCommandStr } from '@/lib/consts'
import { useStore } from '@nanostores/react'
import { $platformInfo } from '@/store/user'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { AlertDialog, AlertDialogContent, AlertDialogHeader, AlertDialogTitle, AlertDialogDescription, AlertDialogFooter, AlertDialogCancel } from '@/components/ui/alert-dialog'
import { getEnrollmentToken, rotateEnrollmentToken } from '@/api/user'
import { toast } from 'sonner'

export const ClientJoinButton = ({ role = 'client' }: { role?: 'client' | 'server' }) => {
  const platformInfo = useStore($platformInfo)
  const [joinToken, setJoinToken] = useState<string>()
  const [busy, setBusy] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [error, setError] = useState('')
  const label = role === 'server' ? '服务端' : '客户端'
  const command = platformInfo && joinToken ? JoinCommandStr(platformInfo, joinToken, undefined, undefined, true, role) : ''

  const load = async () => {
    setBusy(true)
    setError('')
    try { setJoinToken(await getEnrollmentToken(role)) }
    catch (error) {
      setJoinToken(undefined)
      setError(error instanceof Error ? error.message : '读取接入令牌失败')
    } finally { setBusy(false) }
  }

  const rotate = async () => {
    if (!joinToken || busy) return
    setBusy(true)
    try {
      setJoinToken(await rotateEnrollmentToken(role, joinToken))
      setConfirmOpen(false)
      toast.success('接入令牌已更换，请使用新的接入命令')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '更换接入令牌失败')
    } finally { setBusy(false) }
  }

  return (
    <>
      <Popover onOpenChange={(open) => { if (open) void load() }}>
        <PopoverTrigger asChild>
          <Button variant="outline" disabled={!platformInfo}>自动接入{label}</Button>
        </PopoverTrigger>
        <PopoverContent className="w-[32rem] max-w-[95vw]">
          <div className="grid gap-3">
            <h4 className="font-medium">{label}自动接入</h4>
            <p className="text-sm text-muted-foreground">
              下载内核后运行下方命令，即可自动注册并连接面板。接入令牌长期有效，可重复用于新设备；刷新页面、重启面板都不会改变。
              {role === 'server' && ' 接入后可在列表中修改公网地址。'}
              <a className="text-blue-500 ml-1" href="https://github.com/XMRayLabs/Frp-Manager/releases" target="_blank" rel="noopener noreferrer">下载内核</a>
            </p>
            {busy && <p role="status" className="text-sm text-muted-foreground">正在读取或更新令牌…</p>}
            {error && <div role="alert" className="text-sm text-destructive">{error}<Button variant="link" onClick={load} disabled={busy}>重试</Button></div>}
            {command && <pre className="bg-muted p-3 rounded-md font-mono text-sm overflow-x-auto whitespace-pre-wrap break-all">{command}</pre>}
            <Button size="sm" variant="outline" disabled={!command || busy} onClick={async () => {
              try { await navigator.clipboard.writeText(command); toast.success('已复制接入命令') }
              catch { toast.error('复制失败，请手动复制命令') }
            }}>复制接入命令</Button>
            <details>
              <summary className="cursor-pointer text-xs text-muted-foreground">令牌设置（通常无需修改）</summary>
              <Button className="mt-2" size="sm" variant="outline" disabled={!joinToken || busy} onClick={() => setConfirmOpen(true)}>更换接入令牌</Button>
            </details>
            <p className="text-xs text-muted-foreground">已接入设备保存独立身份，更换接入令牌不会断开它们。每次启动内核默认检查更新；需要开机自启时，使用 install 和 start 安装系统服务。</p>
          </div>
        </PopoverContent>
      </Popover>
      <AlertDialog open={confirmOpen} onOpenChange={(open) => { if (!busy) setConfirmOpen(open) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认更换{label}接入令牌？</AlertDialogTitle>
            <AlertDialogDescription>旧令牌和旧接入命令将立即失效，后续新设备需要使用新命令。已经接入的设备不受影响。通常无需更换。</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={busy}>取消</AlertDialogCancel>
            <Button variant="destructive" disabled={busy} onClick={rotate}>{busy ? '正在更换…' : '确认更换'}</Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
