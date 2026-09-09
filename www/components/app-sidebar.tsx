import * as React from 'react'
import { NavMain } from '@/components/nav-main'
import { NavUser } from '@/components/nav-user'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarHeader,
  SidebarMenuButton,
  SidebarRail,
} from '@/components/ui/sidebar'
import { $platformInfo, $userInfo } from '@/store/user'
import { useStore } from '@nanostores/react'
import { RegisterAndLogin } from './header'
import { useRouter } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { getPlatformInfo } from '@/api/platform'
import { getNavItems } from '@/config/nav'
import { useTranslation } from 'react-i18next'
import Image from 'next/image'

export interface AppSidebarProps extends React.ComponentProps<typeof Sidebar> {
  children?: React.ReactNode
  footer?: React.ReactNode
}

export function AppSidebar({ ...props }: AppSidebarProps) {
  const router = useRouter()
  const { t } = useTranslation()
  const userInfo = useStore($userInfo)
  const { data: platformInfo } = useQuery({
    queryKey: ['platformInfo'],
    queryFn: getPlatformInfo,
  })

  React.useEffect(() => {
    $platformInfo.set(platformInfo)
  }, [platformInfo])

  return (
    <Sidebar collapsible="icon" {...props}>
      <SidebarHeader>
        <SidebarMenuButton
          size="lg"
          className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
          onClick={() => router.push('/')}
        >
          <div className="flex aspect-square size-9 items-center justify-center overflow-hidden rounded-md border border-sidebar-border bg-sidebar-primary">
            <Image src="/frp-manager-icon.svg" alt="" width={36} height={36} unoptimized className="size-9" />
          </div>
          <div className="grid flex-1 text-left leading-tight">
            <span className="truncate text-sm font-semibold">{t('app.title')}</span>
            <span className="truncate text-[11px] text-sidebar-foreground/60">连接与隧道管理</span>
          </div>
        </SidebarMenuButton>
      </SidebarHeader>
      <SidebarContent className="px-2 py-4">
        <NavMain items={getNavItems(t, userInfo?.role)} />
        {props.children}
      </SidebarContent>
      <SidebarFooter>
        {props.footer}
        <div className="flex w-full flex-row group-data-[collapsible=icon]:flex-col-reverse gap-2 justify-between">
          {userInfo && <NavUser user={userInfo} />}
          {!userInfo && <RegisterAndLogin />}
        </div>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
