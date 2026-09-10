import { useEffect, useState } from 'react'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { BatchCreateUsersDialog } from '@/components/user/batch-create-users'
import { organization, LanguageGroup, OrganizationContext, Assignment, Audit } from '@/api/organization'
import { listUsers, updateUser, listInvites, createInvite, updateInvite, AdminUser, InviteCode } from '@/api/permission'
import { listServer } from '@/api/server'
import { toast } from 'sonner'

type Confirmation = { title: string; message: string; execute: () => Promise<void> }
function OrganizationPanel() {
  const [context, setContext] = useState<OrganizationContext>()
  const [groups, setGroups] = useState<LanguageGroup[]>([])
  const [assignments, setAssignments] = useState<Assignment[]>([])
  const [users, setUsers] = useState<AdminUser[]>([])
  const [invites, setInvites] = useState<InviteCode[]>([])
  const [audit, setAudit] = useState<Audit[]>([])
  const [servers, setServers] = useState<string[]>([])
  const [selected, setSelected] = useState('')
  const [name, setName] = useState('')
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [keyword, setKeyword] = useState('')
  const [destination, setDestination] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [confirmation, setConfirmation] = useState<Confirmation>()
  const site = context?.role === 'admin'
  const group = groups.find(g => g.id === selected)
  async function reload() {
    const ctx = await organization<OrganizationContext>('context'); setContext(ctx)
    if (!['admin', 'group_admin'].includes(ctx.role)) return
    const [data, accounts, codes, logs] = await Promise.all([
      organization<{ groups: LanguageGroup[]; assignments: Assignment[] }>('groups'), listUsers(), listInvites(), organization<Audit[]>('audit'),
    ])
    setGroups(data.groups ?? []); setAssignments(data.assignments ?? []); setUsers(accounts); setInvites(codes); setAudit(logs ?? [])
    setSelected(old => data.groups?.some(g => g.id === old) ? old : data.groups?.[0]?.id ?? '')
    if (ctx.role === 'admin') { const response = await listServer({ page: 1, pageSize: 1000 }); setServers((response.servers ?? []).map(s => s.id ?? '').filter(Boolean)) }
  }
  useEffect(() => { reload().catch(e => setError(String(e))) }, [])
  async function run(action: () => Promise<unknown>) {
    if (busy) return
    setBusy(true); setError('')
    try { await action(); await reload(); toast.success('已保存') } catch (e) { setError(e instanceof Error ? e.message : String(e)) } finally { setBusy(false) }
  }
  async function preview(path: string, body: object, title: string) {
    try {
      const impact = await organization<{ affected_count: number; affected: { name: string }[] }>(path, body)
      setConfirmation({ title, message: `将停止 ${impact.affected_count} 条隧道并保留配置。重新授权后需要手动启动。${impact.affected?.map(p => p.name).join('、') || '没有受影响的隧道。'}`, execute: async () => { await organization(path, { ...body, confirm: true }) } })
    } catch(e) { setError(String(e)) }
  }
  if (!context) return <p className="p-6">{error || '正在加载语系…'}</p>
  if (!['admin', 'group_admin'].includes(context.role)) return <p className="p-6">此页面仅供管理员使用。</p>
  const members = users.filter(u => u.language_group_id === selected && `${u.user_name} ${u.email}`.toLowerCase().includes(keyword.toLowerCase()))
  return <div className="space-y-5 p-4 md:p-6">
    {error && <p role="alert" className="rounded border border-destructive p-3 text-destructive">{error}</p>}
    <div className="flex flex-wrap items-center gap-3"><label>所属语系 <select aria-label="所属语系" className="rounded border bg-background p-2" value={selected} onChange={e => setSelected(e.target.value)}>{groups.map(g => <option key={g.id} value={g.id}>{g.name}</option>)}</select></label><Button variant="outline" disabled={busy} onClick={() => reload().catch(e => setError(String(e)))}>刷新</Button></div>
    {site && <form className="flex gap-2" onSubmit={e => { e.preventDefault(); void run(() => organization('groups/save', { name })).then(() => setName('')) }}><Input aria-label="新语系名称" placeholder="新语系名称" value={name} onChange={e => setName(e.target.value)} required maxLength={128}/><Button disabled={busy || !name.trim()}>创建语系</Button></form>}
    {group && <>
      <Card><CardHeader><CardTitle>{group.name}</CardTitle>{site && <form className="flex gap-2 pt-2" key={group.id} onSubmit={e=>{e.preventDefault();const data=new FormData(e.currentTarget);void run(()=>organization('groups/save',{id:selected,name:String(data.get('name'))}))}}><Input name="name" aria-label="修改语系名称" defaultValue={group.name} required maxLength={128}/><Button variant="outline" disabled={busy}>修改名称</Button></form>}</CardHeader><CardContent className="space-y-3"><label className="flex items-center gap-2"><input type="checkbox" checked={group.shared} disabled={busy} onChange={e => void run(() => organization('groups/save', { id: selected, name: group.name, shared: e.target.checked }))}/>语系内共享客户端与隧道</label><p className="text-sm text-muted-foreground">关闭时由设备主人和管理员管理。开启后，成员可以配置共享设备的隧道；私有设备不参与共享。设备密钥、终端和升级仍仅限主人及上级管理员。</p>{site && <Button variant="destructive" disabled={busy || users.some(u => u.language_group_id === selected)} onClick={() => setConfirmation({ title:'删除语系', message:`删除「${group.name}」及其服务端授权和邀请码。`, execute:async()=>{await organization('groups/delete',{id:selected})} })}>删除空语系</Button>}</CardContent></Card>
      <Tabs defaultValue="users"><TabsList><TabsTrigger value="users">成员</TabsTrigger><TabsTrigger value="servers">服务端授权</TabsTrigger><TabsTrigger value="invites">邀请注册</TabsTrigger><TabsTrigger value="audit">操作记录</TabsTrigger></TabsList>
        <TabsContent value="users" className="space-y-4">
          <div className="flex gap-3"><Input aria-label="搜索成员" placeholder="搜索用户名或邮箱" value={keyword} onChange={e=>setKeyword(e.target.value)}/><BatchCreateUsersDialog languageGroupID={selected} onCreated={reload}/></div>
          {site && <form className="flex flex-wrap gap-2 rounded border p-3" onSubmit={e=>{e.preventDefault();void run(()=>organization('users/create-admin',{id:selected,username,email}))}}><Input className="w-auto" aria-label="管理员用户名" placeholder="语系管理员用户名" value={username} onChange={e=>setUsername(e.target.value)} required/><Input className="w-auto" aria-label="管理员邮箱" placeholder="邮箱（作为初始密码）" type="email" value={email} onChange={e=>setEmail(e.target.value)} required/><Button disabled={busy}>创建语系管理员</Button></form>}
          {site && <label className="block text-sm">迁移目标 <select className="rounded border bg-background p-2" value={destination} onChange={e=>setDestination(e.target.value)}><option value="">请选择目标语系</option>{groups.filter(g=>g.id!==selected).map(g=><option key={g.id} value={g.id}>{g.name}</option>)}</select></label>}
          <div className="overflow-x-auto"><table className="w-full text-sm"><thead><tr className="border-b text-left"><th className="p-2">用户</th><th>邮箱</th><th>角色 / 状态</th><th>操作</th></tr></thead><tbody>{members.map(u=><tr key={u.user_id} className="border-b"><td className="p-2">{u.user_name}</td><td>{u.email}</td><td>{u.role==='group_admin'?'语系管理员':'普通用户'} · {u.status===2?'已禁用':'正常'}</td><td><div className="flex flex-wrap gap-2 py-2">{(site || u.role==='normal') && <><Button variant="outline" size="sm" disabled={busy} onClick={()=>setConfirmation({title:u.status===2?'启用账号':'禁用账号',message:'禁用只影响面板登录和新设备接入，已有设备与隧道继续运行。',execute:async()=>{await updateUser({user_id:u.user_id,status:u.status===2?1:2})}})}>{u.status===2?'启用':'禁用'}</Button><Button variant="outline" size="sm" onClick={()=>setConfirmation({title:'重置密码',message:`${u.user_name} 的密码将重置为邮箱，旧会话失效，首次登录须改密。`,execute:async()=>{await organization('users/reset-password',{user_id:u.user_id,confirm:true})}})}>重置密码</Button></>}{site && <><Button variant="outline" size="sm" disabled={!destination || busy} onClick={()=>void preview('users/move',{id:destination,user_id:u.user_id},`迁移 ${u.user_name}`)}>迁移语系</Button><Button variant="outline" size="sm" disabled={busy} onClick={()=>setConfirmation({title:'调整管理权限',message:`将 ${u.user_name} 设为${u.role==='group_admin'?'普通用户':'语系管理员'}，旧会话失效。`,execute:async()=>{await updateUser({user_id:u.user_id,role:u.role==='group_admin'?'normal':'group_admin'})}})}>{u.role==='group_admin'?'取消管理':'设为管理员'}</Button></>}</div></td></tr>)}</tbody></table>{!members.length && <p className="p-4 text-muted-foreground">暂无成员</p>}</div>
        </TabsContent>
        <TabsContent value="servers" className="space-y-3"><p className="text-sm text-muted-foreground">服务端由网站管理员统一分配。各语系共享服务器端口池，同一端口不能重复占用。</p>{(site?servers:assignments.filter(a=>a.language_group_id===selected).map(a=>a.server_id)).map(id=>{const assigned=assignments.some(a=>a.language_group_id===selected&&a.server_id===id);return <label key={id} className="flex items-center gap-3 rounded border p-3"><input type="checkbox" checked={assigned} disabled={!site||busy} onChange={e=>{if(e.target.checked)void run(()=>organization('servers/assign',{id:selected,server_id:id,assigned:true}));else void preview('servers/assign',{id:selected,server_id:id,assigned:false},'撤销服务端授权')}}/>{id}</label>})}</TabsContent>
        <TabsContent value="invites" className="space-y-3"><Button disabled={busy} onClick={()=>void run(()=>createInvite({language_group_id:selected,max_uses:1}))}>生成本语系单次邀请码</Button><p className="text-sm text-muted-foreground">使用邀请码注册的用户自动加入 {group.name}。</p>{invites.filter(i=>i.language_group_id===selected).map(i=><div className="flex flex-wrap items-center gap-3 rounded border p-3" key={i.id}><code className="break-all">{i.code}</code><span>{i.used_count}/{i.max_uses} 次</span><Button variant="outline" size="sm" onClick={()=>navigator.clipboard.writeText(i.code).then(()=>toast.success('已复制')).catch(()=>toast.error('复制失败'))}>复制</Button><Button variant="destructive" size="sm" disabled={busy} onClick={()=>void run(()=>updateInvite({id:i.id,disabled:true}))}>撤销</Button></div>)}</TabsContent>
        <TabsContent value="audit"><p className="my-3 text-sm text-muted-foreground">最近 100 条管理操作</p>{audit.map(a=><div className="border-b py-3 text-sm" key={a.id}><time>{new Date(a.created_at).toLocaleString()}</time> · 用户 #{a.actor_id} · {a.action}<p className="break-all text-muted-foreground">{a.target} {a.detail}</p></div>)}</TabsContent>
      </Tabs>
    </>}
    <Dialog open={!!confirmation} onOpenChange={open=>{if(!open&&!busy)setConfirmation(undefined)}}><DialogContent><DialogHeader><DialogTitle>{confirmation?.title}</DialogTitle><DialogDescription className="max-h-64 overflow-auto break-all">{confirmation?.message}</DialogDescription></DialogHeader><Button variant="destructive" disabled={busy} onClick={()=>void run(async()=>{await confirmation?.execute();setConfirmation(undefined)})}>{busy?'处理中…':'确认执行'}</Button></DialogContent></Dialog>
  </div>
}
export default function OrganizationPage(){return <Providers><RootLayout mainHeader={<Header/>}><OrganizationPanel/></RootLayout></Providers>}
