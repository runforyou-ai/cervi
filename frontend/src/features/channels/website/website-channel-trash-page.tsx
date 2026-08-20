/** 网站渠道回收站页。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  listDeletedWebsiteChannels,
  recoverSession,
  restoreWebsiteChannel,
  type WebsiteChannelSummary,
} from "@/api"
import { PageBackHeader } from "@/components/page-back-header"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useDateTime } from "@/hooks/use-date-time"

/** 已删除的网站渠道。 */
export function WebsiteChannelTrashPage() {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [channels, setChannels] = useState<WebsiteChannelSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [restoringId, setRestoringId] = useState("")
  const [error, setError] = useState("")

  /** 加载已删除的网站渠道。 */
  const loadChannels = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      setChannels(await listDeletedWebsiteChannels())
    } catch (requestError) {
      if (recoverSession(requestError, navigate)) {
        return
      }
      console.warn("网站渠道回收站加载失败", requestError)
      setError(t("trash.loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void loadChannels()
  }, [loadChannels])

  /** 恢复网站渠道。 */
  async function handleRestore(channel: WebsiteChannelSummary) {
    setRestoringId(channel.id)
    try {
      await restoreWebsiteChannel(channel.id)
      console.info("网站渠道已恢复", { channel_id: channel.id })
      setChannels((current) =>
        current.filter((item) => item.id !== channel.id)
      )
    } catch (requestError) {
      if (recoverSession(requestError, navigate)) {
        return
      }
      console.warn("恢复网站渠道失败", requestError)
      toast.error(t("trash.restoreError"))
    } finally {
      setRestoringId("")
    }
  }

  return (
    <div className="w-full px-4 py-6 sm:px-6 lg:px-8">
      <PageBackHeader to="/channels/website" title={t("trash.title")} />

      <div>
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
                  <TableHead>{t("list.columns.description")}</TableHead>
                  <TableHead>{t("list.columns.language")}</TableHead>
                  <TableHead>{t("trash.columns.deletedAt")}</TableHead>
                  <TableHead className="text-right">
                    {t("list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {channels.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={5}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("trash.emptyTitle")}
                    </TableCell>
                  </TableRow>
                ) : (
                  channels.map((channel) => (
                    <TableRow key={channel.id}>
                      <TableCell className="min-w-44 font-medium">
                        {channel.name}
                      </TableCell>
                      <TableCell className="min-w-64 max-w-xl text-muted-foreground">
                        <span className="line-clamp-2">
                          {channel.description || t("list.noDescription")}
                        </span>
                      </TableCell>
                      <TableCell className="whitespace-nowrap">
                        {t(
                          `locales.${
                            channel.defaultLocale === "zh-CN"
                              ? "zhCN"
                              : "enUS"
                          }`
                        )}
                      </TableCell>
                      <TableCell className="whitespace-nowrap text-muted-foreground">
                        {channel.deletedAt
                          ? formatDateTime(channel.deletedAt)
                          : "—"}
                      </TableCell>
                      <TableCell className="text-right whitespace-nowrap">
                        <Button
                          variant="outline"
                          size="sm"
                          disabled={restoringId === channel.id}
                          onClick={() => void handleRestore(channel)}
                        >
                          {restoringId === channel.id
                            ? t("trash.restoring")
                            : t("trash.restore")}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </div>
    </div>
  )
}
