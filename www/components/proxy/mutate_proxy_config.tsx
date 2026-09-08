'use client'

import { useEffect, useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { useTranslation } from 'react-i18next'
import { ServerSelector } from '../base/server-selector'
import { ClientSelector } from '../base/client-selector'
import { TypedProxyForm } from '../frpc/proxy_form'
import { ProxyType, TypedProxyConfig } from '@/types/proxy'
import { BaseSelector } from '../base/selector'
import { createProxyConfig } from '@/api/proxy'
import { ClientConfig } from '@/types/client'
import { ObjToUint8Array } from '@/lib/utils'
import { VisitPreview } from '../base/visit-preview'
import { ProxyConfig, Server } from '@/lib/pb/common'
import { TypedProxyConfigValid } from '@/lib/consts'
import { toast } from 'sonner'
import { $proxyTableRefetchTrigger } from '@/store/refetch-trigger'
import { Switch } from '../ui/switch'
import { Textarea } from '../ui/textarea'
import { QuickProxyForm } from './quick_proxy_form'
import { nanoid } from 'nanoid'
import { useStore } from '@nanostores/react'
import { $userInfo } from '@/store/user'
import { automaticProxyName } from '@/lib/proxy-name'

export type ProxyConfigMutateDialogProps = {
  overwrite?: boolean
  defaultProxyConfig?: TypedProxyConfig
  defaultOriginalProxyConfig?: ProxyConfig
  disableChangeProxyName?: boolean
  onSuccess?: () => void
}

export const ProxyConfigMutateDialog = ({ ...props }: ProxyConfigMutateDialogProps) => {
  const { t } = useTranslation()

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="outline" className="w-fit">
          {t('proxy.config.create')}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] overflow-auto">
        <DialogHeader>
          <DialogTitle>{t('proxy.config.create_proxy')}</DialogTitle>
          <DialogDescription>{t('proxy.config.create_proxy_description')}</DialogDescription>
        </DialogHeader>
        <ProxyConfigMutateForm {...props} />
      </DialogContent>
    </Dialog>
  )
}

const AdvancedProxyConfigMutateForm = ({
  overwrite,
  defaultProxyConfig,
  defaultOriginalProxyConfig,
  disableChangeProxyName,
  onSuccess,
}: ProxyConfigMutateDialogProps) => {
  const { t } = useTranslation()
  const [newClientID, setNewClientID] = useState<string | undefined>()
  const [newServerID, setNewServerID] = useState<string | undefined>()
  const [proxyConfigs, setProxyConfigs] = useState<TypedProxyConfig[]>([])
  const [proxyName, setProxyName] = useState<string | undefined>(() => nanoid(8))
  const user = useStore($userInfo)
  const [proxyType, setProxyType] = useState<ProxyType>('http')
  const [selectedServer, setSelectedServer] = useState<Server | undefined>()
  const supportedProxyTypes: ProxyType[] = ['http', 'tcp', 'udp']
  // advanced mode toggle
  const [advancedMode, setAdvancedMode] = useState<boolean>(false)
  const [rawConfig, setRawConfig] = useState<string>('{}')

  const createProxyConfigMutation = useMutation({
    mutationKey: ['createProxyConfig', newClientID, newServerID],
    mutationFn: () =>
      createProxyConfig({
        clientId: newClientID!,
        serverId: newServerID!,
        config: ObjToUint8Array({
          proxies: proxyConfigs.map(config => overwrite || defaultProxyConfig ? config : { ...config, name: automaticProxyName(user!.userName!, newClientID!, newServerID!, config, proxyName!) }),
        } as ClientConfig),
        overwrite,
      }),
    onSuccess: () => {
      toast(t('proxy.config.create_success'))
      $proxyTableRefetchTrigger.set(Math.random())
      onSuccess?.()
    },
    onError: (e) => {
      toast(t('proxy.config.create_failed'), {
        description: JSON.stringify(e),
      })
      $proxyTableRefetchTrigger.set(Math.random())
    },
  })

  useEffect(() => {
    if (proxyName && proxyType) {
      setProxyConfigs([{ ...defaultProxyConfig, name: proxyName, type: proxyType }])
    }
  }, [defaultProxyConfig, proxyName, proxyType])

  useEffect(() => {
    if (proxyConfigs) {
      setRawConfig(JSON.stringify(proxyConfigs, null, 2))
    }
  }, [proxyConfigs, setRawConfig])

  useEffect(() => {
    if (defaultProxyConfig && defaultOriginalProxyConfig) {
      setProxyConfigs([defaultProxyConfig])
      setProxyType(defaultProxyConfig.type)
      setProxyName(defaultProxyConfig.name)
      setNewClientID(defaultOriginalProxyConfig.originClientId)
      setNewServerID(defaultOriginalProxyConfig.serverId)
      setRawConfig(JSON.stringify([defaultProxyConfig], null, 2))
    }
  }, [defaultProxyConfig, defaultOriginalProxyConfig])

  return (
    <>
      <Label>{t('proxy.config.select_server')} </Label>
      <ServerSelector setServerID={setNewServerID} serverID={newServerID} setServer={setSelectedServer} />
      <Label>{t('proxy.config.select_client')} </Label>
      <ClientSelector setClientID={setNewClientID} clientID={newClientID} />
      <div className="flex items-center space-x-2 my-2">
        <Label>{t('proxy.config.advanced_mode')}</Label>
        <Switch onCheckedChange={setAdvancedMode} />
      </div>
      {!advancedMode && (
        <>
          <Label>{t('proxy.config.select_proxy_type')} </Label>
          <BaseSelector
            dataList={supportedProxyTypes.map((type) => ({ value: type, label: type }))}
            value={proxyType}
            setValue={(value) => {
              setProxyType(value as ProxyType)
            }}
          />
          {proxyConfigs &&
            selectedServer &&
            proxyConfigs.length > 0 &&
            proxyConfigs[0] &&
            TypedProxyConfigValid(proxyConfigs[0]) && (
              <div className="flex flex-row w-full overflow-auto">
                <div className="flex flex-col">
                  <VisitPreview server={selectedServer} typedProxyConfig={proxyConfigs[0]} />
                </div>
              </div>
            )}
          {proxyName && newClientID && newServerID && (
            <TypedProxyForm
              serverID={newServerID}
              clientID={newClientID}
              proxyName={proxyName}
              defaultProxyConfig={proxyConfigs && proxyConfigs.length > 0 ? proxyConfigs[0] : undefined}
              clientProxyConfigs={proxyConfigs}
              setClientProxyConfigs={setProxyConfigs}
              enablePreview={false}
            />
          )}
        </>
      )}
      {advancedMode && (
        <>
          <Label>{t('proxy.config.raw_json')}</Label>
          <Textarea
            className="w-full h-64 font-mono text-sm p-2 border"
            value={rawConfig}
            onChange={(e) => setRawConfig(e.target.value)}
          />
          <Button
            onClick={() => {
              try {
                const parsed = JSON.parse(rawConfig) as TypedProxyConfig[]
                setProxyConfigs(parsed)
              } catch {
                toast(t('proxy.config.invalid_json'))
                return
              }
            }}
          >
            {t('proxy.config.draft')}
          </Button>
        </>
      )}
      <Button
        disabled={!user?.userName || !newClientID || !newServerID || createProxyConfigMutation.isPending || (advancedMode ? proxyConfigs.length === 0 : !TypedProxyConfigValid(proxyConfigs[0]))}
        onClick={() => {
          if (!TypedProxyConfigValid(proxyConfigs[0])) {
            toast(t('proxy.config.invalid_config'))
            return
          }
          createProxyConfigMutation.mutate()
        }}
      >
        {t('proxy.config.submit')}
      </Button>
    </>
  )
}

export const ProxyConfigMutateForm = (props: ProxyConfigMutateDialogProps) => {
  const { i18n } = useTranslation()
  const zh = i18n.language.startsWith('zh')
  const [advanced, setAdvanced] = useState(false)
  if (props.defaultProxyConfig || props.defaultOriginalProxyConfig || props.overwrite || props.disableChangeProxyName) {
    return <AdvancedProxyConfigMutateForm {...props} />
  }
  return <div className="space-y-4">
    <div className="flex gap-2">
      <Button type="button" variant={advanced ? 'outline' : 'default'} aria-pressed={!advanced} onClick={() => setAdvanced(false)}>{zh ? '快捷配置' : 'Quick setup'}</Button>
      <Button type="button" variant={advanced ? 'default' : 'outline'} aria-pressed={advanced} onClick={() => setAdvanced(true)}>{zh ? '完整配置' : 'Full configuration'}</Button>
    </div>
    <div hidden={advanced}><QuickProxyForm onSuccess={props.onSuccess} /></div>
    <div hidden={!advanced}><AdvancedProxyConfigMutateForm {...props} /></div>
  </div>
}