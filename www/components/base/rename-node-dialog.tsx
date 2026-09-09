import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import http from '@/api/http'
import { API_PATH } from '@/lib/consts'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../ui/dialog'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { $clientTableRefetchTrigger, $serverTableRefetchTrigger, $proxyTableRefetchTrigger } from '@/store/refetch-trigger'
import { toast } from 'sonner'

export function RenameNodeDialog({ kind, id, open, onOpenChange }: { kind: 'client' | 'server'; id: string; open: boolean; onOpenChange: (open: boolean) => void }) {
  const [newID, setNewID] = useState(id)
  const cache = useQueryClient()
  useEffect(() => { if (open) setNewID(id) }, [open, id])
  const valid = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/.test(newID.trim()) && newID.trim() !== id
  const mutation = useMutation({
    mutationFn: () => http.post(`${API_PATH}/${kind}/rename`, { id, newId: newID.trim() }),
    onSuccess: () => {
      $clientTableRefetchTrigger.set(Math.random())
      $serverTableRefetchTrigger.set(Math.random())
      $proxyTableRefetchTrigger.set(Math.random())
      void cache.invalidateQueries()
      toast.success('ID 已修改，设备可继续使用原有身份连接')
      onOpenChange(false)
    },
    onError: (error: Error) => toast.error(error.message || '修改 ID 失败'),
  })
  return <Dialog open={open} onOpenChange={value => { if (!mutation.isPending) onOpenChange(value) }}>
    <DialogContent>
      <DialogHeader><DialogTitle>修改{kind === 'client' ? '客户端' : '服务端'} ID</DialogTitle><DialogDescription>修改实际节点 ID，已有连接和隧道关联会自动保留，无需重新接入设备。原 ID 将保留为连接兼容入口，不能分配给其他节点。</DialogDescription></DialogHeader>
      <p className="text-sm text-muted-foreground break-all">当前 ID：{id}</p>
      <form className="space-y-4" onSubmit={e => { e.preventDefault(); if (valid && !mutation.isPending) mutation.mutate() }}>
        <label className="grid gap-2 text-sm">新 ID<Input value={newID} maxLength={128} onChange={e => setNewID(e.target.value)} disabled={mutation.isPending} autoComplete="off" /></label>
        <p className="text-xs text-muted-foreground">字母或数字开头，可包含点、横线和下划线，最多 128 字符。建议保留所属用户前缀。</p>
        <div className="flex justify-end gap-2"><Button type="button" variant="outline" disabled={mutation.isPending} onClick={() => onOpenChange(false)}>取消</Button><Button type="submit" disabled={!valid || mutation.isPending}>{mutation.isPending ? '正在修改…' : '保存新 ID'}</Button></div>
      </form>
    </DialogContent>
  </Dialog>
}
