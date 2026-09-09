import { useState } from 'react'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
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
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <IdInput keyword={keyword} setKeyword={setKeyword} refetchTrigger={setTrigger} />
            <div className="flex flex-wrap items-center gap-3">
              {user?.role === 'admin' && <ClientJoinButton role="server" />}
              {user?.role === 'admin' && <CreateServerDialog refetchTrigger={setTrigger} triggerLabel="手动创建" />}
            </div>
          </div>
          <ServerList Servers={[]} Keyword={keyword} TriggerRefetch={trigger} />
        </div>
      </RootLayout>
    </Providers>
  )
}
