/** AI 供应商列表页。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteAIProvider,
  isApiError,
  listAIProviders,
  type AIProviderSummary,
} from "@/api"
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
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 加载并管理企业 AI 供应商列表。 */
export function AIProviderListPage() {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const [providers, setProviders] = useState<AIProviderSummary[]>([])
  const [deletingProvider, setDeletingProvider] =
    useState<AIProviderSummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)

  /** 读取 AI 供应商列表。 */
  const load = useCallback(async () => {
    const version = ++loadVersion.current
    setLoading(true)
    setError(false)
    try {
      const output = await listAIProviders()
      if (version !== loadVersion.current) return
      setProviders(output.providers)
    } catch (requestError) {
      if (version !== loadVersion.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("AI 供应商列表加载失败", requestError)
      setError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [navigate])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

  /** 删除选中的 AI 供应商。 */
  async function confirmDelete() {
    if (!deletingProvider || deleting) return
    setDeleting(true)
    try {
      await deleteAIProvider(deletingProvider.id)
      if (!mounted.current) return
      setProviders((current) =>
        current.filter((provider) => provider.id !== deletingProvider.id),
      )
      setDeletingProvider(null)
      toast.success(t("aiProviders.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("AI 供应商删除失败", {
        provider_id: deletingProvider.id,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("aiProviders.delete.error"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("aiProviders.title")}>
        <Button size="sm" asChild>
          <Link to="/settings/ai-providers/new">
            {t("aiProviders.list.create")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("aiProviders.loading")}
          </div>
        ) : error ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("aiProviders.list.loadError")}
            </p>
            <Button className="mt-4" variant="outline" onClick={load}>
              {t("aiProviders.retry")}
            </Button>
          </div>
        ) : (
          <div className="overflow-hidden rounded-lg border bg-card">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead>{t("aiProviders.list.columns.brand")}</TableHead>
                  <TableHead>{t("aiProviders.list.columns.name")}</TableHead>
                  <TableHead>{t("aiProviders.list.columns.apiUrl")}</TableHead>
                  <TableHead className="text-right">
                    {t("aiProviders.list.columns.actions")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {providers.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-32 text-center text-muted-foreground"
                    >
                      {t("aiProviders.list.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  providers.map((provider) => (
                    <TableRow key={provider.id}>
                      <TableCell>{t("aiProviders.brands.deepseek")}</TableCell>
                      <TableCell className="font-medium">
                        <SelectableText>{provider.name}</SelectableText>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        <SelectableText>{provider.apiUrl}</SelectableText>
                      </TableCell>
                      <TableCell className="text-right whitespace-nowrap">
                        <div className="flex justify-end gap-2">
                          <Button variant="outline" size="sm" asChild>
                            <Link to={`/settings/ai-providers/${provider.id}`}>
                              {t("aiProviders.list.edit")}
                            </Link>
                          </Button>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                aria-label={t("aiProviders.list.more")}
                                title={t("aiProviders.list.more")}
                              >
                                <MoreHorizontalIcon />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem
                                className="text-destructive focus:text-destructive"
                                onSelect={() => setDeletingProvider(provider)}
                              >
                                {t("aiProviders.list.delete")}
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        )}
      </PageContent>

      <AlertDialog
        open={deletingProvider !== null}
        onOpenChange={(open) => !open && !deleting && setDeletingProvider(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingProvider
                ? t("aiProviders.delete.title", {
                    name: deletingProvider.name,
                  })
                : null}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("aiProviders.delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("aiProviders.delete.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDelete()}
            >
              {deleting
                ? t("aiProviders.delete.deleting")
                : t("aiProviders.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
