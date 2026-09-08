import { getAllProxyStats } from '@/api/stats'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ProxyInfo } from '@/lib/pb/common'
import { formatBytes } from '@/lib/utils'
import { useQuery } from '@tanstack/react-query'
import { ArrowDown, ArrowUp, RefreshCcw } from 'lucide-react'
import React from 'react'

type TrafficScope = 'client' | 'server'
type SortKey = 'todayTotal' | 'historyTotal' | 'todayIn' | 'todayOut' | 'name' | 'deviceID'
type SortDir = 'asc' | 'desc'

type TrafficRow = {
  scope: TrafficScope
  deviceID: string
  name: string
  type: string
  todayIn: number
  todayOut: number
  historyIn: number
  historyOut: number
  todayTotal: number
  historyTotal: number
}

type TrafficFilters = {
  keyword: string
  type: string
  activeOnly: boolean
  sortKey: SortKey
  sortDir: SortDir
  page: number
  pageSize: number
}

const defaultFilters: TrafficFilters = {
  keyword: '',
  type: 'all',
  activeOnly: false,
  sortKey: 'todayTotal',
  sortDir: 'desc',
  page: 0,
  pageSize: 10,
}

export function TrafficStatsOverview() {
  const [clientFilters, setClientFilters] = React.useState<TrafficFilters>(defaultFilters)
  const [serverFilters, setServerFilters] = React.useState<TrafficFilters>(defaultFilters)

  const trafficStatsQuery = useQuery({
    queryKey: ['allTrafficStats'],
    queryFn: getAllProxyStats,
    staleTime: 15000,
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
  })
  const clientRows = React.useMemo(
    () =>
      mergeTrafficRows(
        (trafficStatsQuery.data?.proxyInfos ?? []).map((info) =>
          toTrafficRow('client', info.originClientId || info.clientId || '-', info),
        ),
      ),
    [trafficStatsQuery.data],
  )
  const serverRows = React.useMemo(
    () =>
      mergeTrafficRows(
        (trafficStatsQuery.data?.proxyInfos ?? []).map((info) =>
          toTrafficRow('server', info.serverId || '-', info),
        ),
      ),
    [trafficStatsQuery.data],
  )

  return (
    <Tabs defaultValue="clients" className="w-full">
      <div className="flex flex-wrap items-center justify-between gap-3 py-4">
        <TabsList>
          <TabsTrigger value="clients">客户端统计</TabsTrigger>
          <TabsTrigger value="servers">服务端统计</TabsTrigger>
        </TabsList>
        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={() => trafficStatsQuery.refetch()}>
            <RefreshCcw className="mr-2 h-4 w-4" />
            刷新客户端
          </Button>
          <Button variant="outline" size="sm" onClick={() => trafficStatsQuery.refetch()}>
            <RefreshCcw className="mr-2 h-4 w-4" />
            刷新服务端
          </Button>
        </div>
      </div>
      <TabsContent value="clients">
        <TrafficStatsTable
          title="客户端流量"
          rows={clientRows}
          loading={trafficStatsQuery.isLoading}
          filters={clientFilters}
          onFiltersChange={setClientFilters}
        />
      </TabsContent>
      <TabsContent value="servers">
        <TrafficStatsTable
          title="服务端流量"
          rows={serverRows}
          loading={trafficStatsQuery.isLoading}
          filters={serverFilters}
          onFiltersChange={setServerFilters}
        />
      </TabsContent>
    </Tabs>
  )
}

function TrafficStatsTable({
  title,
  rows,
  loading,
  filters,
  onFiltersChange,
}: {
  title: string
  rows: TrafficRow[]
  loading: boolean
  filters: TrafficFilters
  onFiltersChange: (filters: TrafficFilters) => void
}) {
  const types = React.useMemo(() => Array.from(new Set(rows.map((row) => row.type).filter(Boolean))).sort(), [rows])
  const filteredRows = React.useMemo(() => {
    const keyword = filters.keyword.trim().toLowerCase()
    return rows
      .filter((row) => {
        if (!keyword) {
          return true
        }
        return [row.deviceID, row.name, row.type].some((value) => value.toLowerCase().includes(keyword))
      })
      .filter((row) => filters.type === 'all' || row.type === filters.type)
      .filter((row) => !filters.activeOnly || row.todayTotal > 0 || row.historyTotal > 0)
      .sort((a, b) => {
        const left = a[filters.sortKey]
        const right = b[filters.sortKey]
        const result = typeof left === 'number' && typeof right === 'number'
          ? left - right
          : String(left).localeCompare(String(right))
        return filters.sortDir === 'asc' ? result : -result
      })
  }, [rows, filters])

  const totalPages = Math.max(1, Math.ceil(filteredRows.length / filters.pageSize))
  const safePage = Math.min(filters.page, totalPages - 1)
  const pageRows = filteredRows.slice(safePage * filters.pageSize, (safePage + 1) * filters.pageSize)
  const summary = filteredRows.reduce(
    (acc, row) => ({
      today: acc.today + row.todayTotal,
      history: acc.history + row.historyTotal,
    }),
    { today: 0, history: 0 },
  )

  const updateFilters = (patch: Partial<TrafficFilters>) => onFiltersChange({ ...filters, ...patch })
  const resetFilters = () => onFiltersChange(defaultFilters)

  return (
    <Card>
      <CardHeader className="gap-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <CardTitle>{title}</CardTitle>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="secondary">今日 {formatBytes(summary.today)}</Badge>
            <Badge variant="secondary">累计 {formatBytes(summary.history)}</Badge>
            <Badge variant="secondary">{filteredRows.length} 条隧道</Badge>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Input
            className="h-9 w-56"
            value={filters.keyword}
            onChange={(e) => updateFilters({ keyword: e.target.value, page: 0 })}
            placeholder="搜索设备、隧道、协议"
          />
          <select
            className="h-9 rounded-md border bg-background px-3 text-sm"
            value={filters.type}
            onChange={(e) => updateFilters({ type: e.target.value, page: 0 })}
          >
            <option value="all">全部协议</option>
            {types.map((type) => (
              <option key={type} value={type}>{type}</option>
            ))}
          </select>
          <select
            className="h-9 rounded-md border bg-background px-3 text-sm"
            value={filters.sortKey}
            onChange={(e) => updateFilters({ sortKey: e.target.value as SortKey })}
          >
            <option value="todayTotal">今日总流量</option>
            <option value="historyTotal">累计总流量</option>
            <option value="todayIn">今日入站</option>
            <option value="todayOut">今日出站</option>
            <option value="deviceID">设备 ID</option>
            <option value="name">隧道名称</option>
          </select>
          <Button
            variant="outline"
            size="sm"
            onClick={() => updateFilters({ sortDir: filters.sortDir === 'desc' ? 'asc' : 'desc' })}
          >
            {filters.sortDir === 'desc' ? <ArrowDown className="mr-2 h-4 w-4" /> : <ArrowUp className="mr-2 h-4 w-4" />}
            {filters.sortDir === 'desc' ? '降序' : '升序'}
          </Button>
          <Button
            variant={filters.activeOnly ? 'default' : 'outline'}
            size="sm"
            onClick={() => updateFilters({ activeOnly: !filters.activeOnly, page: 0 })}
          >
            只看有流量
          </Button>
          <Button variant="ghost" size="sm" onClick={resetFilters}>重置</Button>
        </div>
      </CardHeader>
      <CardContent>
        <div className="rounded-md border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>设备</TableHead>
                <TableHead>隧道</TableHead>
                <TableHead>协议</TableHead>
                <TableHead className="text-right">今日入站</TableHead>
                <TableHead className="text-right">今日出站</TableHead>
                <TableHead className="text-right">今日总计</TableHead>
                <TableHead className="text-right">累计总计</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-24 text-center">正在加载流量统计</TableCell>
                </TableRow>
              ) : pageRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} className="h-24 text-center">暂无统计数据</TableCell>
                </TableRow>
              ) : (
                pageRows.map((row) => (
                  <TableRow key={`${row.scope}:${row.deviceID}:${row.name}:${row.type}`}>
                    <TableCell className="font-mono">{row.deviceID}</TableCell>
                    <TableCell className="font-mono">{row.name}</TableCell>
                    <TableCell>{row.type}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatBytes(row.todayIn)}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatBytes(row.todayOut)}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatBytes(row.todayTotal)}</TableCell>
                    <TableCell className="text-right tabular-nums">{formatBytes(row.historyTotal)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
        <div className="flex flex-wrap items-center justify-between gap-2 py-3 text-sm text-muted-foreground">
          <div>第 {safePage + 1} / {totalPages} 页，共 {filteredRows.length} 条</div>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={safePage === 0} onClick={() => updateFilters({ page: safePage - 1 })}>
              上一页
            </Button>
            <Button variant="outline" size="sm" disabled={safePage >= totalPages - 1} onClick={() => updateFilters({ page: safePage + 1 })}>
              下一页
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

function toTrafficRow(scope: TrafficScope, deviceID: string, info: ProxyInfo): TrafficRow {
  const todayIn = Number(info.todayTrafficIn ?? 0)
  const todayOut = Number(info.todayTrafficOut ?? 0)
  const historyIn = Number(info.historyTrafficIn ?? 0)
  const historyOut = Number(info.historyTrafficOut ?? 0)
  return {
    scope,
    deviceID,
    name: info.name || '',
    type: info.type || '',
    todayIn,
    todayOut,
    historyIn,
    historyOut,
    todayTotal: todayIn + todayOut,
    historyTotal: historyIn + historyOut,
  }
}

function mergeTrafficRows(rows: TrafficRow[]): TrafficRow[] {
  const merged = new Map<string, TrafficRow>()
  for (const row of rows) {
    const key = `${row.scope}:${row.deviceID}:${row.name}:${row.type}`
    const existing = merged.get(key)
    if (!existing) {
      merged.set(key, { ...row })
      continue
    }
    existing.todayIn += row.todayIn
    existing.todayOut += row.todayOut
    existing.historyIn += row.historyIn
    existing.historyOut += row.historyOut
    existing.todayTotal = existing.todayIn + existing.todayOut
    existing.historyTotal = existing.historyIn + existing.historyOut
  }
  return Array.from(merged.values())
}
