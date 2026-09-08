import { Toaster } from '@/components/ui/sonner'
import { AppSidebar } from '@/components/app-sidebar'
import { Separator } from '@/components/ui/separator'
import { SidebarInset, SidebarTrigger, useSidebar } from '@/components/ui/sidebar'
import { cn } from '@/lib/utils'

export function RootLayout({
  children,
  sidebarItems,
  sidebarFooter,
  mainHeader,
}: {
  children: React.ReactNode
  sidebarItems?: React.ReactNode
  sidebarFooter?: React.ReactNode
  mainHeader?: React.ReactNode
}) {
  const { open, isMobile } = useSidebar()

  return (
    <>
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:z-50 focus:bg-background focus:p-4"
      >
        跳到主要内容
      </a>
      <AppSidebar footer={sidebarFooter}>{sidebarItems}</AppSidebar>
      <SidebarInset className="min-w-0">
        <div className="relative flex min-w-0 w-full flex-1 flex-col overflow-hidden bg-background">
          <header className="z-20 flex h-14 w-full shrink-0 flex-row items-center gap-2 border-b bg-background/95 pr-4 backdrop-blur">
            <div className="flex flex-row items-center gap-2 px-4">
              <SidebarTrigger className="-ml-1" />
              <Separator orientation="vertical" className="mr-2 h-4" />
            </div>
            {mainHeader}
          </header>
          <div
            id="main-content"
            tabIndex={-1}
            className="h-[calc(100dvh_-_56px)] w-full overflow-auto px-4 pb-8 pt-6 md:px-8"
          >
            {children}
          </div>
        </div>
      </SidebarInset>
    </>
  )
}
