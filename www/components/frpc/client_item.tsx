import { RenameNodeDialog } from '../base/rename-node-dialog'
import { ColumnDef, Table, TableMeta } from '@tanstack/react-table'
import { Eye, MoreHorizontal } from 'lucide-react'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import React, { useState } from 'react'
import { ClientEnvFile, ExecCommandStr, LinuxInstallCommand } from '@/lib/consts'
import { useMutation, useQuery } from '@tanstack/react-query'
import { deleteClient, listClient } from '@/api/client'
import { useRouter } from 'next/router'
import { useStore } from '@nanostores/react'
import { $platformInfo, $userInfo } from '@/store/user'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Client, ClientType } from '@/lib/pb/common'
import { ClientStatus, ClientStatus_Status } from '@/lib/pb/api_master'
import { startFrpc, stopFrpc } from '@/api/frp'
import { Badge } from '../ui/badge'
import { ClientDetail } from '../base/client_detail'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { $clientTableRefetchTrigger } from '@/store/refetch-trigger'
import { NeedUpgrade } from '@/config/notify'
import { ClientUpgradeDialog } from '../base/client_upgrade_dialog'
import { DataTableColumnHeader } from '../base/column_header'
import { copyText } from '@/lib/clipboard'
import { NodeInstallGuide } from '../base/node-install-guide'

export type ClientTableSchema = {
  id: string
  status: 'invalid' | 'valid'
  runtimeStatus: 'online' | 'offline' | 'error' | 'paused' | 'unknown'
  ping: number
  secret: string
  stopped: boolean
  ephemeral: boolean
  info?: string
  clientStatus?: ClientStatus
  config?: string
  originClient: Client
  clientIds: string[]
}

export interface TableMetaType extends TableMeta<ClientTableSchema> {
  refetch: () => void
}

export const columns: ColumnDef<ClientTableSchema>[] = [
  {
    accessorKey: 'id',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('client.id')} />
    },
    cell: ({ row }) => {
      return <ClientID client={row.original} />
    },
  },
  {
    accessorKey: 'status',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('client.status')} />
    },
    cell: ({ row }) => {
      function Cell({ client }: { client: ClientTableSchema }) {
        const { t } = useTranslation()
        return (
          <div className={`font-medium ${client.status === 'valid' ? 'text-green-500' : 'text-red-500'} min-w-12`}>
            {client.status === 'valid' ? t('client.status_configured') : t('client.status_unconfigured')}
          </div>
        )
      }
      return <Cell client={row.original} />
    },
  },
  {
    accessorKey: 'info',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('client.info')} />
    },
    cell: ({ row }) => {
      const client = row.original
      return <ClientInfo client={client} />
    },
  },
  {
    accessorKey: 'runtimeStatus',
    enableHiding: true,
    cell: () => null,
  },
  {
    accessorKey: 'ping',
    enableHiding: true,
    cell: () => null,
  },
  {
    accessorKey: 'ephemeral',
    enableHiding: true,
    cell: () => null,
  },
  {
    accessorKey: 'secret',
    header: function Header() {
      const { t } = useTranslation()
      return t('client.secret')
    },
    cell: ({ row }) => {
      const client = row.original
      return <ClientSecret client={client} />
    },
  },
  {
    id: 'action',
    cell: ({ row, table }) => {
      const client = row.original
      return (
        <ClientActions
          client={client}
          table={table as Table<ClientTableSchema> & { options: { meta: TableMetaType } }}
        />
      )
    },
  },
]

export const ClientID = ({ client }: { client: ClientTableSchema }) => {
  const platformInfo = useStore($platformInfo)

  if (!platformInfo) {
    return (
      <Button variant="link" className="px-0">
        {client.id}
      </Button>
    )
  }

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="link" className="px-0 font-mono">
          {client.id}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] w-[min(94vw,44rem)] max-w-[44rem] overflow-y-auto">
        <DialogTitle className="sr-only">Install client node</DialogTitle>
        <NodeInstallGuide type="client" item={client} platformInfo={platformInfo} />
      </DialogContent>
    </Dialog>
  )
}

export const ClientInfo = ({ client }: { client: ClientTableSchema }) => {
  const { t } = useTranslation()
  const platformInfo = useStore($platformInfo)

  const trans = (info: ClientStatus | undefined) => {
    let statusText:
      | 'client.status_online'
      | 'client.status_offline'
      | 'client.status_error'
      | 'client.status_pause'
      | 'client.status_unknown' = 'client.status_unknown'
    if (info === undefined) {
      return statusText
    }
    if (info.status === ClientStatus_Status.ONLINE) {
      statusText = 'client.status_online'
      if (client.stopped) {
        statusText = 'client.status_pause'
      }
    } else if (info.status === ClientStatus_Status.OFFLINE) {
      statusText = 'client.status_offline'
    } else if (info.status === ClientStatus_Status.ERROR) {
      statusText = 'client.status_error'
    }
    return statusText
  }

  const status = client.clientStatus
  const ping = status && status.ping >= 0 ? `${status.ping}ms` : '-'
  const infoColor =
    status?.status === ClientStatus_Status.ONLINE
      ? client.stopped
        ? 'text-yellow-500'
        : 'text-green-500'
      : 'text-red-500'

  return (
    <div className="flex items-center gap-2 flex-row">
      <Badge variant={'secondary'} className={`p-2 border font-mono w-fit ${infoColor} text-nowrap rounded-full h-6`}>
        {`${ping},${t(trans(status))}`}
      </Badge>
      {status?.version && <ClientDetail clientStatus={status} />}
      {NeedUpgrade(status?.version, platformInfo?.version) && (
        <Badge variant={'destructive'} className={`p-2 border font-mono w-fit text-nowrap rounded-full h-6`}>
          {t('client.need_upgrade')}
        </Badge>
      )}
      {client.originClient.ephemeral && (
        <Badge variant={'secondary'} className={`p-2 border font-mono w-fit text-nowrap rounded-full h-6`}>
          {t('client.temp_node')}
        </Badge>
      )}
    </div>
  )
}

export const ClientSecret = ({ client }: { client: ClientTableSchema }) => {
  const { t } = useTranslation()
  const platformInfo = useStore($platformInfo)
  const [copyState, setCopyState] = React.useState<'idle' | 'success' | 'failed'>('idle')
  const startCommand = platformInfo ? ExecCommandStr('client', client, platformInfo) : ''

  const handleCopyStartCommand = async () => {
    if (!platformInfo) {
      setCopyState('failed')
      toast(t('client.actions_menu.copy_failed'))
      window.setTimeout(() => setCopyState('idle'), 2500)
      return
    }

    try {
      await copyText(startCommand)
      setCopyState('success')
      toast(t('client.actions_menu.copy_success'))
      window.setTimeout(() => setCopyState('idle'), 1500)
    } catch (error) {
      setCopyState('failed')
      toast(t('client.actions_menu.copy_failed'), {
        description: error instanceof Error ? error.message : JSON.stringify(error),
      })
      window.setTimeout(() => setCopyState('idle'), 2500)
    }
  }

  if (!platformInfo) {
    return <span className="font-mono text-muted-foreground">********</span>
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="h-8 gap-2 px-2 font-mono" title={t('client.start.title')}>
          <Eye className="h-4 w-4" />
          <span aria-hidden="true">********</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[32rem] max-w-[95vw]">
        <div className="grid gap-4">
          <div className="space-y-2">
            <h4 className="font-medium leading-none">{t('client.start.title')}</h4>
            <p className="text-sm text-muted-foreground">
              {t('client.start.description')} (
              <a
                className="text-blue-500"
                href="https://github.com/XMRayLabs/Frp-Manager/releases"
                target="_blank"
                rel="noopener noreferrer"
              >
                {t('common.download')}
              </a>
              )
            </p>
          </div>
          <div className="grid gap-2">
            <pre className="bg-muted p-3 rounded-md font-mono text-sm overflow-x-auto whitespace-pre-wrap break-all">
              {startCommand}
            </pre>
            <Button
              size="sm"
              variant="outline"
              className="w-full"
              onPointerDown={(event) => {
                event.preventDefault()
                event.stopPropagation()
                void handleCopyStartCommand()
              }}
              disabled={!platformInfo}
            >
              {copyState === 'success' ? 'Copied' : copyState === 'failed' ? 'Copy failed' : t('common.copy')}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}

export interface ClientItemProps {
  client: ClientTableSchema
  table: Table<ClientTableSchema>
}

export const ClientActions: React.FC<ClientItemProps> = ({ client, table }) => {
  const { t } = useTranslation()
  const [renameOpen, setRenameOpen] = React.useState(false)
  const router = useRouter()
  const platformInfo = useStore($platformInfo)
  const userInfo = useStore($userInfo)
  const isAdmin = userInfo?.role === 'admin'
  const [upgradeDialogOpen, setUpgradeDialogOpen] = useState(false)

  const removeClient = useMutation({
    mutationFn: deleteClient,
    onSuccess: () => {
      toast(t('client.delete.success'))
      $clientTableRefetchTrigger.set(Math.random())
    },
    onError: (e) => {
      toast(t('client.delete.failed'), {
        description: e.message,
      })
      $clientTableRefetchTrigger.set(Math.random())
    },
  })

  const stopClient = useMutation({
    mutationFn: stopFrpc,
    onSuccess: () => {
      toast(t('client.operation.stop_success'))
      $clientTableRefetchTrigger.set(Math.random())
    },
    onError: (e) => {
      toast(t('client.operation.stop_failed'), {
        description: e.message,
      })
      $clientTableRefetchTrigger.set(Math.random())
    },
  })

  const startClient = useMutation({
    mutationFn: startFrpc,
    onSuccess: () => {
      toast(t('client.operation.start_success'))
      $clientTableRefetchTrigger.set(Math.random())
    },
    onError: (e) => {
      toast(t('client.operation.start_failed'), {
        description: e.message,
      })
      $clientTableRefetchTrigger.set(Math.random())
    },
  })

  const createAndDownloadFile = (fileName: string, content: string) => {
    const aTag = document.createElement('a')
    const blob = new Blob([content])
    aTag.download = fileName
    aTag.href = URL.createObjectURL(blob)
    aTag.click()
    URL.revokeObjectURL(aTag.href)
  }

  return (
    <>
      <RenameNodeDialog kind="client" id={client.id} open={renameOpen} onOpenChange={setRenameOpen} />
      <ClientUpgradeDialog
        open={upgradeDialogOpen}
        onOpenChange={setUpgradeDialogOpen}
        clientId={client.id}
        defaultUseGithubProxy={false}
        defaultServiceName="frpp"
        onDispatched={() => {
          $clientTableRefetchTrigger.set(Math.random())
        }}
      />

      <Dialog>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" className="h-8 w-8 p-0">
            <span className="sr-only">{t('client.actions_menu.open_menu')}</span>
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>{t('client.actions_menu.title')}</DropdownMenuLabel>
          <DropdownMenuItem onSelect={() => setRenameOpen(true)}>修改 ID</DropdownMenuItem>

          <DropdownMenuItem
            onClick={async () => {
              try {
                if (platformInfo) {
                  await copyText(LinuxInstallCommand('client', client, platformInfo))
                  toast(t('client.actions_menu.copy_success'))
                } else {
                  toast(t('client.actions_menu.copy_failed'))
                }
              } catch (error) {
                toast(t('client.actions_menu.copy_failed'), {
                  description: JSON.stringify(error),
                })
              }
            }}
          >
            {t('client.actions_menu.copy_install_command')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              router.push({ pathname: '/clientedit', query: { clientID: client.id } })
            }}
          >
            {t('client.actions_menu.edit_config')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              try {
                if (platformInfo) {
                  createAndDownloadFile('.env', ClientEnvFile(client, platformInfo))
                }
              } catch (error) {
                toast(t('client.actions_menu.download_failed'), {
                  description: JSON.stringify(error),
                })
              }
            }}
          >
            {t('client.actions_menu.download_config')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              router.push({
                pathname: '/streamlog',
                query: { clientID: client.id, clientType: ClientType.FRPC.toString() },
              })
            }}
          >
            {t('client.actions_menu.realtime_log')}
          </DropdownMenuItem>
          {isAdmin && (
            <DropdownMenuItem
              onClick={() => {
                router.push({
                  pathname: '/console',
                  query: { clientID: client.id, clientType: ClientType.FRPC.toString() },
                })
              }}
            >
              {t('client.actions_menu.remote_terminal')}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={(e) => {
              e.preventDefault()
              e.stopPropagation()
              setUpgradeDialogOpen(true)
            }}
          >
            {t('client.actions_menu.upgrade')}
          </DropdownMenuItem>
          {!client.stopped && (
            <DropdownMenuItem className="text-destructive" onClick={() => stopClient.mutate({ clientId: client.id })}>
              {t('client.actions_menu.pause')}
            </DropdownMenuItem>
          )}
          {client.stopped && (
            <DropdownMenuItem onClick={() => startClient.mutate({ clientId: client.id })}>
              {t('client.actions_menu.resume')}
            </DropdownMenuItem>
          )}
          <DialogTrigger asChild>
            <DropdownMenuItem className="text-destructive">{t('client.actions_menu.delete')}</DropdownMenuItem>
          </DialogTrigger>
        </DropdownMenuContent>
      </DropdownMenu>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('client.delete.title')}</DialogTitle>
          <DialogDescription>
            <p className="text-destructive">{t('client.delete.description')}</p>
            <p className="text-gray-500 border-l-4 border-gray-500 pl-4 py-2">{t('client.delete.warning')}</p>
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button type="submit" onClick={() => removeClient.mutate({ clientId: client.id })}>
              {t('client.delete.confirm')}
            </Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
      </Dialog>
    </>
  )
}
