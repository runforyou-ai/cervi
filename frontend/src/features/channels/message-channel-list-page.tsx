/** 消息渠道列表页，统一展示当前支持的渠道。 */
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  activateMessageChannel,
  deactivateMessageChannel,
  isApiError,
  listMessageChannels,
  type MessageChannelSummary,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceTable } from "@/components/resource-table"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import {
  messageChannelTypeDefinition,
  messageChannelTypeDefinitions,
} from "@/lib/message-channel-types"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type ChannelEnabledStatus = "enabled" | "disabled"

/** 加载并管理消息渠道列表。 */
export function MessageChannelListPage() {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [search, setSearch] = useState("")
  const [category, setCategory] = useState("")
  const [enabledStatus, setEnabledStatus] =
    useState<ChannelEnabledStatus>("enabled")
  const [updatingChannelId, setUpdatingChannelId] = useState("")
  const [confirmingChannel, setConfirmingChannel] =
    useState<MessageChannelSummary | null>(null)
  const {
    data,
    loading,
    refreshing,
    error,
    refresh,
  } = useResource(resourceKeys.messageChannels(), () => listMessageChannels())
  const channels = useMemo(() => data ?? [], [data])
  const showLoading = loading || (Boolean(error) && refreshing)

  const filteredChannels = useMemo(
    () =>
      channels.filter(
        (channel) =>
          channel.name
            .toLocaleLowerCase()
            .includes(search.trim().toLocaleLowerCase()) &&
          (!category || channel.type === category) &&
          channel.enabled === (enabledStatus === "enabled"),
      ),
    [category, channels, enabledStatus, search],
  )

  /** 切换消息渠道的启用状态。 */
  async function handleStatusChange(channel: MessageChannelSummary) {
    setUpdatingChannelId(channel.id)
    try {
      const updated = channel.enabled
        ? await deactivateMessageChannel(channel.id)
        : await activateMessageChannel(channel.id)
      void refresh()
      void invalidate(resourceKeys.channelOptions())
      console.info("消息渠道状态已更新", {
        channel_id: channel.id,
        channel_type: channel.type,
        enabled: updated.enabled,
      })
    } catch (requestError) {
      if (recoverSession(requestError, navigate)) {
        return
      }
      console.warn("切换消息渠道状态失败", {
        channel_id: channel.id,
        channel_type: channel.type,
        enabled: !channel.enabled,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("list.statusUpdateError"),
      )
    } finally {
      setUpdatingChannelId("")
      setConfirmingChannel(null)
    }
  }

  /** 切换渠道状态前请求确认。 */
  function requestStatusChange(channel: MessageChannelSummary) {
    setConfirmingChannel(channel)
  }

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
      <PageHeader title={t("list.title")}>
        <Button size="sm" asChild>
          <Link to="/integrations/channels/new">{t("list.create")}</Link>
        </Button>
      </PageHeader>

      <ListToolbar>
        <ListToolbarSearch
          value={search}
          aria-label={t("filters.search")}
          onChange={(event) => setSearch(event.target.value)}
        />
        <ListToolbarFilter
          label={t("filters.category")}
          allLabel={t("filters.allCategories")}
          value={category}
          options={messageChannelTypeDefinitions.map((definition) => ({
            value: definition.type,
            label: t(`types.${definition.translationKey}`),
          }))}
          onValueChange={setCategory}
        />
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
        {search || category || enabledStatus !== "enabled" ? (
          <ListToolbarReset
            onClick={() => {
              setSearch("")
              setCategory("")
              setEnabledStatus("enabled")
            }}
          >
            {t("common:actions.clearFilters")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>

      <PageContent>
        {showLoading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border p-6 text-center">
            <p className="text-sm text-muted-foreground">{t("list.loadError")}</p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <ResourceTable
              columns={[
                {
                  key: "name",
                  header: t("list.columns.name"),
                  cellClassName: "min-w-44 font-medium",
                  cell: (channel) => channel.name,
                },
                {
                  key: "category",
                  header: t("list.columns.category"),
                  cellClassName: "whitespace-nowrap",
                  cell: (channel) => {
                    const typeDefinition = messageChannelTypeDefinition(
                      channel.type,
                    )
                    if (!typeDefinition) {
                      console.warn("未知的消息渠道类型", channel.type)
                      return ""
                    }
                    return t(`types.${typeDefinition.translationKey}`)
                  },
                },
                {
                  key: "language",
                  header: t("list.columns.language"),
                  cellClassName: "whitespace-nowrap",
                  cell: (channel) =>
                    t(
                      `locales.${channel.defaultLocale === "zh-CN" ? "zhCN" : "enUS"}`,
                    ),
                },
              ]}
              rows={filteredChannels}
              rowKey={(channel) => channel.id}
              empty={
                channels.length === 0
                  ? t("list.emptyTitle")
                  : t("list.emptyFiltered")
              }
              actions={(channel) => ({
                primary: (
                  <Button variant="outline" size="sm" asChild>
                    <Link
                      to={`/integrations/channels/${channel.type}/${channel.id}`}
                    >
                      {t("common:actions.edit")}
                    </Link>
                  </Button>
                ),
                menu: (
                  <DropdownMenuItem
                    destructive={channel.enabled}
                    disabled={updatingChannelId === channel.id}
                    onSelect={() => requestStatusChange(channel)}
                  >
                    {channel.enabled
                      ? t("list.deactivate")
                      : t("list.activate")}
                  </DropdownMenuItem>
                ),
              })}
            />
          </div>
        )}
      </PageContent>

      {confirmingChannel ? (
        <AlertDialog
          open
          onOpenChange={(open) => {
            if (!open) {
              setConfirmingChannel(null)
            }
          }}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t(
                  confirmingChannel.enabled
                    ? "deactivation.title"
                    : "activation.title",
                  { name: confirmingChannel.name },
                )}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t(
                  confirmingChannel.enabled
                    ? "deactivation.description"
                    : "activation.description",
                )}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>
                {t("common:actions.cancel")}
              </AlertDialogCancel>
              <AlertDialogAction
                className={
                  confirmingChannel.enabled
                    ? undefined
                    : "bg-primary text-primary-foreground hover:bg-primary/90"
                }
                disabled={updatingChannelId !== ""}
                onClick={() => void handleStatusChange(confirmingChannel)}
              >
                {confirmingChannel.enabled
                  ? t("deactivation.confirm")
                  : t("activation.confirm")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      ) : null}
    </div>
  )
}
