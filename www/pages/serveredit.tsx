import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { FRPSFormCard } from '@/components/frps/frps_card'
import { $userInfo } from '@/store/user'
import { useStore } from '@nanostores/react'

export default function ServerListPage() {
  const userInfo = useStore($userInfo)

  return (
    <Providers>
      <RootLayout mainHeader={<Header />}>
        {!userInfo ? (
          <div className="mx-auto flex min-h-[360px] w-full max-w-3xl items-center justify-center py-8 text-sm text-muted-foreground">
            正在加载账户信息。
          </div>
        ) : userInfo.role === 'admin' ? (
          <div className="w-full flex items-center justify-center">
            <div className="flex-1 flex-col max-w-2xl">
              <FRPSFormCard />
            </div>
          </div>
        ) : (
          <div className="mx-auto flex min-h-[360px] w-full max-w-3xl items-center justify-center py-8 text-sm text-muted-foreground">
            普通用户不能编辑服务端设置。
          </div>
        )}
      </RootLayout>
    </Providers>
  )
}
