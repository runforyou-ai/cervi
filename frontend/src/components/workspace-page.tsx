import {
  CircleIcon,
  InboxIcon,
  LogOutIcon,
  PanelsTopLeftIcon,
} from "lucide-react"
import { Navigate, useNavigate, useParams } from "react-router"

import { InboxPage } from "@/components/inbox/inbox-page"
import { Separator } from "@/components/ui/separator"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@/components/ui/sidebar"

const menuItems = [
  { id: "inbox", label: "inbox", icon: InboxIcon },
  ...Array.from({ length: 5 }, (_, index) => ({
    id: `menu-${index + 1}`,
    label: `菜单 ${index + 1}`,
    icon: CircleIcon,
  })),
]

function WorkspaceSidebar({
  activeMenuId,
  onLogout,
}: {
  activeMenuId: string
  onLogout: () => void
}) {
  const navigate = useNavigate()
  const { isMobile, setOpenMobile } = useSidebar()

  function selectMenu(menuId: string) {
    navigate(`/workspace/${menuId}`)

    if (isMobile) {
      setOpenMobile(false)
    }
  }

  return (
    <Sidebar variant="inset" collapsible="icon">
      <SidebarHeader>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton size="lg" tooltip="Cervi">
              <div className="flex aspect-square size-8 items-center justify-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground">
                <PanelsTopLeftIcon className="size-4" />
              </div>
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-semibold">Cervi</span>
                <span className="truncate text-xs">工作台</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>菜单</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {menuItems.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={activeMenuId === item.id}
                    tooltip={item.label}
                    onClick={() => selectMenu(item.id)}
                  >
                    <item.icon />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
      <SidebarFooter>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton tooltip="退出登录" onClick={onLogout}>
              <LogOutIcon />
              <span>退出登录</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}

export function WorkspacePage({ onLogout }: { onLogout: () => void }) {
  const { menuId } = useParams()
  const activeMenu = menuItems.find((item) => item.id === menuId)

  if (!activeMenu) {
    return <Navigate to="/workspace/inbox" replace />
  }

  return (
    <SidebarProvider>
      <WorkspaceSidebar activeMenuId={activeMenu.id} onLogout={onLogout} />
      <SidebarInset className="h-svh min-h-0 min-w-0 overflow-hidden md:h-[calc(100svh-1rem)]">
        <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
          <SidebarTrigger className="-ml-1" />
          <Separator orientation="vertical" className="mr-2 h-4" />
          <h1 className="text-sm font-medium">{activeMenu.label}</h1>
        </header>
        {activeMenu.id === "inbox" ? (
          <InboxPage />
        ) : (
          <div className="flex min-h-0 flex-1 p-4">
            <section className="flex flex-1 items-center justify-center rounded-xl border border-dashed bg-muted/20 p-6">
              <div className="max-w-sm text-center">
                <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border bg-background shadow-sm">
                  <PanelsTopLeftIcon className="size-5 text-muted-foreground" />
                </div>
                <h2 className="text-lg font-semibold tracking-tight">
                  {activeMenu.label}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  工作台基础架构已经就绪，业务内容将在后续步骤中补充。
                </p>
              </div>
            </section>
          </div>
        )}
      </SidebarInset>
    </SidebarProvider>
  )
}
