import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { getNodeOverview, type NodeOverview } from '@/api/platform'
import { Server, MonitorSmartphone, CircleDashed, ArrowUpRight } from 'lucide-react'

function pendingDetail(node?: NodeOverview) {
  if (!node) return '未配置、配置不完整、运行异常或需要升级'
  if (!node.pending) return '暂无待处理节点，运行检测在后台刷新'
  return [
    node.unconfigured && `未完成配置 ${node.unconfigured}`,
    node.invalid && `配置有误 ${node.invalid}`,
    node.unavailable && `运行异常 ${node.unavailable}`,
    node.upgrade && `需要升级 ${node.upgrade}`,
  ].filter(Boolean).join(' · ')
}

export default function PlatformInfo() {
  const query = useQuery({ queryKey: ['nodeOverview'], queryFn: getNodeOverview, refetchInterval: 15_000 })
  const info = query.data
  const ratio = (node?: NodeOverview) => node ? `${node.online}/${node.total}` : undefined
  const metrics = [
    { title: '在线客户端', value: ratio(info?.clients), icon: MonitorSmartphone, href: '/clients', detail: '在线 / 总量' },
    { title: '在线服务端', value: ratio(info?.servers), icon: Server, href: '/servers', detail: '在线 / 总量' },
    { title: '待处理客户端', value: info?.clients.pending, icon: CircleDashed, href: '/clients', detail: pendingDetail(info?.clients) },
    { title: '待处理服务端', value: info?.servers.pending, icon: CircleDashed, href: '/servers', detail: pendingDetail(info?.servers) },
  ]
  return (
    <section aria-label="节点概览">
      {query.isError && (
        <p role="alert" className="mb-3 text-sm text-destructive">
          概览暂时无法更新。
          <button className="ml-2 underline" onClick={() => query.refetch()}>重试</button>
        </p>
      )}
      <div className="grid grid-cols-2 gap-4 xl:grid-cols-4">
        {metrics.map((item) => (
          <Link key={item.title} href={item.href} className="metric-card rounded-xl bg-card p-5">
            <div className="flex items-center justify-between text-muted-foreground">
              <item.icon className="size-5" />
              <ArrowUpRight className="size-4" />
            </div>
            <div className="mt-5 break-all text-3xl font-semibold tabular-nums">{item.value ?? '—'}</div>
            <div className="mt-2 text-sm font-medium">{item.title}</div>
            <p className="mt-1 text-xs text-muted-foreground">{item.detail}</p>
          </Link>
        ))}
      </div>
      <p className="mt-2 text-xs text-muted-foreground">同一节点存在多个问题时，待处理总数只计一次。运行状态自动检测，升级以当前面板版本为准。</p>
    </section>
  )
}
