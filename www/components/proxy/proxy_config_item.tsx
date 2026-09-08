import { ColumnDef, Row } from '@tanstack/react-table'
import React from 'react'
import { useTranslation } from 'react-i18next'
import { VisitPreview } from '../base/visit-preview'
import { useQuery } from '@tanstack/react-query'
import { getServer } from '@/api/server'
import { ProxyType, TypedProxyConfig } from '@/types/proxy'
import { ProxyConfigActions } from './proxy_config_actions'
import { ProxyConfig } from '@/lib/pb/common'
import { Badge } from '../ui/badge'
import { DataTableColumnHeader } from '../base/column_header'

export type ProxyConfigTableSchema = {
  id?: number | string
  serverID: string
  clientID: string
  name: string
  type: ProxyType
  status: string
  localIP?: string
  localPort?: number
  remotePort?: number
  visitPreview: string
  config?: string
  originalProxyConfig: ProxyConfig
  stopped: boolean
}

export const columns: ColumnDef<ProxyConfigTableSchema>[] = [
  {
    accessorKey: 'name',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('proxy.item.proxy_name')} />
    },
    cell: ({ row }) => {
      return <div className="font-mono text-nowrap">{row.original.name}</div>
    },
  },
  {
    accessorKey: 'type',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('proxy.item.proxy_type')} />
    },
    cell: ({ row }) => {
      return <div className="font-mono text-nowrap">{row.original.type}</div>
    },
  },
  {
    accessorKey: 'clientID',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('proxy.item.client_id')} />
    },
    cell: ({ row }) => {
      return <div className="font-mono text-nowrap">{row.original.originalProxyConfig.originClientId}</div>
    },
  },
  {
    accessorKey: 'serverID',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('proxy.item.server_id')} />
    },
    cell: ({ row }) => {
      return <div className="font-mono text-nowrap">{row.original.serverID}</div>
    },
  },
  {
    accessorKey: 'status',
    header: function Header({ column }: any) {
      const { t } = useTranslation()
      return <DataTableColumnHeader column={column} title={t('proxy.item.status')} />
    },
    cell: ProxyStatus,
  },
  {
    accessorKey: 'remotePort',
    header: function Header({ column }: any) {
      return <DataTableColumnHeader column={column} title="远程端口" />
    },
    cell: ({ row }) => row.original.remotePort || '-',
  },
  {
    accessorKey: 'localPort',
    header: function Header({ column }: any) {
      return <DataTableColumnHeader column={column} title="本地端口" />
    },
    cell: ({ row }) => row.original.localPort || '-',
  },
  {
    accessorKey: 'visitPreview',
    header: function Header() {
      const { t } = useTranslation()
      return t('proxy.item.visit_preview')
    },
    cell: ({ row }) => {
      return <VisitPreviewField row={row} />
    },
  },
  {
    id: 'action',
    cell: ({ row }) => {
      return (
        <ProxyConfigActions
          row={row}
          serverID={row.original.serverID}
          clientID={row.original.clientID}
          name={row.original.name}
        />
      )
    },
  },
]

function VisitPreviewField({ row }: { row: Row<ProxyConfigTableSchema> }) {
  const { data: server } = useQuery({
    queryKey: ['getServer', row.original.serverID],
    queryFn: () => {
      return getServer({ serverId: row.original.serverID })
    },
  })

  const typedProxyConfig = JSON.parse(row.original.config || '{}') as TypedProxyConfig

  return <VisitPreview server={server?.server || { frpsUrls: [] }} typedProxyConfig={typedProxyConfig} />
}

function ProxyStatus({ row }: { row: Row<ProxyConfigTableSchema> }) {
  function getStatusColor(status: string): string {
    switch (status) {
      case 'new':
        return 'text-blue-500'
      case 'wait start':
        return 'text-yellow-400'
      case 'start error':
        return 'text-red-500'
      case 'running':
        return 'text-green-500'
      case 'check failed':
        return 'text-orange-500'
      case 'error':
        return 'text-red-600'
      default:
        return 'text-gray-500'
    }
  }

  return (
    <div className="flex items-center gap-2 flex-row text-nowrap">
      <Badge
        variant={'secondary'}
        className={`p-2 border rounded font-mono w-fit ${getStatusColor(row.original.status || 'unknown')} text-nowrap rounded-full h-6`}
      >
        {row.original.status || 'unknown'}
      </Badge>
    </div>
  )
}
