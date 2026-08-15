import type { TFunction } from "i18next"
import {
  CircleIcon,
  InboxIcon,
  LogOutIcon,
  PanelsTopLeftIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
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
  { id: "inbox", kind: "inbox" as const, icon: InboxIcon },
  ...Array.from({ length: 5 }, (_, index) => ({
    id: `menu-${index + 1}`,
    kind: "placeholder" as const,
    number: index + 1,
    icon: CircleIcon,
  })),
]

type MenuItem = (typeof menuItems)[number]

function getMenuLabel(item: MenuItem, t: TFunction<"workspace">) {
  return item.kind === "inbox"
    ? t("inbox")
    : t("menu", { number: item.number })
}

function WorkspaceSidebar({
  activeMenuId,
  onLogout,
}: {
  activeMenuId: string
  onLogout: () => void
}) {
  const navigate = useNavigate()
  const { t } = useTranslation("workspace")
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
                <span className="truncate text-xs">{t("brandSubtitle")}</span>
              </div>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel>{t("navigationGroup")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {menuItems.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={activeMenuId === item.id}
                    tooltip={getMenuLabel(item, t)}
                    onClick={() => selectMenu(item.id)}
                  >
                    <item.icon />
                    <span>{getMenuLabel(item, t)}</span>
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
            <SidebarMenuButton tooltip={t("logout")} onClick={onLogout}>
              <LogOutIcon />
              <span>{t("logout")}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
    </Sidebar>
  )
}

export function WorkspacePage({ onLogout }: { onLogout: () => void }) {
  const { t } = useTranslation("workspace")
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
          <h1 className="text-sm font-medium">
            {getMenuLabel(activeMenu, t)}
          </h1>
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
                  {getMenuLabel(activeMenu, t)}
                </h2>
                <p className="mt-2 text-sm text-muted-foreground">
                  {t("placeholderDescription")}
                </p>
              </div>
            </section>
          </div>
        )}
      </SidebarInset>
    </SidebarProvider>
  )
}
