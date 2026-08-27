/** 模型服务供应商列表页。 */
import { useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteAIProvider,
  isApiError,
  listAIProviders,
  type AIProviderModelSummaryData,
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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { aiProviderBrandConfigs } from "@/features/integrations/model-services/model-provider-brands"
import {
  modelServiceSectionConfigs,
  modelServiceSectionOrder,
  type ModelServiceSection,
} from "@/features/integrations/model-services/model-service-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const visibleModelLimit = 3

/** 显示供应商当前类型的模型摘要和完整悬浮目录。 */
function ProviderModelsCell({
  models,
}: {
  models: AIProviderModelSummaryData[]
}) {
  const { t } = useTranslation("integrations")
  if (models.length === 0) return "—"

  const summary = models
    .slice(0, visibleModelLimit)
    .map((model) => model.name)
    .join(t("modelServices.list.modelSeparator"))
  const visibleSummary =
    models.length > visibleModelLimit
      ? `${summary}${t("modelServices.list.modelOverflow")}`
      : summary

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          className="block max-w-sm cursor-help truncate outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {visibleSummary}
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={4} className="max-w-md">
        <ul className="grid gap-1 text-left">
          {models.map((model) => (
            <li key={model.identifier} className="break-words">
              <span className="font-mono">{model.identifier}</span>
              <span aria-hidden="true"> — </span>
              <span>{model.name}</span>
            </li>
          ))}
        </ul>
      </TooltipContent>
    </Tooltip>
  )
}

/** 显示指定类型的模型服务供应商。 */
export function ModelProviderListPage({
  section,
}: {
  section: ModelServiceSection
}) {
  const { t } = useTranslation("integrations")
  const navigate = useNavigate()
  const [deletingProvider, setDeletingProvider] =
    useState<AIProviderSummaryData | null>(null)
  const [deleting, setDeleting] = useState(false)
  const mounted = useRef(true)
  const sectionConfig = modelServiceSectionConfigs[section]
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.aiProviders(),
    () => listAIProviders(),
  )
  const invalidate = useResourceInvalidator()
  const showLoading = loading || (Boolean(error) && refreshing)
  const providers = data?.providers ?? []
  const visibleProviders = providers.filter(
    (provider) =>
      provider.models.some((model) => model.type === sectionConfig.modelType),
  )

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 删除选中的模型服务供应商。 */
  async function confirmDelete() {
    if (!deletingProvider || deleting) return
    setDeleting(true)
    try {
      await deleteAIProvider(deletingProvider.id)
      if (!mounted.current) return
      void refresh()
      void invalidate(resourceKeys.aiProvider(deletingProvider.id))
      setDeletingProvider(null)
      toast.success(t("modelServices.delete.success"))
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
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
          {showLoading ? (
            <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
              <LoaderCircleIcon className="size-4 animate-spin" />
              {t("modelServices.loading")}
            </div>
          ) : error ? (
            <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
              <p className="text-sm text-muted-foreground">
                {t("modelServices.list.loadError")}
              </p>
              <Button
                className="mt-4"
                variant="outline"
                onClick={() => void refresh()}
              >
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
                    <TableHead>{t("modelServices.list.columns.models")}</TableHead>
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
                          <ProviderModelsCell
                            models={provider.models.filter(
                              (model) => model.type === sectionConfig.modelType,
                            )}
                          />
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
