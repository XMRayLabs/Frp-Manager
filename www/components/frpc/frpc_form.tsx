import { TypedProxyConfig } from '@/types/proxy'
import React, { useEffect } from 'react'
import { useState } from 'react'
import { Label } from '@radix-ui/react-label'
import { TypedProxyForm } from './proxy_form'
import { Button } from '@/components/ui/button'
import { Client, RespCode } from '@/lib/pb/common'
import { ClientConfig } from '@/types/client'
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion'
import { AccordionHeader } from '@radix-ui/react-accordion'
import { QueryObserverResult, RefetchOptions, useMutation } from '@tanstack/react-query'
import { updateFRPC } from '@/api/frp'
import { GetClientResponse } from '@/lib/pb/api_client'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { BaseSelector } from '../base/selector'
import { ConnectionProtocols } from '@/lib/consts'

export interface FRPCFormProps {
  clientID: string
  serverID: string
  client?: Client
  clientConfig: ClientConfig
  frpsUrl?: string
  refetchClient: (options?: RefetchOptions) => Promise<QueryObserverResult<GetClientResponse, Error>>
  clientProxyConfigs: TypedProxyConfig[]
  setClientProxyConfigs: React.Dispatch<React.SetStateAction<TypedProxyConfig[]>>
}

export const FRPCForm: React.FC<FRPCFormProps> = ({ clientID, serverID, clientConfig, client, refetchClient, clientProxyConfigs, setClientProxyConfigs, frpsUrl }) => {
  const { t } = useTranslation()
  const [protocol, setProtocol] = useState<string | undefined>("tcp")

  useEffect(() => {
    if (clientConfig.transport?.protocol) {
      setProtocol(clientConfig.transport?.protocol)
    }
  }, [clientConfig])

  const handleDeleteProxy = (proxyName: string) => {
    const newProxies = clientProxyConfigs.filter((proxy) => proxy.name !== proxyName)
    setClientProxyConfigs(newProxies)
  }

  const updateFrpc = useMutation({ mutationFn: updateFRPC })

  const handleUpdate = async () => {
    try {
      const res = await updateFrpc.mutateAsync({
        //@ts-ignore
        config: Buffer.from(
          JSON.stringify({
            ...clientConfig,
            proxies: clientProxyConfigs,
            transport: {
              ...clientConfig.transport,
              protocol,
            }
          } as ClientConfig),
        ),
        serverId: serverID,
        clientId: clientID,
        frpsUrl: frpsUrl,
      })
      if (res.status?.code === RespCode.SUCCESS) await refetchClient()
      toast(t('proxy.status.update'), {
        description: res.status?.code === RespCode.SUCCESS ? t('proxy.status.success') : t('proxy.status.error')
      })
    } catch (error) {
      console.error(error)
      toast(t('proxy.status.update'), {
        description: t('proxy.status.error') + JSON.stringify(error)
      })
    }
  }

  return (
    <div className='flex flex-col space-y-2'>

      <Label className="text-sm font-medium">{t('proxy.form.protocol')}</Label>
      <BaseSelector value={protocol} setValue={setProtocol}
        dataList={ConnectionProtocols.map((item) => { return { label: item, value: item } })}
        placeholder={t('proxy.form.protocol')}
        label={t('proxy.form.protocol')} />
      <Accordion type="single" defaultValue="" collapsible key={clientID + serverID + client}>
        <AccordionItem value="proxies">
          <AccordionTrigger>
            <AccordionHeader className="flex flex-row justify-between w-full">
              <p>{t('proxy.form.config')}</p>
              <p>{t('proxy.form.expand', { count: clientProxyConfigs.length })}</p>
            </AccordionHeader>
          </AccordionTrigger>
          <AccordionContent className="grid gap-2 grid-cols-1">
            {clientProxyConfigs.map((item, index) => {
              return (
                <Accordion type="single" collapsible key={index}>
                  <AccordionItem value={item.name}>
                    <AccordionTrigger>
                      <div className='flex flex-row justify-start items-center w-full gap-4'>
                        <div>{t('proxy.form.tunnel_name')}: {item.name}</div>
                        <div>{t('proxy.form.type_label', { type: item.type })}</div>
                      </div>
                    </AccordionTrigger>
                    <AccordionContent className='border rounded-xl p-4'>
                      <Button variant="outline" className="mb-4" onClick={() => handleDeleteProxy(item.name)}>{t('proxy.form.delete')}</Button>
                      {serverID && clientID && (
                        <TypedProxyForm
                          enablePreview
                          defaultProxyConfig={item}
                          proxyName={item.name}
                          serverID={serverID}
                          clientID={clientID}
                          clientProxyConfigs={clientProxyConfigs}
                          setClientProxyConfigs={setClientProxyConfigs}
                        />
                      )}
                    </AccordionContent>
                  </AccordionItem>
                </Accordion>
              )
            })}
          </AccordionContent>
        </AccordionItem>
      </Accordion>
      <Button
        className="mt-4 sticky bottom-3"
        disabled={updateFrpc.isPending}
        onClick={() => {
          handleUpdate()
        }}
      >
        {updateFrpc.isPending ? '正在保存…' : '保存连接与批量配置'}
      </Button>
    </div>
  )
}
