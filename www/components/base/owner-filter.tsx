import type { ResourceOwners } from '@/api/resource-owners'

export function OwnerFilter({ value, onChange, data, loading, error, retry }: {
  value: string
  onChange: (value: string) => void
  data?: ResourceOwners
  loading: boolean
  error?: Error | null
  retry: () => void
}) {
  return (
    <>
      <select aria-label="所属用户" className="h-9 rounded-md border bg-background px-3 text-sm" value={value} onChange={(event) => onChange(event.target.value)} disabled={loading || !!error}>
        <option value="all">{loading ? '加载所属用户…' : '全部所属用户'}</option>
        {data?.owners.map((owner) => <option key={owner.id} value={String(owner.id)}>{owner.name}</option>)}
      </select>
      {error && <button className="text-xs text-destructive underline" onClick={retry}>所属用户加载失败，重试</button>}
    </>
  )
}
