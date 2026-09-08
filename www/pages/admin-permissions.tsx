import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { listClient } from '@/api/client'
import {
  createInvite,
  getRegisterSetting,
  grantPermission,
  listGroups,
  listInvites,
  listResourcePermissions,
  listUsers,
  revokePermission,
  updateInvite,
  updateRegisterSetting,
  updateUser,
  type AdminUser,
  type InviteCode,
  type ResourcePermission,
  type UserGroup,
} from '@/api/permission'
import { listServer } from '@/api/server'
import { $userInfo } from '@/store/user'
import { useStore } from '@nanostores/react'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { toast } from 'sonner'

const STATUS_NORMAL = 1
const STATUS_BANNED = 2

type UserStatusFilter = 'all' | 'normal' | 'banned'
type UserRoleFilter = 'all' | 'admin' | 'normal'
type PermissionDraft = {
  objType: string
  objID: string
  permission: 'view' | 'edit'
}
type ResourcePermissionDraft = PermissionDraft & {
  targetType: 'user' | 'group'
  targetID: string
}
type ResourceOption = {
  objID: string
  label: string
}

const defaultPermissionDraft: PermissionDraft = {
  objType: 'client',
  objID: '',
  permission: 'view',
}

const defaultResourcePermissionDraft: ResourcePermissionDraft = {
  objType: 'client',
  objID: '',
  permission: 'view',
  targetType: 'user',
  targetID: '',
}

function AdminPermissionPanel() {
  const userInfo = useStore($userInfo)
  const [invites, setInvites] = useState<InviteCode[]>([])
  const [users, setUsers] = useState<AdminUser[]>([])
  const [groups, setGroups] = useState<UserGroup[]>([])
  const [selectedUserID, setSelectedUserID] = useState<number>()
  const [registerEnabled, setRegisterEnabled] = useState(false)
  const [inviteRequired, setInviteRequired] = useState(true)
  const [maxUses, setMaxUses] = useState(1)
  const [days, setDays] = useState(7)
  const [comment, setComment] = useState('')
  const [keyword, setKeyword] = useState('')
  const [roleFilter, setRoleFilter] = useState<UserRoleFilter>('all')
  const [statusFilter, setStatusFilter] = useState<UserStatusFilter>('all')
  const [permissionDraft, setPermissionDraft] = useState<PermissionDraft>(defaultPermissionDraft)
  const [resourcePermissionDraft, setResourcePermissionDraft] = useState<ResourcePermissionDraft>(defaultResourcePermissionDraft)
  const [resourcePermissions, setResourcePermissions] = useState<ResourcePermission[]>([])
  const [profileDraft, setProfileDraft] = useState({ user_name: '', email: '' })
  const [resourceOptions, setResourceOptions] = useState<{ clients: ResourceOption[]; servers: ResourceOption[] }>({ clients: [], servers: [] })

  const reloadRegistration = async () => {
    const [setting, inviteList] = await Promise.all([getRegisterSetting(), listInvites()])
    setRegisterEnabled(setting.register_enabled)
    setInviteRequired(setting.invite_required)
    setInvites(inviteList)
  }

  const reloadUsers = async () => {
    const nextUsers = await listUsers()
    setUsers(nextUsers)
    setSelectedUserID((current) => current ?? nextUsers[0]?.user_id)
  }

  const reloadGroups = async () => {
    const nextGroups = await listGroups()
    setGroups(nextGroups)
  }

  const reloadResources = async () => {
    const [clientResp, serverResp] = await Promise.all([
      listClient({ page: 1, pageSize: 200 }),
      listServer({ page: 1, pageSize: 200 }),
    ])
    setResourceOptions({
      clients: clientResp.clients
        .map((client) => ({ objID: client.id ?? '', label: formatResourceLabel(client.id, client.comment) }))
        .filter((item) => item.objID),
      servers: serverResp.servers
        .map((server) => ({ objID: server.id ?? '', label: formatResourceLabel(server.id, server.comment) }))
        .filter((item) => item.objID),
    })
  }

  useEffect(() => {
    if (userInfo?.role !== 'admin') return
    Promise.all([reloadRegistration(), reloadUsers(), reloadGroups(), reloadResources()]).catch((e) => toast.error(getErrorMessage(e)))
  }, [userInfo?.role])

  const selectedUser = useMemo(() => users.find((user) => user.user_id === selectedUserID) ?? users[0], [selectedUserID, users])

  useEffect(() => {
    if (!selectedUser) return
    setProfileDraft({
      user_name: selectedUser.user_name ?? '',
      email: selectedUser.email ?? '',
    })
  }, [selectedUser])

  const userSummary = useMemo(() => {
    return {
      total: users.length,
      normal: users.filter((user) => user.status !== STATUS_BANNED).length,
      banned: users.filter((user) => user.status === STATUS_BANNED).length,
      admins: users.filter((user) => user.role === 'admin').length,
      standard: users.filter((user) => user.role !== 'admin').length,
    }
  }, [users])

  const filteredUsers = useMemo(() => {
    const q = keyword.trim().toLowerCase()
    return users
      .filter((user) => !q || user.user_name?.toLowerCase().includes(q) || user.email?.toLowerCase().includes(q) || String(user.user_id).includes(q))
      .filter((user) => roleFilter === 'all' || user.role === roleFilter)
      .filter((user) => statusFilter === 'all' || (statusFilter === 'banned' ? user.status === STATUS_BANNED : user.status !== STATUS_BANNED))
  }, [keyword, roleFilter, statusFilter, users])

  const activeInvites = useMemo(() => invites.filter(isInviteActive), [invites])

  useEffect(() => {
    setResourcePermissionDraft((prev) => {
      if (prev.objID) return prev
      const currentResources = prev.objType === 'server' ? resourceOptions.servers : resourceOptions.clients
      if (currentResources[0]?.objID) {
        return { ...prev, objID: currentResources[0].objID }
      }
      if (resourceOptions.servers[0]?.objID) {
        return { ...prev, objType: 'server', objID: resourceOptions.servers[0].objID }
      }
      return prev
    })
  }, [resourceOptions])

  useEffect(() => {
    setResourcePermissionDraft((prev) => {
      if (prev.targetID) return prev
      const firstTargetID = prev.targetType === 'group' ? groups[0]?.group_id : users[0]?.user_id ? String(users[0].user_id) : ''
      return { ...prev, targetID: firstTargetID ?? '' }
    })
  }, [groups, users])

  const reloadResourcePermissions = async (draft: ResourcePermissionDraft = resourcePermissionDraft) => {
    if (!draft.objID) {
      setResourcePermissions([])
      return
    }
    const permissions = await listResourcePermissions({ obj_type: draft.objType, obj_id: draft.objID })
    setResourcePermissions(permissions)
  }

  useEffect(() => {
    if (userInfo?.role !== 'admin' || !resourcePermissionDraft.objID) return
    listResourcePermissions({ obj_type: resourcePermissionDraft.objType, obj_id: resourcePermissionDraft.objID })
      .then(setResourcePermissions)
      .catch((e) => toast.error(getErrorMessage(e)))
  }, [userInfo?.role, resourcePermissionDraft.objType, resourcePermissionDraft.objID])

  const submitInvite = async () => {
    const expiresAt = days > 0 ? Math.floor(Date.now() / 1000) + days * 86400 : undefined
    await createInvite({ max_uses: maxUses, expires_at: expiresAt, comment })
    setComment('')
    await reloadRegistration()
    toast.success('邀请码已创建')
  }

  const toggleRegisterEnabled = async (checked: boolean) => {
    setRegisterEnabled(checked)
    const next = await updateRegisterSetting({ register_enabled: checked })
    setRegisterEnabled(next.register_enabled)
    setInviteRequired(next.invite_required)
    toast.success('注册设置已更新')
  }

  const toggleInviteRequired = async (checked: boolean) => {
    setInviteRequired(checked)
    const next = await updateRegisterSetting({ invite_required: checked })
    setRegisterEnabled(next.register_enabled)
    setInviteRequired(next.invite_required)
    toast.success('注册设置已更新')
  }

  const toggleInvite = async (invite: InviteCode) => {
    await updateInvite({ id: invite.id, disabled: !invite.disabled })
    await reloadRegistration()
  }

  const patchUser = async (user: AdminUser, patch: Partial<AdminUser>) => {
    await updateUser({ user_id: user.user_id, ...patch })
    await reloadUsers()
    toast.success('用户已更新')
  }

  const saveProfile = async () => {
    if (!selectedUser) return
    await patchUser(selectedUser, profileDraft)
  }

  const submitPermission = async (mode: 'grant' | 'revoke') => {
    if (!selectedUser) return
    if (!permissionDraft.objID.trim()) {
      toast.error('请选择要授权的机器')
      return
    }
    const payload = {
      obj_type: permissionDraft.objType,
      obj_id: permissionDraft.objID.trim(),
      target_type: 'user' as const,
      target_id: String(selectedUser.user_id),
      permission: permissionDraft.permission,
    }
    if (mode === 'grant') {
      await grantPermission(payload)
      toast.success('权限已分配')
    } else {
      await revokePermission(payload)
      toast.success('权限已撤销')
    }
  }

  const submitResourcePermission = async (mode: 'grant' | 'revoke', override?: Partial<ResourcePermissionDraft>) => {
    const draft = { ...resourcePermissionDraft, ...override }
    if (!draft.objID) {
      toast.error('请选择客户端或服务端')
      return
    }
    if (!draft.targetID) {
      toast.error(draft.targetType === 'group' ? '请选择权限组' : '请选择用户')
      return
    }
    const payload = {
      obj_type: draft.objType,
      obj_id: draft.objID,
      target_type: draft.targetType,
      target_id: draft.targetID,
      permission: draft.permission,
    }
    if (mode === 'grant') {
      await grantPermission(payload)
      toast.success('机器权限已授权')
    } else {
      await revokePermission(payload)
      toast.success('机器权限已撤销')
    }
    await reloadResourcePermissions(draft)
  }

  if (!userInfo) {
    return <AccessState title="正在加载账户信息" description="权限信息准备好后再显示管理面板。" />
  }

  if (userInfo.role !== 'admin') {
    return <AccessState title="权限不足" description="只有管理员可以打开权限管理。" />
  }

  return (
    <div className="mx-auto w-full max-w-7xl py-4">
      <Tabs defaultValue="users" className="space-y-4">
        <TabsList>
          <TabsTrigger value="users">用户管理</TabsTrigger>
          <TabsTrigger value="resources">机器权限</TabsTrigger>
          <TabsTrigger value="registration">注册与邀请</TabsTrigger>
        </TabsList>

        <TabsContent value="users" className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <Metric title="注册用户" value={userSummary.total} />
            <Metric title="正常用户" value={userSummary.normal} />
            <Metric title="封禁用户" value={userSummary.banned} />
            <Metric title="管理员" value={userSummary.admins} />
            <Metric title="标准用户" value={userSummary.standard} />
          </div>

          <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
            <Card>
              <CardHeader>
                <CardTitle>用户列表</CardTitle>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex flex-wrap items-center gap-2">
                  <Input className="max-w-xs" value={keyword} onChange={(e) => setKeyword(e.target.value)} placeholder="搜索用户、邮箱或 ID" />
                  <select className="h-9 rounded-md border bg-background px-3 text-sm" value={roleFilter} onChange={(e) => setRoleFilter(e.target.value as UserRoleFilter)}>
                    <option value="all">全部角色</option>
                    <option value="admin">管理员</option>
                    <option value="normal">标准用户</option>
                  </select>
                  <select className="h-9 rounded-md border bg-background px-3 text-sm" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value as UserStatusFilter)}>
                    <option value="all">全部状态</option>
                    <option value="normal">正常</option>
                    <option value="banned">已封禁</option>
                  </select>
                  <Button variant="outline" onClick={() => reloadUsers()}>
                    刷新
                  </Button>
                </div>
                <UserTable users={filteredUsers} selectedUserID={selectedUser?.user_id} onSelect={setSelectedUserID} />
              </CardContent>
            </Card>

            <UserDetailPanel
              user={selectedUser}
              profileDraft={profileDraft}
              permissionDraft={permissionDraft}
              onProfileChange={setProfileDraft}
              onPermissionDraftChange={(patch) => setPermissionDraft((prev) => ({ ...prev, ...patch }))}
              onSaveProfile={saveProfile}
              onPatchUser={patchUser}
              onPermission={submitPermission}
              resourceOptions={resourceOptions}
            />
          </div>
        </TabsContent>

        <TabsContent value="resources" className="space-y-4">
          <ResourcePermissionPanel
            users={users}
            groups={groups}
            draft={resourcePermissionDraft}
            permissions={resourcePermissions}
            resourceOptions={resourceOptions}
            onDraftChange={(patch) => setResourcePermissionDraft((prev) => ({ ...prev, ...patch }))}
            onPermission={submitResourcePermission}
            onRefresh={() => reloadResourcePermissions()}
          />
        </TabsContent>

        <TabsContent value="registration" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>注册设置</CardTitle>
            </CardHeader>
            <CardContent className="flex flex-col gap-4">
              <SettingRow title="允许新用户注册" description="关闭后，除首个管理员外，任何邀请码都不能注册新账号。">
                <Switch checked={registerEnabled} onCheckedChange={toggleRegisterEnabled} />
              </SettingRow>
              <SettingRow title="注册必须使用邀请码" description="关闭后，开放注册不再校验邀请码。">
                <Switch checked={inviteRequired} onCheckedChange={toggleInviteRequired} disabled={!registerEnabled} />
              </SettingRow>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>邀请码设置</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 md:grid-cols-[160px_160px_1fr_auto]">
                <Field label="激活次数" hint="这个邀请码最多可注册几个账号。">
                  <Input type="number" min={1} value={maxUses} onChange={(e) => setMaxUses(Number(e.target.value))} placeholder="例如 1" />
                </Field>
                <Field label="有效天数" hint="0 表示长期有效。">
                  <Input type="number" min={0} value={days} onChange={(e) => setDays(Number(e.target.value))} placeholder="例如 7" />
                </Field>
                <Field label="备注" hint="给管理员看的用途说明。">
                  <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="例如 测试客户/某团队" />
                </Field>
                <div className="flex items-end">
                  <Button className="w-full" onClick={submitInvite}>新增邀请码</Button>
                </div>
              </div>
              <InviteTable invites={activeInvites} onToggle={toggleInvite} />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}

function Metric({ title, value }: { title: string; value: number }) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold">{value}</div>
      </CardContent>
    </Card>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="grid gap-1">
      <span className="text-sm font-medium">{label}</span>
      {children}
      {hint && <span className="text-xs text-muted-foreground">{hint}</span>}
    </label>
  )
}

function AccessState({ title, description }: { title: string; description: string }) {
  return (
    <div className="mx-auto flex min-h-[360px] w-full max-w-3xl items-center justify-center py-8">
      <Card className="w-full">
        <CardHeader>
          <CardTitle>{title}</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">{description}</CardContent>
      </Card>
    </div>
  )
}

function SettingRow({ title, description, children }: { title: string; description: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between rounded-md border p-3">
      <div>
        <div className="text-sm font-medium">{title}</div>
        <div className="text-xs text-muted-foreground">{description}</div>
      </div>
      {children}
    </div>
  )
}

function InviteTable({ invites, onToggle }: { invites: InviteCode[]; onToggle: (invite: InviteCode) => void }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>邀请码</TableHead>
          <TableHead>使用次数</TableHead>
          <TableHead>有效期</TableHead>
          <TableHead>状态</TableHead>
          <TableHead className="text-right">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {invites.map((invite) => (
          <TableRow key={invite.id}>
            <TableCell className="font-mono">{invite.code}</TableCell>
            <TableCell>
              {invite.used_count}/{invite.max_uses}
            </TableCell>
            <TableCell>{invite.expires_at ? new Date(invite.expires_at).toLocaleString() : '长期'}</TableCell>
            <TableCell>{invite.disabled ? '禁用' : '启用'}</TableCell>
            <TableCell className="text-right">
              <Button variant="outline" size="sm" onClick={() => onToggle(invite)}>
                {invite.disabled ? '启用' : '禁用'}
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function UserTable({ users, selectedUserID, onSelect }: { users: AdminUser[]; selectedUserID?: number; onSelect: (userID: number) => void }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-16">ID</TableHead>
          <TableHead>用户</TableHead>
          <TableHead>角色</TableHead>
          <TableHead>状态</TableHead>
          <TableHead className="text-right">操作</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {users.map((user) => (
          <TableRow key={user.user_id} className={user.user_id === selectedUserID ? 'bg-muted/50' : undefined}>
            <TableCell>{user.user_id}</TableCell>
            <TableCell>
              <div className="font-medium">{user.user_name}</div>
              <div className="text-xs text-muted-foreground">{user.email}</div>
            </TableCell>
            <TableCell>{user.role === 'admin' ? '管理员' : '标准用户'}</TableCell>
            <TableCell>{user.status === STATUS_BANNED ? '已封禁' : '正常'}</TableCell>
            <TableCell className="text-right">
              <Button variant="outline" size="sm" onClick={() => onSelect(user.user_id)}>
                管理
              </Button>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function ResourcePermissionPanel({
  users,
  groups,
  draft,
  permissions,
  resourceOptions,
  onDraftChange,
  onPermission,
  onRefresh,
}: {
  users: AdminUser[]
  groups: UserGroup[]
  draft: ResourcePermissionDraft
  permissions: ResourcePermission[]
  resourceOptions: { clients: ResourceOption[]; servers: ResourceOption[] }
  onDraftChange: (patch: Partial<ResourcePermissionDraft>) => void
  onPermission: (mode: 'grant' | 'revoke', override?: Partial<ResourcePermissionDraft>) => void
  onRefresh: () => void
}) {
  const resources = draft.objType === 'server' ? resourceOptions.servers : resourceOptions.clients
  const targets = draft.targetType === 'group'
    ? groups.map((group) => ({ id: group.group_id, label: formatResourceLabel(group.group_id, group.group_name) }))
    : users.map((user) => ({ id: String(user.user_id), label: formatUserLabel(user) }))

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
      <Card>
        <CardHeader>
          <CardTitle>客户端/服务端权限</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-[160px_minmax(240px,1fr)_auto]">
            <Field label="机器类型">
              <select
                className="h-9 rounded-md border bg-background px-3 text-sm"
                value={draft.objType}
                onChange={(e) => {
                  const objType = e.target.value
                  const nextResources = objType === 'server' ? resourceOptions.servers : resourceOptions.clients
                  onDraftChange({ objType, objID: nextResources[0]?.objID ?? '' })
                }}
              >
                <option value="client">客户端</option>
                <option value="server">服务端</option>
              </select>
            </Field>
            <Field label="选择机器">
              <select className="h-9 rounded-md border bg-background px-3 text-sm" value={draft.objID} onChange={(e) => onDraftChange({ objID: e.target.value })}>
                <option value="">{resources.length ? '选择要管理的机器' : '暂无机器'}</option>
                {resources.map((resource) => (
                  <option key={resource.objID} value={resource.objID}>
                    {resource.label}
                  </option>
                ))}
              </select>
            </Field>
            <div className="flex items-end">
              <Button variant="outline" className="w-full" onClick={onRefresh}>
                刷新
              </Button>
            </div>
          </div>

          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>授权对象</TableHead>
                <TableHead>类型</TableHead>
                <TableHead>权限</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {permissions.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} className="py-8 text-center text-sm text-muted-foreground">
                    暂无单独授权记录
                  </TableCell>
                </TableRow>
              )}
              {permissions.map((permission) => (
                <TableRow key={`${permission.target_type}:${permission.target_id}`}>
                  <TableCell>
                    <div className="font-medium">{permission.target_name}</div>
                    <div className="text-xs text-muted-foreground">{permission.target_id}</div>
                  </TableCell>
                  <TableCell>{permission.target_type === 'group' ? '权限组' : '用户'}</TableCell>
                  <TableCell>{permission.permission === 'edit' ? '编辑' : '仅查看'}</TableCell>
                  <TableCell className="text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => onPermission('revoke', {
                        targetType: permission.target_type,
                        targetID: permission.target_id,
                        permission: permission.permission,
                      })}
                    >
                      撤销
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>新增授权</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3">
          <Field label="授权对象类型">
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              value={draft.targetType}
              onChange={(e) => {
                const targetType = e.target.value as ResourcePermissionDraft['targetType']
                const targetID = targetType === 'group' ? groups[0]?.group_id ?? '' : users[0]?.user_id ? String(users[0].user_id) : ''
                onDraftChange({ targetType, targetID })
              }}
            >
              <option value="user">用户</option>
              <option value="group">权限组</option>
            </select>
          </Field>
          <Field label={draft.targetType === 'group' ? '选择权限组' : '选择用户'}>
            <select className="h-9 rounded-md border bg-background px-3 text-sm" value={draft.targetID} onChange={(e) => onDraftChange({ targetID: e.target.value })}>
              <option value="">{targets.length ? '选择授权对象' : '暂无可选对象'}</option>
              {targets.map((target) => (
                <option key={target.id} value={target.id}>
                  {target.label}
                </option>
              ))}
            </select>
          </Field>
          <Field label="权限级别" hint="编辑权限包含仅查看权限。">
            <select className="h-9 rounded-md border bg-background px-3 text-sm" value={draft.permission} onChange={(e) => onDraftChange({ permission: e.target.value as ResourcePermissionDraft['permission'] })}>
              <option value="view">仅查看</option>
              <option value="edit">编辑</option>
            </select>
          </Field>
          <Button onClick={() => onPermission('grant')}>授权给当前机器</Button>
        </CardContent>
      </Card>
    </div>
  )
}

function UserDetailPanel({
  user,
  profileDraft,
  permissionDraft,
  resourceOptions,
  onProfileChange,
  onPermissionDraftChange,
  onSaveProfile,
  onPatchUser,
  onPermission,
}: {
  user?: AdminUser
  profileDraft: { user_name: string; email: string }
  permissionDraft: PermissionDraft
  resourceOptions: { clients: ResourceOption[]; servers: ResourceOption[] }
  onProfileChange: (value: { user_name: string; email: string }) => void
  onPermissionDraftChange: (patch: Partial<PermissionDraft>) => void
  onSaveProfile: () => void
  onPatchUser: (user: AdminUser, patch: Partial<AdminUser>) => void
  onPermission: (mode: 'grant' | 'revoke') => void
}) {
  const currentResourceOptions = permissionDraft.objType === 'server' ? resourceOptions.servers : resourceOptions.clients
  const currentResourceName = permissionDraft.objType === 'server' ? '服务端' : '客户端'

  if (!user) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>用户详情</CardTitle>
        </CardHeader>
        <CardContent className="text-sm text-muted-foreground">请选择一个用户。</CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>用户详情</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="rounded-md border p-3">
          <div className="text-sm font-medium">{user.user_name}</div>
          <div className="text-xs text-muted-foreground">ID {user.user_id}</div>
        </div>

        <div className="space-y-2">
          <div className="text-sm font-medium">基础信息</div>
          <Input value={profileDraft.user_name} onChange={(e) => onProfileChange({ ...profileDraft, user_name: e.target.value })} placeholder="用户名" />
          <Input value={profileDraft.email} onChange={(e) => onProfileChange({ ...profileDraft, email: e.target.value })} placeholder="邮箱" />
          <Button onClick={onSaveProfile}>保存资料</Button>
        </div>

        <div className="space-y-2">
          <div className="text-sm font-medium">账号控制</div>
          <div className="flex flex-wrap gap-2">
            <select className="h-9 rounded-md border bg-background px-3 text-sm" value={user.role || 'normal'} onChange={(e) => onPatchUser(user, { role: e.target.value })}>
              <option value="normal">标准用户</option>
              <option value="admin">管理员</option>
            </select>
            <Button variant={user.status === STATUS_BANNED ? 'outline' : 'destructive'} size="sm" onClick={() => onPatchUser(user, { status: user.status === STATUS_BANNED ? STATUS_NORMAL : STATUS_BANNED })}>
              {user.status === STATUS_BANNED ? '解除封禁' : '封禁用户'}
            </Button>
          </div>
        </div>

        <div className="space-y-2">
          <div className="text-sm font-medium">单用户权限</div>
          <div className="grid gap-2">
            <select className="h-9 rounded-md border bg-background px-3 text-sm" value={permissionDraft.objType} onChange={(e) => onPermissionDraftChange({ objType: e.target.value, objID: '' })}>
              <option value="client">客户端</option>
              <option value="server">服务端</option>
            </select>
            <select className="h-9 rounded-md border bg-background px-3 text-sm" value={permissionDraft.objID} onChange={(e) => onPermissionDraftChange({ objID: e.target.value })}>
              <option value="">{currentResourceOptions.length ? `选择${currentResourceName}` : `暂无可授权${currentResourceName}`}</option>
              {currentResourceOptions.map((resource) => (
                <option key={resource.objID} value={resource.objID}>
                  {resource.label}
                </option>
              ))}
            </select>
            <select className="h-9 rounded-md border bg-background px-3 text-sm" value={permissionDraft.permission} onChange={(e) => onPermissionDraftChange({ permission: e.target.value as PermissionDraft['permission'] })}>
              <option value="view">仅查看</option>
              <option value="edit">编辑</option>
            </select>
            <div className="flex gap-2">
              <Button size="sm" onClick={() => onPermission('grant')}>
                授权
              </Button>
              <Button variant="outline" size="sm" onClick={() => onPermission('revoke')}>
                撤销
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function getErrorMessage(error: unknown) {
  if (typeof error === 'string') return error
  if (error instanceof Error) return error.message
  return '操作失败'
}

function formatResourceLabel(id?: string, comment?: string) {
  if (!id) return ''
  return comment ? `${comment} (${id})` : id
}

function formatUserLabel(user: AdminUser) {
  const name = user.user_name || user.email || `用户 ${user.user_id}`
  return `${name} (#${user.user_id})`
}

function isInviteActive(invite: InviteCode) {
  const hasUses = invite.max_uses <= 0 || invite.used_count < invite.max_uses
  const notExpired = !invite.expires_at || new Date(invite.expires_at).getTime() > Date.now()
  return hasUses && notExpired
}

export default function AdminPermissionsPage() {
  return (
    <Providers>
      <RootLayout mainHeader={<Header />}>
        <AdminPermissionPanel />
      </RootLayout>
    </Providers>
  )
}
