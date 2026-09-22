/** 通讯录二级导航：分类树和来源渠道筛选。 */
import { forwardRef, useMemo, type ButtonHTMLAttributes } from "react"
import {
  ContactRoundIcon,
  PanelsTopLeftIcon,
  PlusIcon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"

import { ChannelType, type ChannelOption, type Team } from "@/api"
import { messageChannelTypeDefinition } from "@/lib/message-channel-types"
import { PagePaneNav } from "@/components/page-split"
import { RowActionsMenu } from "@/components/row-actions-menu"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { agentReturnPath } from "@/features/contacts/agents/agent-navigation"
import { channelTypeLabel } from "@/features/contacts/external/contact-labels"
import type { ContactScope } from "@/features/contacts/contact-scope"
import { cn } from "@/lib/utils"

const contactNavHoverClass =
  "hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
const contactNavLeafActiveClass =
  "bg-sidebar-accent font-medium text-sidebar-accent-foreground"
const contactNavPathActiveClass = "font-medium text-sidebar-accent-foreground"
const contactNavSubitemClass =
  "flex h-8 w-full items-center gap-2 rounded-md py-1.5 pr-2 text-left text-sm text-muted-foreground transition-colors"

/** 通讯录子分类按钮。 */
const SubscopeButton = forwardRef<
  HTMLButtonElement,
  {
    active: boolean
    nested?: boolean
    icon?: LucideIcon
  } & ButtonHTMLAttributes<HTMLButtonElement>
>(function SubscopeButton(
  { active, children, nested = false, icon: Icon, className, ...props },
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      className={cn(
        contactNavSubitemClass,
        contactNavHoverClass,
        // 三级项放在带竖线的容器内，由容器负责缩进。
        nested ? "pl-2" : "pl-8",
        active && contactNavLeafActiveClass,
        className,
      )}
      {...props}
    >
      {Icon ? <Icon className="size-3.5 shrink-0" /> : null}
      <span className="truncate">{children}</span>
    </button>
  )
})

/** 通讯录分类和来源渠道筛选。 */
export function ContactScopeSidebar({
  scope,
  deleted,
  channelId,
  channelType,
  channels,
  teamId,
  teams,
}: {
  scope: ContactScope
  deleted: boolean
  channelId: string
  channelType: string
  channels: ChannelOption[]
  teamId: string
  teams: Team[]
}) {
  const { t } = useTranslation(["contacts", "common"])
  const navigate = useNavigate()
  const location = useLocation()
  const groupedChannels = useMemo(() => {
    const groups = new Map<ChannelType, ChannelOption[]>()
    for (const channel of channels) {
      groups.set(channel.type, [...(groups.get(channel.type) ?? []), channel])
    }
    return [...groups.entries()]
  }, [channels])

  return (
    <PagePaneNav
      label={t("scopeNavigation")}
      title={t("title")}
      action={
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-sm"
              className="shrink-0 text-muted-foreground"
              aria-label={t("common:actions.add")}
              title={t("common:actions.add")}
            >
              <PlusIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="right" align="start">
            <DropdownMenuItem
              onSelect={() =>
                navigate(
                  scope === "team" && teamId
                    ? `/contacts/teams/${encodeURIComponent(teamId)}?new=1`
                    : "/contacts/employees?new=1",
                )
              }
            >
              {t("add.member")}
            </DropdownMenuItem>
            <DropdownMenuItem
              onSelect={() =>
                navigate(
                  scope === "team" && teamId
                    ? `/contacts/ai-employees/new?teamId=${encodeURIComponent(teamId)}&returnTo=${encodeURIComponent(agentReturnPath(location.pathname, location.search))}`
                    : `/contacts/ai-employees/new?returnTo=${encodeURIComponent(agentReturnPath(location.pathname, location.search))}`,
                )
              }
            >
              {t("add.agent")}
            </DropdownMenuItem>
            {/* TODO: 添加外部联系人入口，校验联系人身份和可发送渠道。 */}
            {/*
            <DropdownMenuItem
              disabled={channels.length === 0}
              onSelect={() => navigate("/contacts/external?new=1")}
            >
              {t("add.external")}
            </DropdownMenuItem>
            */}
            <DropdownMenuItem
              onSelect={() => navigate("/contacts/employees?newTeam=1")}
            >
              {t("teams.create")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      }
    >
      {/* 分组标题只标示层级，子项始终展开。 */}
      <div
        className={cn(
          "flex h-8 w-full items-center gap-2 px-2.5 text-sm",
          scope !== "external" && contactNavPathActiveClass,
        )}
      >
        <UsersIcon className="size-4" />
        <span>{t("scopes.members")}</span>
      </div>
      <div className="flex flex-col gap-0.5">
        <SubscopeButton
          active={scope === "employees"}
          onClick={() => navigate("/contacts/employees")}
        >
          {t("scopes.employees")}
        </SubscopeButton>
        <SubscopeButton
          active={scope === "agents"}
          onClick={() => navigate("/contacts/ai-employees")}
        >
          {t("scopes.agents")}
        </SubscopeButton>
        {/* 选中「团队」展示所有团队的成员；进入具体团队时保留路径高亮。 */}
        <SubscopeButton
          active={scope === "team" && !teamId}
          className={cn(scope === "team" && teamId && contactNavPathActiveClass)}
          onClick={() => navigate("/contacts/teams")}
        >
          {t("scopes.teams")}
        </SubscopeButton>
        {/* 三级项左侧以竖线标示层级。 */}
        <div className="ml-9 flex flex-col gap-0.5 border-l pl-2">
          {teams.map((team) => (
            // 编辑和删除团队进入团队页并打开对应弹窗。
            <RowActionsMenu
              key={team.id}
              buttonSize="icon-xs"
              // 「⋯」浮在行右端，名称右侧留出按钮宽度后截断。
              buttonClassName="absolute top-1/2 right-1 -translate-y-1/2 bg-sidebar-accent"
              actions={[
                {
                  key: "edit",
                  label: t("common:actions.edit"),
                  onSelect: () => navigate(`/contacts/teams/${team.id}?editTeam=1`),
                },
                {
                  key: "delete",
                  label: t("common:actions.delete"),
                  destructive: true,
                  separatorBefore: true,
                  onSelect: () => navigate(`/contacts/teams/${team.id}?deleteTeam=1`),
                },
              ]}
            >
              {({ moreButton, menuOpen }) => (
                <div className="group/row relative">
                  <SubscopeButton
                    active={scope === "team" && teamId === team.id}
                    className={cn("pr-8", menuOpen && "bg-sidebar-accent")}
                    icon={PanelsTopLeftIcon}
                    nested
                    onClick={() => navigate(`/contacts/teams/${team.id}`)}
                  >
                    {team.name}
                  </SubscopeButton>
                  {moreButton}
                </div>
              )}
            </RowActionsMenu>
          ))}
        </div>
      </div>

      <div
        className={cn(
          "flex h-8 w-full items-center gap-2 px-2.5 text-sm",
          scope === "external" && contactNavPathActiveClass,
        )}
      >
        <ContactRoundIcon className="size-4" />
        <span>{t("scopes.external")}</span>
      </div>
      <div className="flex flex-col gap-0.5">
        <SubscopeButton
          active={scope === "external" && !deleted && !channelId && !channelType}
          onClick={() => navigate("/contacts/external")}
        >
          {t("all")}
        </SubscopeButton>
        {groupedChannels.map(([type, items]) => (
          <div key={type} className="flex flex-col gap-0.5">
            {/* 选中渠道类别展示该类别下所有渠道的联系人。 */}
            <SubscopeButton
              active={
                scope === "external" && !deleted && !channelId && channelType === type
              }
              className={cn(
                scope === "external" &&
                  items.some((channel) => channel.id === channelId) &&
                  contactNavPathActiveClass,
              )}
              onClick={() => navigate(`/contacts/external?channelType=${type}`)}
            >
              {channelTypeLabel(type, t)}
            </SubscopeButton>
            <div className="ml-9 flex flex-col gap-0.5 border-l pl-2">
              {items.map((channel) => (
                <SubscopeButton
                  key={channel.id}
                  active={scope === "external" && channelId === channel.id}
                  icon={messageChannelTypeDefinition(type)?.icon}
                  nested
                  onClick={() =>
                    navigate(`/contacts/external?channelId=${channel.id}`)
                  }
                >
                  {channel.name}
                </SubscopeButton>
              ))}
            </div>
          </div>
        ))}
      </div>
    </PagePaneNav>
  )
}
