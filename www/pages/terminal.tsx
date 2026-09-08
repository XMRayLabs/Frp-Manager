import { Providers } from '@/components/providers'
import { useEffect, useState } from 'react'
import { ClientType } from '@/lib/pb/common'
import dynamic from 'next/dynamic'
import { ClientStatus } from '@/lib/pb/api_master'
import { useSearchParams } from 'next/navigation'
import { $userInfo } from '@/store/user'
import { getUserInfo } from '@/api/user'
import { useStore } from '@nanostores/react'
import { useQuery } from '@tanstack/react-query'

const TerminalComponentProps = dynamic(() => import('@/components/base/read-write-xterm'), {
  ssr: false
})

export default function TerminalPage() {
  return (
    <Providers>
      <TerminalContent />
    </Providers>
  )
}

function TerminalContent() {
  const userInfo = useStore($userInfo)
  const [clientID, setClientID] = useState<string | undefined>(undefined)
  const [clear, setClear] = useState<number>(0)
  const [enabled, setEnabled] = useState<boolean>(false)
  const [clientType, setClientType] = useState<ClientType>(ClientType.FRPC)
  const [status, setStatus] = useState<"loading" | "success" | "error" | undefined>()

  useQuery({
    queryKey: ['terminalUserInfo'],
    queryFn: getUserInfo,
    retry: false,
  })

  const searchParams = useSearchParams()
  const paramClientID = searchParams.get('clientID')
  const paramClientType = searchParams.get('clientType')

  useEffect(() => {
    if (paramClientID) {
      setClientID(paramClientID)
    }
    if (paramClientType) {
      if (paramClientType == ClientType.FRPC.toString()) {
        setClientType(ClientType.FRPC)
      } else if (paramClientType == ClientType.FRPS.toString()) {
        setClientType(ClientType.FRPS)
      }
    }
    if (paramClientID && paramClientType) {
      setEnabled(true)
    }
  }, [paramClientID, paramClientType])

  useEffect(() => {
    setClear(Math.random())
    setStatus(undefined)
    if (!clientID) {
      return;
    }
    if (!enabled) {
      return;
    }
    const abortController = new AbortController();
    setStatus("loading");

    return () => {
      abortController.abort("unmount");
      setEnabled(false);
    };
  }, [clientID, enabled]);

  if (!userInfo) {
    return <TerminalAccessState title="正在加载账户信息" />
  }

  if (userInfo.role !== 'admin') {
    return <TerminalAccessState title="权限不足，普通用户不能打开远程控制台。" />
  }

  return (
    <div className='flex-col h-[100dvh] flex w-full relative'>
      {/* <TerminalAlerts clientID={clientID||''} status={"error"} onAlertChange={() => { }} /> */}
      <TerminalComponentProps
        setStatus={setStatus}
        isLoading={!enabled}
        clientStatus={{
          clientId: clientID,
          clientType: clientType,
          version: { platform: "linux" },
        } as ClientStatus}
        reset={clear} />
    </div>
  )
}

function TerminalAccessState({ title }: { title: string }) {
  return (
    <div className="flex h-[100dvh] w-full items-center justify-center p-6 text-sm text-muted-foreground">
      {title}
    </div>
  )
}
