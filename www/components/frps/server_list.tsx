import { useRouter } from 'next/router'
import { getNodeOverview } from '@/api/platform'
import { collectPages } from '@/lib/collect-pages'
import { Server } from '@/lib/pb/common'
import { ServerTableSchema, columns as serverColumnsDef } from './server_item'
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
import { listServer } from '@/api/server'
import { $serverTableRefetchTrigger } from '@/store/refetch-trigger'
import { useStore } from '@nanostores/react'
import { getClientsStatus } from '@/api/platform'
import { ClientType } from '@/lib/pb/common'
import { ClientStatus_Status } from '@/lib/pb/api_master'
import { Button } from '../ui/button'

export interface ServerListProps {
  Servers: Server[]
  Keyword?: string
  TriggerRefetch?: string
}

export const ServerList: React.FC<ServerListProps> = ({ Servers, Keyword, TriggerRefetch }) => {
  const router = useRouter()
  const pendingOnly = router.query.pending === '1'
  const pendingQuery = useQuery({ queryKey: ['nodeOverview'], queryFn: getNodeOverview, enabled: pendingOnly, refetchInterval: 15000 })
  const pendingIDs = React.useMemo(() => new Set(pendingQuery.data?.servers.pendingIds || []), [pendingQuery.data])
  const [sorting, setSorting] = React.useState<SortingState>([])
  const [columnFilters, setColumnFilters] = React.useState<ColumnFiltersState>([])
  const [configFilter, setConfigFilter] = React.useState<'all' | 'valid' | 'invalid'>('all')
  const [runtimeFilter, setRuntimeFilter] = React.useState<'all' | 'online' | 'offline' | 'paused'>('all')
  const [columnVisibility] = React.useState<VisibilityState>({
    runtimeStatus: false,
    ping: false,
  })
  const globalRefetchTrigger = useStore($serverTableRefetchTrigger)

  const [{ pageIndex, pageSize }, setPagination] = React.useState<PaginationState>({
    pageIndex: 0,
    pageSize: 10,
  })

  const advancedFilter = pendingOnly || configFilter !== 'all' || runtimeFilter !== 'all'
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
    queryKey: ['listServerPage', fetchDataOptions],
    queryFn: async () => {
      if (advancedFilter) {
        const result = await collectPages(async (page, pageSize) => {
          const response = await listServer({ page, pageSize, keyword: Keyword })
          return { total: response.total ?? 0, items: response.servers }
        })
        return { total: result.total, servers: result.items }
      }
      return await listServer({
        page: fetchDataOptions.pageIndex + 1,
        pageSize: fetchDataOptions.pageSize,
        keyword: fetchDataOptions.Keyword,
      })
    },
    placeholderData: keepPreviousData,
    refetchInterval: advancedFilter ? 30_000 : 10_000,
    staleTime: 10_000,
  })

  const allServers = dataQuery.data?.servers ?? Servers
  const serverIds = React.useMemo(() => allServers.map((server) => server.id || '').filter(Boolean), [allServers])
  const statusQuery = useQuery({
    queryKey: ['listServerStatuses', serverIds.join(','), globalRefetchTrigger],
    queryFn: async () => {
      if (serverIds.length === 0) {
        return undefined
      }
      return await getClientsStatus({ clientIds: serverIds, clientType: ClientType.FRPS })
    },
    enabled: serverIds.length > 0,
    staleTime: 5000,
    refetchInterval: 15000,
    refetchIntervalInBackground: false,
  })

  const rows = React.useMemo(() => {
    return allServers
      .map((server) => {
        const id = server.id || ''
        const status = statusQuery.data?.clients[id]
        const runtimeStatus: ServerTableSchema['runtimeStatus'] =
          status?.status === ClientStatus_Status.ONLINE
            ? 'online'
            : status?.status === ClientStatus_Status.ERROR
              ? 'error'
              : status?.status === ClientStatus_Status.OFFLINE
                ? 'offline'
                : 'unknown'
        const version = status?.version
        return {
          id,
          status: server.config == undefined || server.config == '' ? 'invalid' : 'valid',
          runtimeStatus,
          ping: status?.ping ?? Number.MAX_SAFE_INTEGER,
          info: `${runtimeStatus} ${status?.ping ?? ''} ${version?.gitVersion ?? ''} ${version?.platform ?? ''} ${version?.goVersion ?? ''}`,
          clientStatus: status,
          secret: server.secret == undefined ? '' : server.secret,
          ip: server.ip || '',
          config: server.config,
          stopped: false,
          frpsUrls: server.frpsUrls || [],
        } as ServerTableSchema
      })
      .filter((row) => !pendingOnly || pendingIDs.has(row.id))
      .filter((row) => configFilter === 'all' || row.status === configFilter)
      .filter((row) => runtimeFilter === 'all' || row.runtimeStatus === runtimeFilter)
  }, [allServers, statusQuery.data, pendingOnly, pendingIDs, configFilter, runtimeFilter])

  React.useEffect(() => {
    setPagination((current) => ({ ...current, pageIndex: 0 }))
  }, [Keyword, pendingOnly, configFilter, runtimeFilter])

  const table = useReactTable({
    data: rows,
    columns: serverColumnsDef,
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
      loading={dataQuery.isPending || (pendingOnly && pendingQuery.isPending)}
      error={dataQuery.error?.message || (pendingOnly ? pendingQuery.error?.message : undefined)}
      onRetry={() => {
        void dataQuery.refetch()
        if (pendingOnly) void pendingQuery.refetch()
        void statusQuery.refetch()
      }}
      emptyMessage={
        Keyword || advancedFilter
          ? '没有匹配的节点，试试清空搜索或筛选条件。'
          : '还没有节点。点击上方自动接入，复制命令到设备上运行。'
      }
      table={table}
      columns={serverColumnsDef}
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <Button variant={pendingOnly ? 'default' : 'outline'} onClick={() => { const query = { ...router.query }; if (pendingOnly) delete query.pending; else query.pending = '1'; void router.replace({ pathname: router.pathname, query }, undefined, { shallow: true }) }}>{pendingOnly ? '待处理 · 点击查看全部' : '仅看待处理'}</Button>
          <span className="text-xs text-muted-foreground">
            共 {dataQuery.data?.total ?? 0} 个节点 · {advancedFilter ? '全局筛选' : '按页加载 · 排序作用于当前页'}
          </span>
          <select
            aria-label="服务端筛选"
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
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setConfigFilter('all')
              setRuntimeFilter('all')
              setSorting([])
              if (pendingOnly) { const query = { ...router.query }; delete query.pending; void router.replace({ pathname: router.pathname, query }, undefined, { shallow: true }) }
            }}
          >
            重置
          </Button>
        </div>
      }
    />
  )
}
