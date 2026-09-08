import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { getPlatformInfo } from '@/api/platform'
import { Server, MonitorSmartphone, CircleDashed, ArrowUpRight } from 'lucide-react'
export default function PlatformInfo() {
  const query = useQuery({ queryKey: ['platformInfo'], queryFn: getPlatformInfo, refetchInterval: 30_000 })
  const info = query.data
  const metrics = [
    { title: '服务端', value: info?.totalServerCount, icon: Server, href: '/servers', detail: '公网连接入口' },
    {
      title: '客户端',
      value: info?.totalClientCount,
      icon: MonitorSmartphone,
      href: '/clients',
      detail: '已接入的设备',
    },
    {
      title: '待配置服务端',
      value: info?.unconfiguredServerCount,
      icon: CircleDashed,
      href: '/servers',
      detail: '接入后尚未配置',
    },
    {
      title: '待配置客户端',
      value: info?.unconfiguredClientCount,
      icon: CircleDashed,
      href: '/clients',
      detail: '下一步：配置隧道',
    },
  ]
  return (
    <section aria-label="节点概览">
      {query.isError && (
        <p role="alert" className="mb-3 text-sm text-destructive">
          概览暂时无法更新。
          <button className="ml-2 underline" onClick={() => query.refetch()}>
            重试
          </button>
        </p>
      )}
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {metrics.map((item) => (
          <Link key={item.title} href={item.href} className="metric-card rounded-xl bg-card p-5">
            <div className="flex items-center justify-between text-muted-foreground">
              <item.icon className="size-5" />
              <ArrowUpRight className="size-4" />
            </div>
            <div className="mt-5 text-3xl font-semibold tabular-nums">{item.value ?? '—'}</div>
            <div className="mt-2 text-sm font-medium">{item.title}</div>
            <p className="mt-1 text-xs text-muted-foreground">{item.detail}</p>
          </Link>
        ))}
      </div>
    </section>
  )
}
