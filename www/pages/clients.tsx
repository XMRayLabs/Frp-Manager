import { useState } from 'react'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { PageHeading } from '@/components/page-heading'
import { ClientList } from '@/components/frpc/client_list'
import { CreateClientDialog } from '@/components/frpc/client_create_dialog'
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
            title="客户端"
            description="将内网设备接入面板，然后为它配置访问隧道。节点接入后会自动出现在这里。"
            actions={<ClientJoinButton role="client" />}
          />
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <IdInput keyword={keyword} setKeyword={setKeyword} refetchTrigger={setTrigger} />
            <details className="relative">
              <summary className="cursor-pointer text-sm text-muted-foreground">手动创建</summary>
              <div className="absolute right-0 z-10 rounded-lg border bg-popover p-3 shadow-lg">
                <CreateClientDialog refetchTrigger={setTrigger} />
              </div>
            </details>
          </div>
          <ClientList Clients={[]} Keyword={keyword} TriggerRefetch={trigger} />
        </div>
      </RootLayout>
    </Providers>
  )
}
