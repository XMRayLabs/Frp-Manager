import { Copy, Download, ExternalLink, Network, Terminal } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  ExecCommandStr,
  LATEST_DOWNLOADS,
  LATEST_RELEASE_URL,
  LinuxInstallCommand,
  WindowsInstallCommand,
} from '@/lib/consts'
import { copyText } from '@/lib/clipboard'
import type { GetPlatformInfoResponse } from '@/lib/pb/api_user'

type NodeInstallGuideProps = {
  type: 'client' | 'server'
  item: { id?: string; secret?: string }
  platformInfo: GetPlatformInfoResponse
}

function CommandRow({ label, value }: { label: string; value: string }) {
  const { t } = useTranslation()

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium">{label}</span>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7"
          title={t('node_install.copy', { label })}
          onClick={() => copyText(value)}
        >
          <Copy className="size-3.5" />
        </Button>
      </div>
      <pre className="max-h-24 overflow-auto whitespace-pre-wrap break-all rounded-md border bg-muted/50 p-2 text-[11px] leading-5">
        {value}
      </pre>
    </div>
  )
}

export function NodeInstallGuide({ type, item, platformInfo }: NodeInstallGuideProps) {
  const { t } = useTranslation()
  const windowsCommand = WindowsInstallCommand(type, item, platformInfo)
  const linuxCommand = LinuxInstallCommand(type, item, platformInfo)
  const binaryCommand = ExecCommandStr(type, item, platformInfo)

  return (
    <div className="grid gap-4">
      <div className="space-y-1">
        <h4 className="font-semibold">
          {t(type === 'client' ? 'node_install.title_client' : 'node_install.title_server')}
        </h4>
        <p className="text-xs text-muted-foreground">
          {t('node_install.description')}
        </p>
      </div>

      <div className="grid grid-cols-2 gap-2">
        <Button asChild variant="outline" size="sm">
          <a href={LATEST_DOWNLOADS.windowsGui} target="_blank" rel="noopener noreferrer">
            <Download className="mr-2 size-4" />
            {t('node_install.windows_gui')}
          </a>
        </Button>
        <Button asChild variant="outline" size="sm">
          <a href={LATEST_RELEASE_URL} target="_blank" rel="noopener noreferrer">
            <ExternalLink className="mr-2 size-4" />
            {t('node_install.all_platforms')}
          </a>
        </Button>
      </div>

      <div className="grid gap-2 rounded-md border p-3 text-xs">
        <div className="flex items-center gap-2 font-medium">
          <Network className="size-4 text-primary" />
          {t('node_install.endpoints')}
        </div>
        <div className="grid grid-cols-[3rem_1fr] gap-2">
          <span className="text-muted-foreground">{t('node_install.api')}</span>
          <code className="break-all">{platformInfo.clientApiUrl}</code>
          <span className="text-muted-foreground">{t('node_install.rpc')}</span>
          <code className="break-all">{platformInfo.clientRpcUrl}</code>
        </div>
      </div>

      <div className="flex items-center gap-2 text-xs font-medium">
        <Terminal className="size-4 text-primary" />
        {t('node_install.install_connect')}
      </div>
      <CommandRow label={t('node_install.windows_powershell')} value={windowsCommand} />
      <CommandRow label={t('node_install.linux_systemd')} value={linuxCommand} />
      <CommandRow label={t('node_install.existing_binary')} value={binaryCommand} />
    </div>
  )
}
