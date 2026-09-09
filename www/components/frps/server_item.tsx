import { RenameNodeDialog } from '../base/rename-node-dialog'
import { ColumnDef, Table } from '@tanstack/react-table'
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
import React from 'react'
import { ClientEnvFile, ExecCommandStr } from '@/lib/consts'
import { useMutation, useQuery } from '@tanstack/react-query'
import { deleteServer } from '@/api/server'
import { useRouter } from 'next/router'
import { useStore } from '@nanostores/react'
import { $platformInfo, $userInfo } from '@/store/user'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { ClientType } from '@/lib/pb/common'
import { ClientStatus, ClientStatus_Status } from '@/lib/pb/api_master'
import { Badge } from '../ui/badge'
import { ClientDetail } from '../base/client_detail'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { $serverTableRefetchTrigger } from '@/store/refetch-trigger'
import { DataTableColumnHeader } from '../base/column_header'
import { copyText } from '@/lib/clipboard'
import { NodeInstallGuide } from '../base/node-install-guide'

export type ServerTableSchema = {
  id: string
  status: 'invalid' | 'valid'
  runtimeStatus: 'online' | 'offline' | 'error' | 'paused' | 'unknown'
  ping: number
  secret: string
  stopped: boolean
  info?: string
  clientStatus?: ClientStatus
  ip: string
  config?: string
  frpsUrls: string[]
}

export const columns: ColumnDef<ServerTableSchema>[] = [
  {
    accessorKey: 'id',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('server.id')} />
    },
    cell: ({ row }) => {
      return <ServerID server={row.original} />
    },
  },
  {
    accessorKey: 'status',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('server.status')} />
    },
    cell: ({ row }) => {
      function Cell({ server }: { server: ServerTableSchema }) {
        const { t } = useTranslation()
        return (
          <div className={`font-medium ${server.status === 'valid' ? 'text-green-500' : 'text-red-500'} min-w-12`}>
            {server.status === 'valid' ? t('server.status_configured') : t('server.status_unconfigured')}
          </div>
        )
      }
      return <Cell server={row.original} />
    },
  },
  {
    accessorKey: 'info',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('server.info')} />
    },
    cell: ({ row }) => {
      const server = row.original
      return <ServerInfo server={server} />
    },
  },
  {
    accessorKey: 'ip',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('server.ip')} />
    },
    cell: ({ row }) => {
      return row.original.ip
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
    accessorKey: 'secret',
    header: function Header() {
      const { t } = useTranslation()
      return t('server.secret')
    },
    cell: ({ row }) => {
      const server = row.original
      return <ServerSecret server={server} />
    },
  },
  {
    id: 'action',
    cell: ({ row, table }) => {
      const server = row.original
      return <ServerActions server={server} table={table as Table<ServerTableSchema>} />
    },
  },
]

export const ServerID = ({ server }: { server: ServerTableSchema }) => {
  const platformInfo = useStore($platformInfo)

  if (!platformInfo) {
    return (
      <Button variant="link" className="px-0">
        {server.id}
      </Button>
    )
  }

  return (
    <Dialog>
      <DialogTrigger asChild>
        <Button variant="link" className="px-0 font-mono">
          {server.id}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-h-[90vh] w-[min(94vw,44rem)] max-w-[44rem] overflow-y-auto">
        <DialogTitle className="sr-only">Install server node</DialogTitle>
        <NodeInstallGuide type="server" item={server} platformInfo={platformInfo} />
      </DialogContent>
    </Dialog>
  )
}

export const ServerInfo = ({ server }: { server: ServerTableSchema }) => {
  const { t } = useTranslation()

  const trans = (info: ClientStatus | undefined) => {
    let statusText: 'server.status_online' | 'server.status_offline' |
      'server.status_error' | 'server.status_pause' |
      'server.status_unknown' = 'server.status_unknown'
    if (info === undefined) {
      return statusText
    }
    if (info.status === ClientStatus_Status.ONLINE) {
      statusText = 'server.status_online'
      if (server.stopped) {
        statusText = 'server.status_pause'
      }
    } else if (info.status === ClientStatus_Status.OFFLINE) {
      statusText = 'server.status_offline'
    } else if (info.status === ClientStatus_Status.ERROR) {
      statusText = 'server.status_error'
    }
    return statusText
  }

  const status = server.clientStatus
  const ping = status && status.ping >= 0 ? `${status.ping}ms` : '-'
  const infoColor =
    status?.status === ClientStatus_Status.ONLINE ? (
      server.stopped ? 'text-yellow-500' : 'text-green-500') : 'text-red-500'

  return (
    <div className="flex items-center gap-2 flex-row">
      <Badge variant={"secondary"} className={`p-2 border rounded font-mono w-fit ${infoColor} text-nowrap rounded-full h-6`}>
        {`${ping},${t(trans(status))}`}
      </Badge>
      {status?.version &&
        <ClientDetail clientStatus={status} />
      }
    </div>
  )
}

export const ServerSecret = ({ server }: { server: ServerTableSchema }) => {
  const { t } = useTranslation()
  const platformInfo = useStore($platformInfo)
  const [copyState, setCopyState] = React.useState<'idle' | 'success' | 'failed'>('idle')
  const startCommand = platformInfo ? ExecCommandStr('server', server, platformInfo) : ''

  const handleCopyStartCommand = async () => {
    if (!platformInfo) {
      setCopyState('failed')
      toast(t('server.actions_menu.copy_failed'))
      window.setTimeout(() => setCopyState('idle'), 2500)
      return
    }

    try {
      await copyText(startCommand)
      setCopyState('success')
      toast(t('server.actions_menu.copy_success'))
      window.setTimeout(() => setCopyState('idle'), 1500)
    } catch {
      setCopyState('failed')
      toast(t('server.actions_menu.copy_failed'))
      window.setTimeout(() => setCopyState('idle'), 2500)
    }
  }

  if (!platformInfo) {
    return <span className="font-mono text-muted-foreground">********</span>
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="h-8 gap-2 px-2 font-mono" title={t('server.start.title')}>
          <Eye className="h-4 w-4" />
          <span aria-hidden="true">********</span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[32rem] max-w-[95vw]">
        <div className="grid gap-4">
          <div className="space-y-2">
            <h4 className="font-medium leading-none">{t('server.start.title')}</h4>
            <p className="text-sm text-muted-foreground">
              {t('server.start.description')} (<a className='text-blue-500' href='https://github.com/XMRayLabs/Frp-Manager/releases' target="_blank" rel="noopener noreferrer">{t('common.download')}</a>)
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

export interface ServerItemProps {
  server: ServerTableSchema
  table: Table<ServerTableSchema>
}

export const ServerActions: React.FC<ServerItemProps> = ({ server, table }) => {
  const { t } = useTranslation()
  const [renameOpen, setRenameOpen] = React.useState(false)
  const router = useRouter()
  const platformInfo = useStore($platformInfo)
  const userInfo = useStore($userInfo)
  const isAdmin = userInfo?.role === 'admin'

  const removeServer = useMutation({
    mutationFn: deleteServer,
    onSuccess: () => {
      toast(t('server.delete.success'))
      $serverTableRefetchTrigger.set(Math.random())
    },
    onError: (e) => {
      toast(t('server.delete.failed'), {
        description: e.message,
      })
      $serverTableRefetchTrigger.set(Math.random())
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
      <RenameNodeDialog kind="server" id={server.id} open={renameOpen} onOpenChange={setRenameOpen} />
    <Dialog>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" className="h-8 w-8 p-0">
            <span className="sr-only">{t('server.actions_menu.open_menu')}</span>
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuLabel>{t('server.actions_menu.title')}</DropdownMenuLabel>
          <DropdownMenuItem onSelect={() => setRenameOpen(true)}>修改 ID</DropdownMenuItem>

          <DropdownMenuSeparator />
          {isAdmin && (
            <DropdownMenuItem
              onClick={() => {
                router.push({ pathname: '/serveredit', query: { serverID: server.id } })
              }}
            >
              {t('server.actions_menu.edit_config')}
            </DropdownMenuItem>
          )}
          <DropdownMenuItem
            onClick={() => {
              try {
                if (platformInfo) {
                  createAndDownloadFile('.env', ClientEnvFile(server, platformInfo))
                }
              } catch (error) {
                toast(t('server.actions_menu.download_failed'), {
                  description: JSON.stringify(error),
                })
              }
            }}
          >
            {t('server.actions_menu.download_config')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              router.push({ pathname: '/streamlog', query: { serverID: server.id, clientType: ClientType.FRPS.toString() } })
            }}
          >
            {t('server.actions_menu.realtime_log')}
          </DropdownMenuItem>
          {isAdmin && (
            <DropdownMenuItem
              onClick={() => {
                router.push({ pathname: '/console', query: { clientID: server.id, clientType: ClientType.FRPS.toString() } })
              }}
            >
              {t('server.actions_menu.remote_terminal')}
            </DropdownMenuItem>
          )}
          {isAdmin && (
            <DialogTrigger asChild>
              <DropdownMenuItem className="text-destructive">{t('server.actions_menu.delete')}</DropdownMenuItem>
            </DialogTrigger>
          )}
        </DropdownMenuContent>
      </DropdownMenu>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('server.delete.title')}</DialogTitle>
          <DialogDescription>
            <p className="text-destructive">{t('server.delete.description')}</p>
            <p className="text-gray-500 border-l-4 border-gray-500 pl-4 py-2">
              {t('server.delete.warning')}
            </p>
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <DialogClose asChild>
            <Button type="submit" onClick={() => removeServer.mutate({ serverId: server.id })}>
              {t('server.delete.confirm')}
            </Button>
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
    </>
  )
}
