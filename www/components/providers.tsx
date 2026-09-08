import React, { createContext, useContext } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SidebarProvider } from '@/components/ui/sidebar'
import { Toaster } from './ui/sonner'

const Provided = createContext(false)
export const Providers = ({ children }: { children: React.ReactNode }) => {
  const inherited = useContext(Provided)
  if (inherited) return <>{children}</>
  return <RootProviders>{children}</RootProviders>
}
function RootProviders({ children }: { children: React.ReactNode }) {
  const [client] = React.useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 30_000,
            gcTime: 5 * 60_000,
            retry: 1,
            refetchOnWindowFocus: true,
            refetchIntervalInBackground: false,
          },
        },
      }),
  )
  return (
    <Provided.Provider value={true}>
      <QueryClientProvider client={client}>
        <SidebarProvider>
          {children}
          <Toaster />
        </SidebarProvider>
      </QueryClientProvider>
    </Provided.Provider>
  )
}
