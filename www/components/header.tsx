import { Button } from './ui/button'
import { useStore } from '@nanostores/react'
import { useRouter } from 'next/router'
import { $platformInfo, $userInfo, $statusOnline } from '@/store/user'
import { getUserInfo } from '@/api/user'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { getPlatformInfo } from '@/api/platform'
import { useTranslation } from 'react-i18next'
import { LanguageSwitcher } from './language-switcher'
import { CircleCheck, CircleX } from 'lucide-react'

export const Header = ({ title }: { title?: string }) => {
  const router = useRouter()
  const isOnline = useStore($statusOnline)
  const currentPath = router.pathname

  const { isPending } = useQuery({
    queryKey: ['userInfo'],
    queryFn: () => getUserInfo({}),
    retry: false,
  })

  useEffect(() => {
    // 只有在初始化完成后才进行状态检查和跳转
    if (!isPending) {
      // console.log('isInitializing', isOnline, token, currentPath)
      // 如果用户未登录且不在登录/注册页面，则跳转到登录页
      const isAuthPage = ['/login', '/register'].includes(currentPath)
      if (!isOnline && !isAuthPage) {
        router.push('/login')
      }
    }
  }, [isOnline, router, isPending, currentPath])

  return (
    <div className="flex w-full justify-between items-center gap-2">
      {title && <h1 className="truncate text-sm font-semibold">{title}</h1>}
      {!title && (
        <h1 className="truncate text-sm font-semibold">
          {(
            {
              '/': '概览',
              '/clients': '客户端',
              '/servers': '服务端',
              '/proxies': '隧道管理',
              '/clientedit': '配置客户端',
              '/serveredit': '配置服务端',
              '/clientstats': '流量统计',
              '/platform-settings': '面板设置',
            } as Record<string, string>
          )[currentPath] || 'frp-manager'}
        </h1>
      )}
      <div className="flex items-center gap-2">
        <span className={isOnline ? 'status-chip status-chip-online' : 'status-chip status-chip-offline'}>
          {isOnline ? <CircleCheck className="size-3.5" /> : <CircleX className="size-3.5" />}
          {isOnline ? '已连接面板' : '未登录'}
        </span>
        <LanguageSwitcher />
      </div>
    </div>
  )
}

export const RegisterAndLogin = () => {
  const router = useRouter()
  const userInfo = useStore($userInfo)
  const { t } = useTranslation()

  const platformInfo = useQuery({
    queryKey: ['platformInfo'],
    queryFn: getPlatformInfo,
  })

  useEffect(() => {
    $platformInfo.set(platformInfo.data)
  }, [platformInfo.data])

  const { data: userInfoQuery } = useQuery({
    queryKey: ['userInfo'],
    queryFn: getUserInfo,
  })

  useEffect(() => {
    $userInfo.set(userInfoQuery?.userInfo)
    $statusOnline.set(!!userInfoQuery?.userInfo)
  }, [userInfoQuery])

  return (
    <>
      {!userInfo && (
        <Button variant="ghost" size="sm" onClick={() => router.push('/login')}>
          {t('common.login')}
        </Button>
      )}
      {!userInfo && (
        <Button variant="ghost" size="sm" onClick={() => router.push('/register')}>
          {t('common.register')}
        </Button>
      )}
    </>
  )
}
