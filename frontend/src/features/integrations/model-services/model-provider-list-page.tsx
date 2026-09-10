/** 模型服务供应商列表页。 */
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"

import {
  deleteAIProvider,
  listAIProviders,
  type AIProviderModelSummaryData,
  type AIProviderSummaryData,
} from "@/api"
import { ResourceContent } from "@/components/resource-content"
import {
  ResourceTable,
  ResourceTableActions,
} from "@/components/resource-table"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { SelectableText } from "@/components/selectable-text"
import { DeleteConfirmationDialog } from "@/components/delete-confirmation-dialog"
import { Button } from "@/components/ui/button"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { TableCell } from "@/components/ui/table"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { aiProviderBrandConfigs } from "@/features/integrations/model-services/model-provider-brands"
import {
  modelServiceSectionConfigs,
  modelServiceSectionOrder,
  type ModelServiceSection,
} from "@/features/integrations/model-services/model-service-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { useIntegrationDeletion } from "@/features/integrations/use-integration-deletion"

const visibleModelLimit = 3

/** 显示供应商当前类型的模型摘要和完整悬浮目录。 */
function ProviderModelsCell({ models }: { models: AIProviderModelSummaryData[] }) {
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
export function ModelProviderListPage({ section }: { section: ModelServiceSection }) {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const sectionConfig = modelServiceSectionConfigs[section]
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.aiProviders(),
    () => listAIProviders(),
  )
  const showLoading = loading || (Boolean(error) && refreshing)
  const providers = data?.providers ?? []
  const visibleProviders = providers.filter((provider) =>
    provider.models.some((model) => model.type === sectionConfig.modelType),
  )

  const deletion = useIntegrationDeletion<AIProviderSummaryData>({
    deleteItem: deleteAIProvider,
    listKey: resourceKeys.aiProviders(),
    detailKey: resourceKeys.aiProvider,
    entityName: "模型服务供应商",
    successMessage: t("modelServices.delete.success"),
    errorMessage: t("modelServices.delete.error"),
  })

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
          onValueChange={(value) => navigate(`/integrations/model-services/${value}`)}
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
          <ResourceContent
            loading={showLoading}
            error={Boolean(error)}
            errorMessage={t("modelServices.list.loadError")}
            onRetry={() => void refresh()}
          >
            <div className="overflow-hidden rounded-lg border bg-card">
              <ResourceTable
                columns={[
                  { key: "brand", header: t("modelServices.list.columns.brand") },
                  { key: "name", header: t("modelServices.list.columns.name") },
                  {
                    key: "models",
                    header: t("modelServices.list.columns.models"),
                  },
                  {
                    key: "apiUrl",
                    header: t("modelServices.list.columns.apiUrl"),
                  },
                  {
                    key: "actions",
                    header: t("common:table.actions"),
                    className: "w-px",
                  },
                ]}
                rows={visibleProviders}
                rowKey={(provider) => provider.id}
                empty={t("modelServices.list.empty", {
                  type: t(sectionConfig.nameKey),
                })}
              >
                {(provider) => (
                  <>
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
                    <ResourceTableActions
                      menu={
                        <DropdownMenuItem
                          destructive
                          onSelect={() => deletion.select(provider)}
                        >
                          {t("common:actions.delete")}
                        </DropdownMenuItem>
                      }
                    >
                      <Button variant="outline" size="sm" asChild>
                        <Link
                          to={`/integrations/model-services/${section}/${provider.id}`}
                        >
                          {t("common:actions.edit")}
                        </Link>
                      </Button>
                    </ResourceTableActions>
                  </>
                )}
              </ResourceTable>
            </div>
          </ResourceContent>
        </div>
      </PageContent>

      <DeleteConfirmationDialog
        open={deletion.item !== null}
        pending={deletion.pending}
        title={
          deletion.item
            ? t("modelServices.delete.title", { name: deletion.item.name })
            : ""
        }
        description={t("modelServices.delete.description")}
        onOpenChange={(open) => {
          if (!open) deletion.select(null)
        }}
        onConfirm={() => void deletion.confirm()}
      />
    </div>
  )
}
