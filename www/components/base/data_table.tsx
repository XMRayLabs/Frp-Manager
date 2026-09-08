'use client'

import {
  ColumnDef,
  flexRender,
  getSortedRowModel,
  getCoreRowModel,
  ColumnFiltersState,
  useReactTable,
  getFilteredRowModel,
  getPaginationRowModel,
  SortingState,
  Table as TableType,
} from '@tanstack/react-table'

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'

import React from 'react'
import { Input } from '@/components/ui/input'
import { DataTablePagination } from './data_table_pagination'
import { useTranslation } from 'react-i18next'

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[]
  data?: TData[]
  filterColumnName?: string
  table: TableType<TData>
  loading?: boolean
  error?: string
  onRetry?: () => void
  emptyMessage?: string
  toolbar?: React.ReactNode
}

export function DataTable<TData, TValue>({
  columns,
  filterColumnName,
  table,
  toolbar,
  loading,
  error,
  onRetry,
  emptyMessage,
}: DataTableProps<TData, TValue>) {
  const { t } = useTranslation()

  return (
    <div className="space-y-4" aria-busy={loading}>
      {error && (
        <div
          role="alert"
          className="flex items-center justify-between rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm"
        >
          <span>加载失败：{error}</span>
          <button onClick={onRetry} className="underline">
            重试
          </button>
        </div>
      )}
      {(toolbar || filterColumnName) && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          {filterColumnName ? (
            <Input
              placeholder={t('table.filter.placeholder', { column: filterColumnName })}
              value={(table.getColumn(filterColumnName)?.getFilterValue() as string) ?? ''}
              onChange={(event) => table.getColumn(filterColumnName)?.setFilterValue(event.target.value)}
              className="max-w-sm"
            />
          ) : null}
          {toolbar && <div className="flex min-w-0 flex-wrap items-center gap-2">{toolbar}</div>}
        </div>
      )}
      <div className="overflow-hidden rounded-xl border bg-card">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id} className="text-nowrap">
                {headerGroup.headers.map((header) => {
                  return (
                    <TableHead key={header.id}>
                      {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
                    </TableHead>
                  )
                })}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id} data-state={row.getIsSelected() && 'selected'}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>{flexRender(cell.column.columnDef.cell, cell.getContext())}</TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-40 px-6 text-center text-muted-foreground">
                  {loading ? '正在加载节点…' : emptyMessage || t('table.noData')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className="my-2">
        <DataTablePagination table={table} />
      </div>
    </div>
  )
}
