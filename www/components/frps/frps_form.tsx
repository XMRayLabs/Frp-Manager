import { ServerConfig } from '@/types/server'
import { useEffect, useState } from 'react'
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
import { Input } from '../ui/input'
import { websiteEntry } from '@/lib/website-mapping'
import { HostField, PortField } from '../base/form-field'

const ServerConfigSchema = z.object({
  bindAddr: ZodIPSchema.default('0.0.0.0').optional(),
  bindPort: ZodPortSchema.default(7000),
  proxyBindAddr: ZodIPSchema.optional(),
  vhostHTTPPort: z.preprocess(value => value === '' || value === 0 || value == null ? undefined : value, ZodPortSchema.optional()),
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
  const [websiteEnabled, setWebsiteEnabled] = useState(!!server.websiteBaseUrl)
  const [websiteBaseURL, setWebsiteBaseURL] = useState(server.websiteBaseUrl || '')
  const [websiteDomain, setWebsiteDomain] = useState(() => { try { return JSON.parse(server.config || '{}').subDomainHost || '' } catch { return '' } })
  const [websitePort, setWebsitePort] = useState(() => { try { return String(JSON.parse(server.config || '{}').vhostHTTPPort || 9080) } catch { return '9080' } })
  const form = useForm<z.infer<typeof ServerConfigZodSchema>>({
    resolver: zodResolver(ServerConfigZodSchema),
  })

  const updateFrps = useMutation({ mutationFn: updateFRPS })

  useEffect(() => {
    form.reset({})
  }, [form])

  useEffect(() => {
    form.reset({ ...JSON.parse(server?.config || '{}'), publicHost: server.ip })
    setWebsiteEnabled(!!server.websiteBaseUrl)
    setWebsiteBaseURL(server.websiteBaseUrl || '')
    const cfg = JSON.parse(server.config || '{}')
    setWebsiteDomain(cfg.subDomainHost || '')
    setWebsitePort(String(cfg.vhostHTTPPort || 9080))
  }, [form, server])

  const onSubmit = async (values: z.infer<typeof ServerConfigZodSchema>) => {
    try {
      const { publicHost, ...rest } = values
      const config = { ...JSON.parse(server.config || '{}'), ...rest,
        ...(websiteEnabled ? { subDomainHost: websiteDomain.trim().toLowerCase(), vhostHTTPPort: Number(websitePort) } : {}) }
      if (websiteEnabled && !websiteEntry(websiteBaseURL.trim(), JSON.stringify(config))) throw new Error('请填写有效的 HTTP 入口端口，且对外网站地址的域名与子域名后缀一致')
      let resp = await updateFrps.mutateAsync({
        websiteBaseUrl: websiteEnabled ? websiteBaseURL.trim() : '',
        comment: server.comment,
        serverIp: publicHost,
        serverId: serverID,
        frpsUrls: frpsUrls,
        // @ts-ignore
        config: Buffer.from(
          JSON.stringify(config as ServerConfig),
        ),
      })
      if (resp.status?.code === RespCode.SUCCESS) await queryClient.invalidateQueries({ queryKey: ['getServer', serverID] })
      toast(resp.status?.code === RespCode.SUCCESS ? t('server.operation.update_success') : t('server.operation.update_failed'), {
        description: resp.status?.message,
      })
    } catch (error) {
      console.error(error)
      toast.error(error instanceof Error ? error.message : t('server.operation.update_failed'))
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
            <fieldset disabled={updateFrps.isPending} className="rounded-lg border p-4 space-y-3">
              <label className="flex items-center gap-2 text-sm font-medium"><input type="checkbox" checked={websiteEnabled} onChange={e => setWebsiteEnabled(e.target.checked)} />网站域名映射（可选）</label>
              <p className="text-xs text-muted-foreground">默认不启用。仅为需要发布网站的服务端配置；TCP 和 HTTP/SOCKS5 代理无需设置。</p>
              {websiteEnabled && <>
                <label className="grid gap-2 text-sm">子域名后缀<Input required placeholder="frp.xmray.de" value={websiteDomain} onChange={e => setWebsiteDomain(e.target.value)} /></label>
                <label className="grid gap-2 text-sm">HTTP 入口端口<Input required type="number" min={1} max={65535} value={websitePort} onChange={e => setWebsitePort(e.target.value)} /><span className="text-xs text-muted-foreground">例如 9080，供 Nginx 转发，不要填写已被面板或 Nginx 占用的端口。</span></label>
                <label className="grid gap-2 text-sm">对外网站地址<Input required placeholder="https://frp.xmray.de" value={websiteBaseURL} onChange={e => setWebsiteBaseURL(e.target.value)} /><span className="text-xs text-muted-foreground">填写入口协议、域名后缀及可选端口。用户地址自动追加网站名与账号，例如 qwen-admin.frp.xmray.de。</span></label>
                <p className="text-xs text-muted-foreground">先将服务端升级至 1.1.7 或更新版本，再为 *.{websiteDomain || '域名后缀'} 配置 DNS、泛域名证书和 Nginx，转发至本服务端 HTTP 入口并保留 Host。多个服务端应分别使用不同后缀与入口；保存服务端配置可能短暂重连。</p>
              </>}
            </fieldset>
            <details className="rounded-lg border p-4"><summary className="cursor-pointer text-sm font-medium">高级网络参数（按需修改）</summary><div className="mt-4 grid gap-4 sm:grid-cols-2">
            <HostField name="bindAddr" label={t('server.form.bind_addr')} control={form.control} />
            <HostField name="proxyBindAddr" label={t('server.form.proxy_bind_addr')} control={form.control} />
            {!websiteEnabled && <><PortField name="vhostHTTPPort" label={t('server.form.vhost_http_port')} control={form.control} /><HostField name="subDomainHost" label={t('server.form.subdomain_host')} control={form.control} /></>}
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
