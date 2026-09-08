import { Providers } from '@/components/providers'
import { RootLayout } from '@/components/layout'
import { Header } from '@/components/header'
import { TrafficStatsOverview } from '@/components/stats/traffic_stats_overview'

export default function ClientStatsPage() {
  return (
    <Providers>
      <RootLayout mainHeader={<Header />}>
        <div className="w-full">
          <div className="flex-1 flex-col">
            <TrafficStatsOverview />
          </div>
        </div>
      </RootLayout>
    </Providers>
  )
}
