import { useState } from 'react'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { PageHeading } from '@/components/page-heading'
import { ServerList } from '@/components/frps/server_list'
import { CreateServerDialog } from '@/components/frps/server_create_dialog'
import { ClientJoinButton } from '@/components/frpc/client_join_button'
import { IdInput } from '@/components/base/id_input'
import { useStore } from '@nanostores/react'
import { $userInfo } from '@/store/user'
export default function NodesPage() {
  const [keyword, setKeyword] = useState(''),
    [trigger, setTrigger] = useState('')
  const user = useStore($userInfo)
  return (
    <Providers>
      <RootLayout mainHeader={<Header />}>
        <div className="mx-auto max-w-[1600px]">
          <PageHeading
            title="服务端"
            description="管理对外提供连接的公网入口。先接入服务端，再配置监听地址和端口。"
            actions={user?.role === 'admin' && <ClientJoinButton role="server" />}
          />
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <IdInput keyword={keyword} setKeyword={setKeyword} refetchTrigger={setTrigger} />
            <details className="relative">
              <summary className="cursor-pointer text-sm text-muted-foreground">手动创建</summary>
              <div className="absolute right-0 z-10 rounded-lg border bg-popover p-3 shadow-lg">
                {user?.role === 'admin' && <CreateServerDialog refetchTrigger={setTrigger} />}
              </div>
            </details>
          </div>
          <ServerList Servers={[]} Keyword={keyword} TriggerRefetch={trigger} />
        </div>
      </RootLayout>
    </Providers>
  )
}
