import { organization, LanguageGroup } from '@/api/organization'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { batchCreateUsers } from '@/api/permission'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogTrigger } from '@/components/ui/dialog'
import { toast } from 'sonner'

function increment(value: string, offset: number) {
  return value.replace(/(\d+)$/, (number) => String(Number(number) + offset).padStart(number.length, '0'))
}

export function BatchCreateUsersDialog({ onCreated, languageGroupID }: { onCreated: () => Promise<void>; languageGroupID?: string }) {
  const [selectedGroup,setSelectedGroup] = useState('')
  const groupQuery=useQuery({queryKey:['batchLanguageGroups'],queryFn:()=>organization<{groups:LanguageGroup[]}>('groups'),enabled:!languageGroupID})
  const targetGroup=languageGroupID||selectedGroup
  const [open, setOpen] = useState(false)
  const [username, setUsername] = useState('ig01')
  const [email, setEmail] = useState('ig01@xmray.de')
  const [count, setCount] = useState(30)
  const [busy, setBusy] = useState(false)
  const parts = email.trim().split('@')
  const valid = /\d+$/.test(username.trim()) && parts.length === 2 && /\d+$/.test(parts[0]) && !!parts[1] && Number.isInteger(count) && count >= 1 && count <= 100
  const lastName = increment(username.trim(), count - 1)
  const lastEmail = `${increment(parts[0] || '', count - 1)}@${parts[1] || ''}`
  return (
    <Dialog open={open} onOpenChange={(value) => { if (!busy) setOpen(value) }}>
      <DialogTrigger asChild><Button>批量创建用户</Button></DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-y-auto">
        <DialogHeader><DialogTitle>批量创建用户</DialogTitle><DialogDescription>编号自动递增并保留前导零。创建普通用户，初始密码使用各自邮箱，首次登录必须修改。</DialogDescription></DialogHeader>
        <form className="grid gap-4" onSubmit={async (event) => {
          event.preventDefault()
          if (!valid || busy) return
          setBusy(true)
          try {
            const created = await batchCreateUsers({ username: username.trim(), email: email.trim(), count, language_group_id: targetGroup })
            toast.success(`已创建 ${created.length} 个用户，初始密码为各自邮箱`)
            setOpen(false)
            try { await onCreated() } catch { toast.error('用户已创建，列表刷新失败，请点击刷新') }
          } catch (error) { toast.error(error instanceof Error ? error.message : '批量创建失败') }
          finally { setBusy(false) }
        }}>
          {!languageGroupID && <label className="grid gap-2 text-sm">所属语系<select className="rounded border bg-background p-2" value={selectedGroup} onChange={e=>setSelectedGroup(e.target.value)} required><option value="">请选择语系</option>{groupQuery.data?.groups.map(g=><option value={g.id} key={g.id}>{g.name}</option>)}</select></label>}
          <label className="grid gap-2 text-sm">起始用户名<Input value={username} onChange={(e) => setUsername(e.target.value)} required /></label>
          <label className="grid gap-2 text-sm">起始邮箱<Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required /></label>
          <label className="grid gap-2 text-sm">创建数量<Input type="number" min={1} max={100} value={count} onChange={(e) => setCount(Number(e.target.value))} required /></label>
          {valid && <div className="rounded-md bg-muted p-3 text-sm break-all space-y-1"><p>用户名：{username.trim()} → {lastName}</p><p>邮箱：{email.trim()} → {lastEmail}</p><p>共 {count} 个用户；每个初始密码与邮箱相同。</p></div>}
          <p className="text-xs text-muted-foreground">若有重名或邮箱冲突，整批不创建，不会覆盖已有用户。</p>
          <Button type="submit" disabled={!valid || busy || !targetGroup}>{busy ? '正在创建…' : `创建 ${count} 个用户`}</Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
