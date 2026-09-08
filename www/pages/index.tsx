import Link from 'next/link'
import { ArrowUpRight, Cable, MonitorSmartphone, Server, ArrowRight } from 'lucide-react'
import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { PageHeading } from '@/components/page-heading'
import PlatformInfo from '@/components/platforminfo'
const steps = [
  { icon: Server, title: '接入服务端', text: '选择一台公网服务器，作为设备的访问入口。', href: '/servers' },
  {
    icon: MonitorSmartphone,
    title: '接入客户端',
    text: '复制接入命令，在需要访问的内网设备上运行。',
    href: '/clients',
  },
  { icon: Cable, title: '创建隧道', text: '选择两端节点，将本地服务映射到公网。', href: '/proxies' },
]
export default function Home() {
  return (
    <Providers>
      <RootLayout mainHeader={<Header title="概览" />}>
        <div className="mx-auto max-w-[1440px] space-y-8">
          <PageHeading
            title="连接，从这里开始"
            description="集中管理设备和隧道，快速找到需要处理的配置。"
            actions={
              <Link className="quick-action" href="/proxies">
                管理隧道 <ArrowUpRight className="size-4" />
              </Link>
            }
          />
          <PlatformInfo />
          <section className="rounded-xl border bg-card p-6">
            <div className="mb-6 flex items-center justify-between">
              <div>
                <h2 className="font-semibold">三步建立连接</h2>
                <p className="mt-1 text-sm text-muted-foreground">第一次使用？按顺序完成接入即可。</p>
              </div>
              <span className="rounded-full bg-primary/10 px-3 py-1 text-xs text-primary">快速开始</span>
            </div>
            <div className="grid gap-4 lg:grid-cols-3">
              {steps.map((step, i) => (
                <Link
                  key={step.href}
                  href={step.href}
                  className="group rounded-lg border p-5 transition-colors hover:border-primary hover:bg-primary/5"
                >
                  <div className="mb-5 flex items-center justify-between">
                    <step.icon className="size-6 text-primary" />
                    <span className="font-mono text-xs text-muted-foreground">0{i + 1}</span>
                  </div>
                  <h3 className="flex items-center justify-between font-semibold">
                    {step.title}
                    <ArrowRight className="size-4 transition-transform group-hover:translate-x-1" />
                  </h3>
                  <p className="mt-2 text-sm leading-relaxed text-muted-foreground">{step.text}</p>
                </Link>
              ))}
            </div>
          </section>
          <div className="grid gap-4 md:grid-cols-2">
            <Link href="/clientstats" className="rounded-xl border bg-card p-5">
              <h3 className="font-medium">查看流量</h3>
              <p className="mt-1 text-sm text-muted-foreground">按设备查看用量和历史趋势。</p>
            </Link>
            <Link href="/streamlog" className="rounded-xl border bg-card p-5">
              <h3 className="font-medium">排查连接问题</h3>
              <p className="mt-1 text-sm text-muted-foreground">查看节点实时日志，定位配置和连接异常。</p>
            </Link>
          </div>
        </div>
      </RootLayout>
    </Providers>
  )
}
