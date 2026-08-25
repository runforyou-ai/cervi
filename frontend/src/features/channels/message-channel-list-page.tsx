/** 消息渠道列表页，统一展示当前支持的渠道。 */
import { useCallback, useEffect, useMemo, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

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
  ListToolbarSearch,
} from "@/components/list-toolbar"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { SelectableText } from "@/components/selectable-text"
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
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  messageChannelTypeDefinition,
  messageChannelTypeDefinitions,
} from "@/features/channels/message-channel-types"
import { recoverSession } from "@/lib/session-navigation"

type ChannelEnabledStatus = "enabled" | "disabled"

/** 消息渠道列表中的一行。 */
function MessageChannelRow({
  channel,
  updating,
  onStatusChange,
}: {
  channel: MessageChannelSummary
  updating: boolean
  onStatusChange: (channel: MessageChannelSummary) => void
}) {
  const { t } = useTranslation("channels")
  const typeDefinition = messageChannelTypeDefinition(channel.type)
  if (!typeDefinition) {
    console.warn("未知的消息渠道类型", channel.type)
  }

  return (
    <TableRow>
      <TableCell className="min-w-44 font-medium">
        <SelectableText>{channel.name}</SelectableText>
      </TableCell>
      <TableCell className="whitespace-nowrap">
        {typeDefinition ? t(`types.${typeDefinition.translationKey}`) : ""}
      </TableCell>
      <TableCell className="whitespace-nowrap">
        {t(
          `locales.${channel.defaultLocale === "zh-CN" ? "zhCN" : "enUS"}`
        )}
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={`/integrations/channels/${channel.type}/${channel.id}`}>
              {t("list.edit")}
            </Link>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("list.more")}
                title={t("list.more")}
              >
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className={
                  channel.enabled
                    ? "text-destructive focus:text-destructive"
                    : undefined
                }
                disabled={updating}
                onSelect={() => onStatusChange(channel)}
              >
                {channel.enabled ? t("list.deactivate") : t("list.activate")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TableCell>
    </TableRow>
  )
}

/** 加载并管理消息渠道列表。 */
export function MessageChannelListPage() {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const [channels, setChannels] = useState<MessageChannelSummary[]>([])
  const [search, setSearch] = useState("")
  const [category, setCategory] = useState("")
  const [enabledStatus, setEnabledStatus] =
    useState<ChannelEnabledStatus>("enabled")
  const [updatingChannelId, setUpdatingChannelId] = useState("")
  const [confirmingChannel, setConfirmingChannel] =
    useState<MessageChannelSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  /** 加载消息渠道列表。 */
  const loadChannels = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      setChannels(await listMessageChannels())
    } catch (requestError) {
      if (recoverSession(requestError, navigate)) {
        return
      }
      console.warn("消息渠道列表加载失败", requestError)
      setError(t("list.loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void loadChannels()
  }, [loadChannels])

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
      setChannels((current) =>
        current.map((item) => (item.id === updated.id ? updated : item)),
      )
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
      toast.error(t("list.statusUpdateError"))
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
            {t("filters.clear")}
          </ListToolbarReset>
        ) : null}
      </ListToolbar>

      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("loading")}
          </div>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border p-6 text-center">
            <p className="text-sm text-muted-foreground">{error}</p>
            <Button className="mt-4" variant="outline" onClick={loadChannels}>
              {t("retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("list.columns.name")}</TableHead>
                  <TableHead>{t("list.columns.category")}</TableHead>
                  <TableHead>{t("list.columns.language")}</TableHead>
                  <TableHead className="text-right">
                    {t("list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {filteredChannels.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {channels.length === 0
                        ? t("list.emptyTitle")
                        : t("list.emptyFiltered")}
                    </TableCell>
                  </TableRow>
                ) : (
                  filteredChannels.map((channel) => (
                    <MessageChannelRow
                      key={channel.id}
                      channel={channel}
                      updating={updatingChannelId === channel.id}
                      onStatusChange={requestStatusChange}
                    />
                  ))
                )}
              </TableBody>
            </Table>
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
                {t("statusConfirmation.cancel")}
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
