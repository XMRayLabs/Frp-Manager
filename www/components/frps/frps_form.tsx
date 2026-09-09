import { ServerConfig } from '@/types/server'
import { useEffect } from 'react'
import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import { Form } from '@/components/ui/form'
import { ZodIPSchema, ZodPortSchema, ZodStringSchema } from '@/lib/consts'
import { RespCode, Server } from '@/lib/pb/common'
import { updateFRPS } from '@/api/frp'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { HostField, PortField } from '../base/form-field'

const ServerConfigSchema = z.object({
  bindAddr: ZodIPSchema.default('0.0.0.0').optional(),
  bindPort: ZodPortSchema.default(7000),
  proxyBindAddr: ZodIPSchema.optional(),
  vhostHTTPPort: ZodPortSchema.optional(),
  subDomainHost: ZodStringSchema.optional(),
  publicHost: ZodStringSchema.optional(),
  quicBindPort: ZodPortSchema.optional(),
  kcpBindPort: ZodPortSchema.optional(),
})

export const ServerConfigZodSchema = ServerConfigSchema

export interface FRPSFormProps {
  serverID: string
  server: Server
  frpsUrls: string[]
}

const FRPSForm: React.FC<FRPSFormProps> = ({ serverID, server, frpsUrls }) => {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<z.infer<typeof ServerConfigZodSchema>>({
    resolver: zodResolver(ServerConfigZodSchema),
  })

  const updateFrps = useMutation({ mutationFn: updateFRPS })

  useEffect(() => {
    form.reset({})
  }, [form])

  useEffect(() => {
    form.reset({ ...JSON.parse(server?.config || '{}'), publicHost: server.ip })
  }, [form, server])

  const onSubmit = async (values: z.infer<typeof ServerConfigZodSchema>) => {
    try {
      const { publicHost, ...rest } = values
      let resp = await updateFrps.mutateAsync({
        serverIp: publicHost,
        serverId: serverID,
        frpsUrls: frpsUrls,
        // @ts-ignore
        config: Buffer.from(
          JSON.stringify({
            ...JSON.parse(server?.config || '{}'),
            ...rest,
          } as ServerConfig),
        ),
      })
      if (resp.status?.code === RespCode.SUCCESS) await queryClient.invalidateQueries({ queryKey: ['getServer', serverID] })
      toast(resp.status?.code === RespCode.SUCCESS ? t('server.operation.update_success') : t('server.operation.update_failed'), {
        description: resp.status?.message,
      })
    } catch (error) {
      console.error(error)
      toast(t('server.operation.update_title'), {
        description: t('server.operation.update_failed')
      })
    }
  }

  return (
    <div className="flex flex-col w-full pt-2">
      {server.comment && <p className="text-sm text-muted-foreground">{server.comment}</p>}
      {serverID && (
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 px-0.5">
            <HostField name="publicHost" label={t('server.form.public_host')} placeholder='8.8.8.8' control={form.control} defaultValue={server?.ip}/>
            <PortField name="bindPort" label={t('server.form.bind_port')} control={form.control} />
            <details className="rounded-lg border p-4"><summary className="cursor-pointer text-sm font-medium">高级网络参数（按需修改）</summary><div className="mt-4 grid gap-4 sm:grid-cols-2">
            <HostField name="bindAddr" label={t('server.form.bind_addr')} control={form.control} />
            <HostField name="proxyBindAddr" label={t('server.form.proxy_bind_addr')} control={form.control} />
            <PortField name="vhostHTTPPort" label={t('server.form.vhost_http_port')} control={form.control} />
            <HostField name="subDomainHost" label={t('server.form.subdomain_host')} control={form.control} />
            <PortField name="quicBindPort" label={t('server.form.quic_bind_port')} control={form.control} />
            <PortField name="kcpBindPort" label={t('server.form.kcp_bind_port')} control={form.control} />
            </div></details>
            <Button type="submit" disabled={updateFrps.isPending} className="sticky bottom-3">{updateFrps.isPending ? '正在保存…' : '保存服务端配置'}</Button>
          </form>
        </Form>
      )}
    </div>
  )
}

export default FRPSForm
