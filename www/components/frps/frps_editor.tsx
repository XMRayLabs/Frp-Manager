import { useState } from 'react'
import { Textarea } from '@/components/ui/textarea'
import { Button } from '@/components/ui/button'
import { FRPSFormProps } from './frps_form'
import { useMutation } from '@tanstack/react-query'
import { updateFRPS } from '@/api/frp'
import { RespCode } from '@/lib/pb/common'
import { ObjToUint8Array } from '@/lib/utils'
import { toast } from 'sonner'

export const FRPSEditor: React.FC<FRPSFormProps & { onSaved: () => Promise<unknown> }>  = ({ serverID, server, frpsUrls, onSaved }) => {
  const [value, setValue] = useState(() => { try { return JSON.stringify(JSON.parse(server?.config || '{}'), null, 2) } catch { return server?.config || '{}' } })
  const [comment, setComment] = useState(server?.comment || '')
  const mutation = useMutation({ mutationFn: updateFRPS })
  let error = ''
  try { const parsed = JSON.parse(value); if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) error = '配置必须是 JSON 对象' } catch { error = 'JSON 格式有误，请修正后保存' }
  return <div className="grid gap-4">
    <label className="grid gap-2 text-sm">备注<Textarea value={comment} onChange={e => setComment(e.target.value)} /></label>
    <label className="grid gap-2 text-sm">完整 JSON 配置<Textarea className="min-h-[420px] font-mono" value={value} onChange={e => setValue(e.target.value)} spellCheck={false} /></label>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <Button disabled={!!error || mutation.isPending} onClick={async () => {
      try {
        const response = await mutation.mutateAsync({ serverId: serverID, serverIp: server.ip, config: ObjToUint8Array(JSON.parse(value)), comment, frpsUrls })
        if (response.status?.code !== RespCode.SUCCESS) throw new Error(response.status?.message || '保存失败')
        await onSaved(); toast.success('配置已保存')
      } catch (error) { toast.error(error instanceof Error ? error.message : '保存失败') }
    }}>{mutation.isPending ? '正在保存…' : '保存完整配置'}</Button>
  </div>
}
