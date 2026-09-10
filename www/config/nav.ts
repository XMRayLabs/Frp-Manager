import {
  LayoutDashboard,
  Server,
  MonitorSmartphone,
  Cable,
  ChartNetwork,
  ScrollText,
  SquareTerminal,
  Network,
  Settings,
  ShieldCheck,
  SquareFunction,
} from 'lucide-react'
export const getNavItems = (t: (key: string) => string, role?: string) => [
  { title: '概览', url: '/', icon: LayoutDashboard },
  { title: t('nav.clients'), url: '/clients', icon: MonitorSmartphone },
  { title: t('nav.servers'), url: '/servers', icon: Server },
  { title: '隧道管理', url: '/proxies', icon: Cable },
  { title: t('nav.trafficStats'), url: '/clientstats', icon: ChartNetwork },
  {
    title: '高级工具',
    url: '/streamlog',
    icon: Network,
    items: [
      { title: t('nav.realTimeLog'), url: '/streamlog' },
      { title: 'WireGuard 网络', url: '/wg/networks' },
      { title: 'WireGuard 接口', url: '/wg/wireguards' },
      { title: '网络端点', url: '/wg/endpoints' },
      { title: '网络链路', url: '/wg/links' },
      { title: t('nav.workers'), url: '/workers' },
      ...(role === 'admin'
        ? [
            { title: t('nav.console'), url: '/console' },
            { title: '可视化画布', url: '/canvas' },
          ]
        : []),
    ],
  },
  ...(['admin', 'group_admin'].includes(role ?? '') ? [{ title: '语系管理', url: '/organization', icon: ShieldCheck }] : []),
  ...(role === 'admin' ? [{ title: '权限管理', url: '/admin-permissions', icon: ShieldCheck }] : []),
  { title: '面板设置', url: '/platform-settings', icon: Settings },
]
