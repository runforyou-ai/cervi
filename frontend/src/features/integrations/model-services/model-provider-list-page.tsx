/** 模型服务供应商列表页。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteAIProvider,
  isApiError,
  listAIProviders,
  type AIProviderSummaryData,
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { aiProviderBrandConfigs } from "@/features/integrations/model-services/model-provider-brands"
import {
  modelServiceSectionConfigs,
  modelServiceSectionOrder,
  modelTypeNameKeys,
  type ModelServiceSection,
} from "@/features/integrations/model-services/model-service-options"
import { apiErrorMessage } from "@/lib/form-errors"
import { useSessionRecovery } from "@/lib/session-navigation"

/** 显示指定类型的模型服务供应商。 */
export function ModelProviderListPage({
  section,
}: {
  section: ModelServiceSection
}) {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const recoverSession = useSessionRecovery()
  const [providers, setProviders] = useState<AIProviderSummaryData[]>([])
  const [deletingProvider, setDeletingProvider] =
    useState<AIProviderSummaryData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const sectionConfig = modelServiceSectionConfigs[section]
  const visibleProviders = providers.filter(
    (provider) => provider.modelTypes.includes(sectionConfig.modelType),
  )

  /** 读取模型服务供应商列表。 */
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
      if (recoverSession(requestError)) return
      console.warn("模型服务供应商列表加载失败", requestError)
      setError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [recoverSession])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

  /** 删除选中的模型服务供应商。 */
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
      toast.success(t("modelServices.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError)) return
      console.warn("模型服务供应商删除失败", {
        provider_id: deletingProvider.id,
        error: requestError,
      })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError)
          : t("modelServices.delete.error"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("modelServices.title")}>
        <Button size="sm" asChild>
          <Link to={`/integrations/model-services/${section}/new`}>
            {t("modelServices.list.create")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent>
        <Tabs
          value={section}
          onValueChange={(value) =>
            navigate(`/integrations/model-services/${value}`)
          }
        >
          <TabsList>
            {modelServiceSectionOrder.map((item) => (
              <TabsTrigger key={item} value={item}>
                {t(modelServiceSectionConfigs[item].nameKey)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <div className="mt-6">
          {loading ? (
            <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
              <LoaderCircleIcon className="size-4 animate-spin" />
              {t("modelServices.loading")}
            </div>
          ) : error ? (
            <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
              <p className="text-sm text-muted-foreground">
                {t("modelServices.list.loadError")}
              </p>
              <Button className="mt-4" variant="outline" onClick={load}>
                {t("modelServices.retry")}
              </Button>
            </div>
          ) : (
            <div className="overflow-hidden rounded-lg border bg-card">
              <Table>
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead>{t("modelServices.list.columns.brand")}</TableHead>
                    <TableHead>{t("modelServices.list.columns.name")}</TableHead>
                    <TableHead>{t("modelServices.list.columns.capabilities")}</TableHead>
                    <TableHead>{t("modelServices.list.columns.apiUrl")}</TableHead>
                    <TableHead className="text-right">
                      {t("modelServices.list.columns.actions")}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {visibleProviders.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell
                        colSpan={5}
                        className="h-32 text-center text-muted-foreground"
                      >
                        {t("modelServices.list.empty", {
                          type: t(sectionConfig.nameKey),
                        })}
                      </TableCell>
                    </TableRow>
                  ) : (
                    visibleProviders.map((provider) => (
                      <TableRow key={provider.id}>
                        <TableCell>
                          {t(aiProviderBrandConfigs[provider.brand].nameKey)}
                        </TableCell>
                        <TableCell className="font-medium">
                          <SelectableText>{provider.name}</SelectableText>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {provider.modelTypes
                            .map((type) => t(modelTypeNameKeys[type]))
                            .join("、")}
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          <SelectableText>{provider.apiUrl}</SelectableText>
                        </TableCell>
                        <TableCell className="text-right whitespace-nowrap">
                          <div className="flex justify-end gap-2">
                            <Button variant="outline" size="sm" asChild>
                              <Link
                                to={`/integrations/model-services/${section}/${provider.id}`}
                              >
                                {t("modelServices.list.edit")}
                              </Link>
                            </Button>
                            <DropdownMenu>
                              <DropdownMenuTrigger asChild>
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  aria-label={t("modelServices.list.more")}
                                  title={t("modelServices.list.more")}
                                >
                                  <MoreHorizontalIcon />
                                </Button>
                              </DropdownMenuTrigger>
                              <DropdownMenuContent align="end">
                                <DropdownMenuItem
                                  className="text-destructive focus:text-destructive"
                                  onSelect={() => setDeletingProvider(provider)}
                                >
                                  {t("modelServices.list.delete")}
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
        </div>
      </PageContent>

      <AlertDialog
        open={deletingProvider !== null}
        onOpenChange={(open) => !open && !deleting && setDeletingProvider(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingProvider
                ? t("modelServices.delete.title", {
                    name: deletingProvider.name,
                  })
                : null}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("modelServices.delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("modelServices.delete.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDelete()}
            >
              {deleting
                ? t("modelServices.delete.deleting")
                : t("modelServices.delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
