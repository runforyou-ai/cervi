import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  listDeletedWebsiteChannels,
  restoreWebsiteChannel,
  type WebsiteChannelSummary,
} from "@/api/channels"
import { ApiError } from "@/api/client"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
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

export function WebsiteChannelTrashPage() {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [channels, setChannels] = useState<WebsiteChannelSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [restoringId, setRestoringId] = useState("")
  const [error, setError] = useState("")

  const loadChannels = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      setChannels(await listDeletedWebsiteChannels())
    } catch (requestError) {
      if (
        requestError instanceof ApiError &&
        requestError.code === "AUTH_REQUIRED"
      ) {
        navigate("/login", { replace: true })
        return
      }
      setError(t("trash.loadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void loadChannels()
  }, [loadChannels])

  async function handleRestore(channel: WebsiteChannelSummary) {
    setRestoringId(channel.id)
    try {
      await restoreWebsiteChannel(channel.id)
      setChannels((current) =>
        current.filter((item) => item.id !== channel.id)
      )
    } catch (requestError) {
      if (
        requestError instanceof ApiError &&
        requestError.code === "AUTH_REQUIRED"
      ) {
        navigate("/login", { replace: true })
        return
      }
      toast.error(t("trash.restoreError"))
    } finally {
      setRestoringId("")
    }
  }

  return (
    <div className="w-full px-4 py-6 sm:px-6 lg:px-8">
      <Breadcrumb className="mb-6">
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to="/channels/website">{t("list.title")}</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{t("trash.title")}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <h2 className="text-xl font-semibold tracking-tight">
        {t("trash.title")}
      </h2>

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
