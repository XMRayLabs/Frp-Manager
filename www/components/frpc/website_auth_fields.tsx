'use client'

import { useState } from 'react'
import { nanoid } from 'nanoid'
import type { WebsiteAccount } from '@/types/plugin'
import { Input } from '../ui/input'
import { Button } from '../ui/button'

export function WebsiteAuthFields({ enabled, onEnabledChange, accounts, onAccountsChange, disabled = false, minimumVersion = '1.1.6' }: {
  enabled: boolean; onEnabledChange: (value: boolean) => void
  accounts: WebsiteAccount[]; onAccountsChange: (accounts: WebsiteAccount[]) => void; disabled?: boolean; minimumVersion?: string
}) {
  const [show, setShow] = useState(false)
  const update = (index: number, changes: Partial<WebsiteAccount>) => onAccountsChange(accounts.map((account, i) => i === index ? { ...account, ...changes } : account))
  return <fieldset disabled={disabled} className="space-y-3 rounded-lg border p-3">
    <label className="flex items-center gap-2 text-sm font-medium"><input type="checkbox" checked={enabled} onChange={event => {
      onEnabledChange(event.target.checked)
      if (event.target.checked && accounts.length === 0) onAccountsChange([{ username: 'user', password: '' }])
    }} />网站访问密码（可选）</label>
    {enabled && <>
      <p className="text-xs text-muted-foreground">仅用于 HTTP 网站；访问时弹出登录框。每组账号均可访问，可单独增删。需客户端升级至 {minimumVersion} 或更新版本并在线后保存；公网建议通过 HTTPS 访问。</p>
      {accounts.map((account, index) => <div key={index} className="space-y-2 border-t pt-3">
        <div className="flex items-center justify-between gap-2"><span className="text-xs text-muted-foreground">账号 {index + 1}</span><Button type="button" variant="ghost" size="sm" onClick={() => onAccountsChange(accounts.filter((_, i) => i !== index))}>删除</Button></div>
        <div className="grid gap-2 sm:grid-cols-2">
          <label className="grid gap-1 text-sm">用户名<Input required maxLength={64} autoComplete="off" value={account.username} onChange={event => update(index, { username: event.target.value })} /></label>
          <label className="grid gap-1 text-sm">{account.passwordHash ? '新密码（留空保留）' : '密码'}<Input type={show ? 'text' : 'password'} required={!account.passwordHash} autoComplete="new-password" placeholder={account.passwordHash ? '已设置，留空保留原密码' : '8 至 72 字节'} value={account.password || ''} onChange={event => update(index, { password: event.target.value })} /></label>
        </div>
        <Button type="button" variant="outline" size="sm" onClick={() => { update(index, { password: nanoid(20) }); setShow(true) }}>生成密码</Button>
      </div>)}
      <div className="flex flex-wrap items-center gap-4"><Button type="button" variant="outline" size="sm" disabled={accounts.length >= 32} onClick={() => onAccountsChange([...accounts, { username: '', password: '' }])}>添加账号</Button><label className="flex items-center gap-2 text-xs"><input type="checkbox" checked={show} onChange={event => setShow(event.target.checked)} />显示新密码</label></div>
      {accounts.length === 0 && <p role="alert" className="text-xs text-destructive">至少保留一个账号。需要取消保护时，请关闭“网站访问密码”。</p>}
    </>}
  </fieldset>
}
