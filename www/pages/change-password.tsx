import { useState } from 'react'
import { useRouter } from 'next/router'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { changeInitialPassword, getPasswordStatus } from '@/api/user'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { toast } from 'sonner'

export default function ChangePasswordPage() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const [current, setCurrent] = useState('')
  const [password, setPassword] = useState('')
  const [confirmation, setConfirmation] = useState('')
  const [busy, setBusy] = useState(false)
  const status = useQuery({ queryKey: ['passwordStatus'], queryFn: getPasswordStatus, retry: false })
  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <section className="w-full max-w-md space-y-5 rounded-xl border bg-card p-6">
        <h1 className="text-xl font-semibold">首次登录，请修改密码</h1>
        <p className="text-sm text-muted-foreground">初始密码是你的邮箱。设置新密码后才能使用面板；至少 8 位，包含大小写字母、数字、符号中的至少三类，不能与邮箱相同。</p>
        {status.data?.mustChangePassword === false ? <Button onClick={() => router.replace('/')}>密码已修改，进入面板</Button> : (
          <form className="grid gap-4" onSubmit={async (event) => {
            event.preventDefault()
            if (busy) return
            if (password !== confirmation) { toast.error('两次输入的新密码不一致'); return }
            setBusy(true)
            try {
              await changeInitialPassword(current, password)
              setCurrent(''); setPassword(''); setConfirmation('')
              queryClient.clear()
              toast.success('密码已修改')
              await router.replace('/')
            } catch (error) { toast.error(error instanceof Error ? error.message : '修改密码失败') }
            finally { setBusy(false) }
          }}>
            <label className="grid gap-2 text-sm">当前密码<Input type="password" autoComplete="current-password" value={current} onChange={(e) => setCurrent(e.target.value)} required /></label>
            <label className="grid gap-2 text-sm">新密码<Input type="password" autoComplete="new-password" value={password} onChange={(e) => setPassword(e.target.value)} minLength={8} maxLength={72} required /></label>
            <label className="grid gap-2 text-sm">确认新密码<Input type="password" autoComplete="new-password" value={confirmation} onChange={(e) => setConfirmation(e.target.value)} minLength={8} maxLength={72} required /></label>
            <Button type="submit" disabled={busy || status.isPending || status.isError}>{busy ? '正在修改…' : '修改密码并进入面板'}</Button>
            {status.isError && <Button type="button" variant="outline" onClick={() => status.refetch()}>状态读取失败，重试</Button>}
          </form>
        )}
        <Button variant="link" className="px-0" onClick={async () => { await fetch('/api/v1/auth/logout', { credentials: 'include' }); window.location.assign('/login') }}>退出登录</Button>
      </section>
    </main>
  )
}
