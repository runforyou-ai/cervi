/** 渠道列表页，按中间栏选中的渠道类别展示该类别下的渠道。 */
import { useEffect, useMemo } from "react"
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams, useSearchParams } from "react-router"

import {
  activateMessageChannel,
  deactivateMessageChannel,
  listMessageChannels,
  type MessageChannelSummary,
} from "@/api"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
} from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { Button } from "@/components/ui/button"
import {
  isMessageChannelType,
  messageChannelTypeDefinition,
} from "@/lib/message-channel-types"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

type ChannelEnabledStatus = "enabled" | "disabled"

/** 加载并管理当前类别的渠道列表。 */
export function MessageChannelListPage() {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const { channelType = "" } = useParams()
  const typeDefinition = isMessageChannelType(channelType)
    ? messageChannelTypeDefinition(channelType)
    : undefined
  const [searchParams, setSearchParams] = useSearchParams()
  // 状态筛选记在 URL 中，从编辑页返回时保留。
  const enabledStatus: ChannelEnabledStatus =
    searchParams.get("status") === "disabled" ? "disabled" : "enabled"
  const setEnabledStatus = (value: ChannelEnabledStatus) =>
    setSearchParams(value === "enabled" ? {} : { status: value }, {
      replace: true,
    })
  const {
    data,
    loading,
    retrying,
    error,
    refresh,
  } = useResource(resourceKeys.messageChannels(), () => listMessageChannels())
  const channels = useMemo(() => data ?? [], [data])
  const showLoading = loading || retrying

  const filteredChannels = useMemo(
    () =>
      channels.filter(
        (channel) =>
          channel.type === channelType &&
          channel.enabled === (enabledStatus === "enabled"),
      ),
    [channelType, channels, enabledStatus],
  )
  const typeChannelCount = channels.filter(
    (channel) => channel.type === channelType,
  ).length

  /** 无效的类别参数回到默认渠道类别。 */
  useEffect(() => {
    if (!typeDefinition) navigate("/channels", { replace: true })
  }, [navigate, typeDefinition])

  const statusChange = useConfirmedAction<MessageChannelSummary>({
    action: (channel) =>
      channel.enabled
        ? deactivateMessageChannel(channel.id)
        : activateMessageChannel(channel.id),
    invalidateKeys: () => [
      resourceKeys.messageChannels(),
      resourceKeys.channelOptions(),
    ],
    logLabel: "切换消息渠道状态",
    errorMessage: () => t("list.statusUpdateError"),
  })

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
      <PageHeader
        title={
          typeDefinition
            ? t(`types.${typeDefinition.translationKey}`)
            : t("list.title")
        }
        description={t("list.description")}
      >
        <Button variant="ghost" size="icon-sm" asChild>
          <Link
            to={`/channels/${channelType}/new`}
            aria-label={t("list.create")}
            title={t("list.create")}
          >
            <PlusIcon />
          </Link>
        </Button>
      </PageHeader>

      <ListToolbar>
        <ListToolbarFilter
          label={t("filters.status")}
          value={enabledStatus}
          options={[
            { value: "enabled", label: t("statuses.enabled") },
            { value: "disabled", label: t("statuses.disabled") },
          ]}
          onValueChange={(value) =>
            setEnabledStatus(value as ChannelEnabledStatus)
          }
        />
        {enabledStatus !== "enabled" ? (
          <ListToolbarReset onClick={() => setEnabledStatus("enabled")}>
            {t("common:actions.clearFilters")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>

      <ResourceListLayout
        loading={showLoading}
        error={Boolean(error)}
        errorMessage={t("list.loadError")}
        onRetry={() => void refresh()}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "name",
              header: t("list.columns.name"),
              cellClassName: "min-w-44 font-medium",
              cell: (channel) => {
                const definition = messageChannelTypeDefinition(channel.type)
                return (
                  <div className="flex items-center gap-2.5">
                    <span
                      aria-hidden="true"
                      className={cn(
                        "flex size-7 shrink-0 items-center justify-center rounded-lg",
                        definition?.softClassName ?? "bg-muted text-muted-foreground",
                      )}
                    >
                      {definition ? <definition.icon className="size-4" /> : null}
                    </span>
                    <span className="truncate">{channel.name}</span>
                  </div>
                )
              },
            },
          ]}
          rows={filteredChannels}
          rowKey={(channel) => channel.id}
          empty={
            typeChannelCount === 0
              ? t("list.emptyTitle")
              : t("list.emptyFiltered")
          }
          onRowActivate={(channel) =>
            navigate(
              `/channels/${channel.type}/${channel.id}${enabledStatus === "enabled" ? "" : "?status=disabled"}`,
            )
          }
          rowActions={(channel) => [
            {
              key: "status",
              label: channel.enabled ? t("list.deactivate") : t("list.activate"),
              disabled:
                statusChange.pending && statusChange.item?.id === channel.id,
              destructive: channel.enabled,
              separatorBefore: channel.enabled,
              onSelect: () => statusChange.select(channel),
            },
          ]}
        />
      </ResourceListLayout>

      <ConfirmationDialog
        open={statusChange.item !== null}
        pending={statusChange.pending}
        title={
          statusChange.item
            ? t(
                statusChange.item.enabled
                  ? "deactivation.title"
                  : "activation.title",
                { name: statusChange.item.name },
              )
            : ""
        }
        description={
          statusChange.item
            ? t(
                statusChange.item.enabled
                  ? "deactivation.description"
                  : "activation.description",
              )
            : ""
        }
        destructive={statusChange.item?.enabled ?? true}
        onOpenChange={(open) => {
          if (!open) statusChange.select(null)
        }}
        onConfirm={() => void statusChange.confirm()}
      />
    </div>
  )
}
