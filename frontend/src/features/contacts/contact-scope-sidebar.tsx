/** 通讯录二级导航：分类树和来源渠道筛选。 */
import {
  forwardRef,
  useMemo,
  type ButtonHTMLAttributes,
} from "react"
import {
  ChevronRightIcon,
  ContactRoundIcon,
  PanelsTopLeftIcon,
  PlusIcon,
  UsersIcon,
  type LucideIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { ChannelType, type ChannelOption, type Team } from "@/api"
import { messageChannelTypeDefinition } from "@/features/channels/message-channel-types"
import { PagePaneNav } from "@/components/page-split"
import { Button } from "@/components/ui/button"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
        nested ? "pl-14" : "pl-8",
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
  channels,
  teamId,
  teams,
}: {
  scope: ContactScope
  deleted: boolean
  channelId: string
  channels: ChannelOption[]
  teamId: string
  teams: Team[]
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
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
              className="bg-primary text-primary-foreground hover:bg-primary/90 hover:text-primary-foreground"
              aria-label={t("add.label")}
              title={t("add.label")}
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
                    ? `/contacts/teams/${encodeURIComponent(teamId)}?newAgent=1`
                    : "/contacts/ai-employees?newAgent=1",
                )
              }
            >
              {t("add.agent")}
            </DropdownMenuItem>
            {/* TODO: 等手工联系人身份和可发送渠道关系明确后再恢复添加外部联系人入口。 */}
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
      <Collapsible defaultOpen>
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className={cn(
              "group flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors",
              contactNavHoverClass,
              scope !== "external" && contactNavPathActiveClass,
            )}
          >
            <UsersIcon className="size-4" />
            <span>{t("scopes.members")}</span>
            <ChevronRightIcon className="ml-auto size-4 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent className="flex flex-col gap-0.5">
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
          <Collapsible defaultOpen>
            <CollapsibleTrigger asChild>
              <button
                type="button"
                className={cn(
                  "group pl-8",
                  contactNavSubitemClass,
                  contactNavHoverClass,
                  scope === "team" && contactNavPathActiveClass,
                )}
              >
                <span>{t("scopes.teams")}</span>
                <ChevronRightIcon className="ml-auto size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
              </button>
            </CollapsibleTrigger>
            <CollapsibleContent className="flex flex-col gap-0.5">
              {teams.map((team) => (
                <SubscopeButton
                  key={team.id}
                  active={scope === "team" && teamId === team.id}
                  icon={PanelsTopLeftIcon}
                  nested
                  onClick={() => navigate(`/contacts/teams/${team.id}`)}
                >
                  {team.name}
                </SubscopeButton>
              ))}
            </CollapsibleContent>
          </Collapsible>
        </CollapsibleContent>
      </Collapsible>

      <Collapsible defaultOpen>
        <CollapsibleTrigger asChild>
          <button
            type="button"
            className={cn(
              "group flex h-9 w-full items-center gap-2 rounded-md px-2.5 text-left text-sm transition-colors",
              contactNavHoverClass,
              scope === "external" && contactNavPathActiveClass,
            )}
          >
            <ContactRoundIcon className="size-4" />
            <span>{t("scopes.external")}</span>
            <ChevronRightIcon className="ml-auto size-4 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent className="flex flex-col gap-0.5">
          <SubscopeButton
            active={scope === "external" && !deleted && !channelId}
            onClick={() => navigate("/contacts/external")}
          >
            {t("all")}
          </SubscopeButton>
          {groupedChannels.map(([type, items]) => (
            <Collapsible key={type} defaultOpen>
              <CollapsibleTrigger asChild>
                <button
                  type="button"
                  className={cn(
                    "group pl-8",
                    contactNavSubitemClass,
                    contactNavHoverClass,
                    scope === "external" &&
                      items.some((channel) => channel.id === channelId) &&
                      contactNavPathActiveClass,
                  )}
                >
                  <span>{channelTypeLabel(type, t)}</span>
                  <ChevronRightIcon className="ml-auto size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-90" />
                </button>
              </CollapsibleTrigger>
              <CollapsibleContent className="flex flex-col gap-0.5">
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
              </CollapsibleContent>
            </Collapsible>
          ))}
        </CollapsibleContent>
      </Collapsible>
    </PagePaneNav>
  )
}
