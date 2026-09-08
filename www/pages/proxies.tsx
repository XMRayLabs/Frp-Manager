import { PageHeading } from '@/components/page-heading'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { ProxyConfigList } from '@/components/proxy/proxy_config_list'
import { useState } from 'react'
import { ProxyConfigMutateDialog } from '@/components/proxy/mutate_proxy_config'
import { IdInput } from '@/components/base/id_input'
import { ClientSelector } from '@/components/base/client-selector'
import { ServerSelector } from '@/components/base/server-selector'
import { $proxyTableRefetchTrigger } from '@/store/refetch-trigger'

export default function Proxies() {
  const [keyword, setKeyword] = useState('')
  const [clientID, setClientID] = useState<string | undefined>(undefined)
  const [serverID, setServerID] = useState<string | undefined>(undefined)

  const triggerRefetch = (n: string) => {
    $proxyTableRefetchTrigger.set(Math.random())
  }

  return (
    <Providers>
      <RootLayout mainHeader={<Header />}>
        <div className="mx-auto w-full max-w-[1600px]">
          <PageHeading
            title="隧道管理"
            description="选择客户端与公网服务端，将本地服务映射到外部端口。创建后可在列表中启停和修改。"
            actions={<ProxyConfigMutateDialog />}
          />
          <div className="flex flex-1 flex-col">
            <div className="flex flex-1 flex-row mb-2 gap-2">
              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-2">
                <ClientSelector clientID={clientID} setClientID={setClientID} />
                <ServerSelector serverID={serverID} setServerID={setServerID} />
                <IdInput setKeyword={setKeyword} keyword={keyword} refetchTrigger={triggerRefetch} />
                <button
                  className="quick-action"
                  onClick={() => {
                    setClientID(undefined)
                    setServerID(undefined)
                    setKeyword('')
                  }}
                >
                  清空筛选
                </button>
              </div>
            </div>
            <ProxyConfigList Keyword={keyword} ProxyConfigs={[]} ClientID={clientID} ServerID={serverID} />
          </div>
        </div>
      </RootLayout>
    </Providers>
  )
}
