import { useEffect } from 'react'
import { useSidebar } from '@/components/ui/sidebar'

import { ChevronRight, type LucideIcon } from 'lucide-react'
import { useRouter } from 'next/router'

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from '@/components/ui/sidebar'

export function NavMain({
  items,
}: {
  items: {
    title: string
    url: string
    icon?: LucideIcon
    isActive?: boolean
    items?: {
      title: string
      url: string
    }[]
  }[]
}) {
  const router = useRouter()
  const { setOpenMobile } = useSidebar()
  useEffect(() => {
    setOpenMobile(false)
  }, [router.pathname, setOpenMobile])
  const urlSelected = (url: string) => router.pathname === url
  return (
    <SidebarMenu className="gap-1">
      {items.map((item) => (
        <Collapsible
          key={item.url + router.pathname}
          defaultOpen={item.items?.some((subItem) => urlSelected(subItem.url))}
          className="group/collapsible"
        >
          <>
            {!item.items && (
              <SidebarMenuItem>
                <SidebarMenuButton
                  className="h-9"
                  isActive={urlSelected(item.url)}
                  onClick={() => router.push(item.url)}
                  tooltip={item.title}
                >
                  {item.icon && <item.icon />}
                  <span>{item.title}</span>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )}
            {item.items && (
              <SidebarMenuItem>
                <CollapsibleTrigger asChild>
                  <SidebarMenuButton
                    className="h-9"
                    isActive={item.items.some((subItem) => urlSelected(subItem.url))}
                    tooltip={item.title}
                  >
                    {item.icon && <item.icon />}
                    <span>{item.title}</span>
                    <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/collapsible:rotate-90" />
                  </SidebarMenuButton>
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <SidebarMenuSub>
                    {item.items?.map((subItem) => (
                      <SidebarMenuSubItem key={subItem.title}>
                        <SidebarMenuSubButton asChild isActive={urlSelected(subItem.url)}>
                          <button type="button" onClick={() => router.push(subItem.url)}>
                            <span>{subItem.title}</span>
                          </button>
                        </SidebarMenuSubButton>
                      </SidebarMenuSubItem>
                    ))}
                  </SidebarMenuSub>
                </CollapsibleContent>
              </SidebarMenuItem>
            )}
          </>
        </Collapsible>
      ))}
    </SidebarMenu>
  )
}
