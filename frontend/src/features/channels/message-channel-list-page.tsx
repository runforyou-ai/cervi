/** 消息渠道列表页，统一展示当前支持的渠道。 */
import { useMemo, useState } from "react"
import { PlusIcon } from "lucide-react"
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
import {
  ListToolbar,
  ListToolbarFilter,
  ListToolbarReset,
} from "@/components/list-toolbar"
import { ListActionButton } from "@/components/list-action-button"
import { PageHeader } from "@/components/page-header"
import { ResourceListLayout } from "@/components/resource-list"
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
import {
  messageChannelTypeDefinition,
  messageChannelTypeDefinitions,
} from "@/lib/message-channel-types"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

type ChannelEnabledStatus = "enabled" | "disabled"

/** 加载并管理消息渠道列表。 */
export function MessageChannelListPage() {
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
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
          (!category || channel.type === category) &&
          channel.enabled === (enabledStatus === "enabled"),
      ),
    [category, channels, enabledStatus],
  )

  /** 切换消息渠道的启用状态。 */
  async function handleStatusChange(channel: MessageChannelSummary) {
    setUpdatingChannelId(channel.id)
    try {
      await (channel.enabled
        ? deactivateMessageChannel(channel.id)
        : activateMessageChannel(channel.id))
      void refresh()
      void invalidate(resourceKeys.channelOptions())
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
      <PageHeader
        title={t("list.title")}
        description={t("list.description")}
      >
        <Button variant="ghost" size="icon-sm" asChild>
          <Link
            to="/settings/channels/new"
            aria-label={t("list.create")}
            title={t("list.create")}
          >
            <PlusIcon />
          </Link>
        </Button>
      </PageHeader>

      <ListToolbar>
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
        {category || enabledStatus !== "enabled" ? (
          <ListToolbarReset
            onClick={() => {
              setCategory("")
              setEnabledStatus("enabled")
            }}
          >
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
            channels.length === 0
              ? t("list.emptyTitle")
              : t("list.emptyFiltered")
          }
          onRowActivate={(channel) =>
            navigate(`/settings/channels/${channel.type}/${channel.id}`)
          }
          actions={(channel) => {
            const label = channel.enabled
              ? t("list.deactivate")
              : t("list.activate")
            return {
              primary: (
                <ListActionButton
                  tone={channel.enabled ? "destructive" : "success"}
                  disabled={updatingChannelId === channel.id}
                  onClick={() => requestStatusChange(channel)}
                >
                  {label}
                </ListActionButton>
              ),
            }
          }}
        />
      </ResourceListLayout>

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
                {t("common:actions.confirm")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      ) : null}
    </div>
  )
}
