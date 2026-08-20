/** 网站渠道列表页。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ApiError,
  deleteWebsiteChannel,
  listWebsiteChannels,
  type WebsiteChannelSummary,
} from "@/api"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
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

/** 网站渠道列表中的一行。 */
function WebsiteChannelRow({
  channel,
  onDeleteRequested,
}: {
  channel: WebsiteChannelSummary
  onDeleteRequested: (channel: WebsiteChannelSummary) => void
}) {
  const { t } = useTranslation("channels")

  return (
    <TableRow>
      <TableCell className="min-w-44 font-medium">
        <Link
          className="hover:underline"
          to={`/channels/website/${channel.id}`}
        >
          {channel.name}
        </Link>
      </TableCell>
      <TableCell className="whitespace-nowrap">
        {t(
          `locales.${channel.defaultLocale === "zh-CN" ? "zhCN" : "enUS"}`
        )}
      </TableCell>
      <TableCell className="text-right whitespace-nowrap">
        <div className="flex justify-end gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={`/channels/website/${channel.id}`}>
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
                className="text-destructive focus:text-destructive"
                onSelect={() => onDeleteRequested(channel)}
              >
                {t("list.delete")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </TableCell>
    </TableRow>
  )
}

/** 展示网站渠道列表并支持删除。 */
export function WebsiteChannelListPage() {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const [channels, setChannels] = useState<WebsiteChannelSummary[]>([])
  const [deletingChannel, setDeletingChannel] =
    useState<WebsiteChannelSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  /** 加载网站渠道列表。 */
  const loadChannels = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      setChannels(await listWebsiteChannels())
    } catch (requestError) {
      if (
        requestError instanceof ApiError &&
        requestError.code === "AUTH_REQUIRED"
      ) {
        navigate("/login", { replace: true })
        return
      }
      setError(t("list.loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void loadChannels()
  }, [loadChannels])

  /** 将网站渠道移入回收站。 */
  async function handleDelete(channel: WebsiteChannelSummary) {
    try {
      await deleteWebsiteChannel(channel.id)
      setChannels((current) =>
        current.filter((item) => item.id !== channel.id)
      )
      setDeletingChannel(null)
    } catch (requestError) {
      if (
        requestError instanceof ApiError &&
        requestError.code === "AUTH_REQUIRED"
      ) {
        navigate("/login", { replace: true })
        return
      }
      toast.error(t("delete.error"))
    }
  }

  return (
    <div className="w-full px-4 py-6 sm:px-6 lg:px-8">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <h2 className="text-xl font-semibold tracking-tight">
          {t("list.title")}
        </h2>
        <div className="flex shrink-0 items-center gap-2">
          <Button size="sm" asChild>
            <Link to="/channels/website/new">{t("list.create")}</Link>
          </Button>
          <Button variant="outline" size="sm" asChild>
            <Link to="/channels/website/trash">{t("list.trash")}</Link>
          </Button>
        </div>
      </div>

      <div className="mt-6">
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
                  <TableHead>{t("list.columns.language")}</TableHead>
                  <TableHead className="text-right">
                    {t("list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {channels.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={3}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("list.emptyTitle")}
                    </TableCell>
                  </TableRow>
                ) : (
                  channels.map((channel) => (
                    <WebsiteChannelRow
                      key={channel.id}
                      channel={channel}
                      onDeleteRequested={setDeletingChannel}
                    />
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </div>

      <AlertDialog
        open={deletingChannel !== null}
        onOpenChange={(open) => {
          if (!open) {
            setDeletingChannel(null)
          }
        }}
      >
        <AlertDialogContent aria-describedby={undefined}>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingChannel
                ? t("delete.title", { name: deletingChannel.name })
                : null}
            </AlertDialogTitle>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("delete.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (deletingChannel) {
                  void handleDelete(deletingChannel)
                }
              }}
            >
              {t("delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
