import { useState } from 'react'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { ClientList } from '@/components/frpc/client_list'
import { CreateClientDialog } from '@/components/frpc/client_create_dialog'
import { ClientJoinButton } from '@/components/frpc/client_join_button'
import { IdInput } from '@/components/base/id_input'
export default function NodesPage() {
  const [keyword, setKeyword] = useState(''),
    [trigger, setTrigger] = useState('')
  return (
    <Providers>
      <RootLayout mainHeader={<Header />}>
        <div className="mx-auto max-w-[1600px]">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
            <IdInput keyword={keyword} setKeyword={setKeyword} refetchTrigger={setTrigger} />
            <div className="flex flex-wrap items-center gap-3">
              <ClientJoinButton role="client" />
              <CreateClientDialog refetchTrigger={setTrigger} triggerLabel="手动创建" />
            </div>
          </div>
          <ClientList Clients={[]} Keyword={keyword} TriggerRefetch={trigger} />
        </div>
      </RootLayout>
    </Providers>
  )
}
