import { getResourceOwners } from '@/api/resource-owners'
import { OwnerFilter } from '@/components/base/owner-filter'
import { collectPages } from '@/lib/collect-pages'
import { Client } from '@/lib/pb/common'
import { ClientTableSchema, columns as clientColumnsDef } from './client_item'
import { DataTable } from '../base/data_table'

import {
  getSortedRowModel,
  getCoreRowModel,
  ColumnFiltersState,
  useReactTable,
  getFilteredRowModel,
  getPaginationRowModel,
  SortingState,
  PaginationState,
  VisibilityState,
} from '@tanstack/react-table'

import React from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { listClient } from '@/api/client'
import { ClientConfigured } from '@/lib/consts'
import { useStore } from '@nanostores/react'
import { $clientTableRefetchTrigger } from '@/store/refetch-trigger'
import { getClientsStatus } from '@/api/platform'
import { ClientType } from '@/lib/pb/common'
import { ClientStatus_Status } from '@/lib/pb/api_master'
import { Button } from '../ui/button'

export interface ClientListProps {
  Clients: Client[]
  Keyword?: string
  TriggerRefetch?: string
}

export const ClientList: React.FC<ClientListProps> = ({ Clients, Keyword, TriggerRefetch }) => {
  const [ownerFilter, setOwnerFilter] = React.useState('all')
  const [sorting, setSorting] = React.useState<SortingState>([])
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
  const [configFilter, setConfigFilter] = React.useState<'all' | 'valid' | 'invalid'>('all')
  const [runtimeFilter, setRuntimeFilter] = React.useState<'all' | 'online' | 'offline' | 'paused'>('all')
  const [nodeFilter, setNodeFilter] = React.useState<'all' | 'persistent' | 'ephemeral'>('all')
  const [columnVisibility] = React.useState<VisibilityState>({
    runtimeStatus: false,
    ping: false,
    ephemeral: false,
  })
  const globalRefetchTrigger = useStore($clientTableRefetchTrigger)

  const [{ pageIndex, pageSize }, setPagination] = React.useState<PaginationState>({
    pageIndex: 0,
    pageSize: 10,
  })

  const ownersQuery = useQuery({ queryKey: ['resourceOwners', 'client', TriggerRefetch, globalRefetchTrigger], queryFn: () => getResourceOwners('client'), refetchInterval: 30_000 })
  const advancedFilter = ownerFilter !== 'all' || configFilter !== 'all' || runtimeFilter !== 'all' || nodeFilter !== 'all'
  const fetchDataOptions = {
    advancedFilter,
    pageIndex: advancedFilter ? 0 : pageIndex,
    pageSize,
    Keyword,
    TriggerRefetch,
    globalRefetchTrigger,
  }
  const pagination = React.useMemo(
    () => ({
      pageIndex,
      pageSize,
    }),
    [pageIndex, pageSize],
  )

  const dataQuery = useQuery({
    queryKey: ['listClientPage', fetchDataOptions],
    queryFn: async () => {
      if (advancedFilter) {
        const result = await collectPages(async (page, pageSize) => {
          const response = await listClient({ page, pageSize, keyword: Keyword })
          return { total: response.total ?? 0, items: response.clients }
        })
        return { total: result.total, clients: result.items }
      }
      return await listClient({
        page: fetchDataOptions.pageIndex + 1,
        pageSize: fetchDataOptions.pageSize,
        keyword: fetchDataOptions.Keyword,
      })
    },
    placeholderData: keepPreviousData,
    refetchInterval: advancedFilter ? 30_000 : 10_000,
    staleTime: 10_000,
  })

  const allClients = dataQuery.data?.clients ?? Clients
  const clientIds = React.useMemo(() => allClients.map((client) => client.id || '').filter(Boolean), [allClients])
  const statusQuery = useQuery({
    queryKey: ['listClientStatuses', clientIds.join(','), globalRefetchTrigger],
    queryFn: async () => {
      if (clientIds.length === 0) {
        return undefined
      }
      return await getClientsStatus({ clientIds, clientType: ClientType.FRPC })
    },
    enabled: clientIds.length > 0,
    staleTime: 5000,
    refetchInterval: 15000,
    refetchIntervalInBackground: false,
  })

  const rows = React.useMemo(() => {
    return allClients
      .map((client) => {
        const id = client.id || ''
        const status = statusQuery.data?.clients[id]
        const runtimeStatus: ClientTableSchema['runtimeStatus'] = client.stopped
          ? 'paused'
          : status?.status === ClientStatus_Status.ONLINE
            ? 'online'
            : status?.status === ClientStatus_Status.ERROR
              ? 'error'
              : status?.status === ClientStatus_Status.OFFLINE
                ? 'offline'
                : 'unknown'
        const version = status?.version
        return {
          id,
          status: ClientConfigured(client) ? 'valid' : 'invalid',
          runtimeStatus,
          ping: status?.ping ?? Number.MAX_SAFE_INTEGER,
          info: `${runtimeStatus} ${status?.ping ?? ''} ${version?.gitVersion ?? ''} ${version?.platform ?? ''} ${version?.goVersion ?? ''}`,
          clientStatus: status,
          secret: client.secret == undefined ? '' : client.secret,
          config: client.config,
          stopped: client.stopped || false,
          ephemeral: client.ephemeral || false,
          originClient: client,
          clientIds: client.clientIds || [],
        } as ClientTableSchema
      })
      .filter((row) => ownerFilter === 'all' || String(ownersQuery.data?.resources[row.id]) === ownerFilter)
      .filter((row) => configFilter === 'all' || row.status === configFilter)
      .filter((row) => runtimeFilter === 'all' || row.runtimeStatus === runtimeFilter)
      .filter((row) => nodeFilter === 'all' || (nodeFilter === 'ephemeral' ? row.ephemeral : !row.ephemeral))
  }, [allClients, statusQuery.data, configFilter, runtimeFilter, nodeFilter, ownerFilter, ownersQuery.data])

  React.useEffect(() => {
    setPagination((current) => ({ ...current, pageIndex: 0 }))
  }, [Keyword, configFilter, runtimeFilter, nodeFilter, ownerFilter])

  const table = useReactTable({
    data: rows,
    columns: clientColumnsDef,
    manualPagination: !advancedFilter,
    pageCount: advancedFilter ? undefined : Math.ceil((dataQuery.data?.total ?? 0) / pageSize),
    autoResetPageIndex: false,
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    onSortingChange: (updater) => {
      setSorting(updater)
      setPagination((current) => ({ ...current, pageIndex: 0 }))
    },
    onPaginationChange: setPagination,
    onColumnFiltersChange: setColumnFilters,
    getFilteredRowModel: getFilteredRowModel(),
    getSortedRowModel: getSortedRowModel(),
    state: {
      sorting,
      pagination,
      columnFilters,
      columnVisibility,
    },
  })
  React.useEffect(() => {
    if (!advancedFilter && dataQuery.data && !dataQuery.isPlaceholderData) {
      const last = Math.max(0, Math.ceil((dataQuery.data.total ?? 0) / pageSize) - 1)
      if (pageIndex > last) setPagination((current) => ({ ...current, pageIndex: last }))
    }
  }, [advancedFilter, dataQuery.data, dataQuery.isPlaceholderData, pageIndex, pageSize])
  return (
    <DataTable
      loading={dataQuery.isPending}
      error={dataQuery.error?.message}
      onRetry={() => {
        void dataQuery.refetch()
        void statusQuery.refetch()
      }}
      emptyMessage={
        Keyword || advancedFilter
          ? '没有匹配的节点，试试清空搜索或筛选条件。'
          : '还没有节点。点击上方自动接入，复制命令到设备上运行。'
      }
      table={table}
      columns={clientColumnsDef}
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <OwnerFilter value={ownerFilter} onChange={setOwnerFilter} data={ownersQuery.data} loading={ownersQuery.isPending} error={ownersQuery.error} retry={() => { void ownersQuery.refetch() }} />
          <span className="text-xs text-muted-foreground">
            共 {dataQuery.data?.total ?? 0} 个节点 · {advancedFilter ? '全局筛选' : '按页加载 · 排序作用于当前页'}
          </span>
          <select
            aria-label="客户端筛选"
            className="h-9 rounded-md border bg-background px-3 text-sm"
            value={configFilter}
            onChange={(e) => setConfigFilter(e.target.value as any)}
          >
            <option value="all">全部配置</option>
            <option value="valid">已配置</option>
            <option value="invalid">未配置</option>
          </select>
          <select
            className="h-9 rounded-md border bg-background px-3 text-sm"
            value={runtimeFilter}
            onChange={(e) => setRuntimeFilter(e.target.value as any)}
          >
            <option value="all">全部在线状态</option>
            <option value="online">在线</option>
            <option value="offline">离线</option>
            <option value="paused">已暂停</option>
          </select>
          <select
            className="h-9 rounded-md border bg-background px-3 text-sm"
            value={nodeFilter}
            onChange={(e) => setNodeFilter(e.target.value as any)}
          >
            <option value="all">全部节点</option>
            <option value="persistent">常驻节点</option>
            <option value="ephemeral">临时节点</option>
          </select>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setOwnerFilter('all')
              setConfigFilter('all')
              setRuntimeFilter('all')
              setNodeFilter('all')
              setSorting([])
            }}
          >
            重置
          </Button>
        </div>
      }
    />
  )
}
