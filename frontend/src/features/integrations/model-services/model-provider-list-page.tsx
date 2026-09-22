/** 模型服务供应商列表页。 */
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"

import {
  deleteAIProvider,
  listAIProviders,
  type AIProviderModelSummaryData,
  type AIProviderSummaryData,
} from "@/api"
import { ResourceListLayout } from "@/components/resource-list"
import { ResourceTable } from "@/components/resource-table"
import { PageHeader } from "@/components/page-header"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { Button } from "@/components/ui/button"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { aiProviderBrandConfigs } from "@/features/integrations/model-services/model-provider-brands"
import {
  modelServiceSectionConfigs,
  modelServiceSectionOrder,
  type ModelServiceSection,
} from "@/features/integrations/model-services/model-service-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource } from "@/hooks/use-resource"

/** 显示供应商名称、品牌和当前类型的模型摘要。 */
function ProviderCell({
  brand,
  name,
  models,
}: {
  brand: string
  name: string
  models: AIProviderModelSummaryData[]
}) {
  const { t } = useTranslation("integrations")
  const summary = models
    .map((model) => model.name)
    .join(t("modelServices.list.modelSeparator"))

  return (
    <div className="min-w-0">
      <p className="truncate font-medium">
        {brand}
        <span className="text-muted-foreground"> · </span>
        {name}
      </p>
      {summary ? (
        <p className="truncate text-xs text-muted-foreground">{summary}</p>
      ) : null}
    </div>
  )
}

/** 显示指定类型的模型服务供应商。 */
export function ModelProviderListPage({ section }: { section: ModelServiceSection }) {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const sectionConfig = modelServiceSectionConfigs[section]
  const { data, loading, retrying, error, refresh } = useResource(
    resourceKeys.aiProviders(),
    () => listAIProviders(),
  )
  const showLoading = loading || retrying
  const providers = data?.providers ?? []
  const visibleProviders = providers.filter((provider) =>
    provider.models.some((model) => model.type === sectionConfig.modelType),
  )

  const deletion = useConfirmedAction<AIProviderSummaryData>({
    action: (provider) => deleteAIProvider(provider.id),
    invalidateKeys: (provider) => [
      resourceKeys.aiProviders(),
      resourceKeys.aiProvider(provider.id),
    ],
    logLabel: "模型服务供应商删除",
    successMessage: () => t("modelServices.delete.success"),
    errorMessage: () => t("modelServices.delete.error"),
  })

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={t("modelServices.title")}
        description={t("modelServices.description")}
      >
        <Button variant="ghost" size="icon-sm" asChild>
          <Link
            to={`/settings/model-services/${section}/new`}
            aria-label={t("modelServices.list.create")}
            title={t("modelServices.list.create")}
          >
            <PlusIcon />
          </Link>
        </Button>
      </PageHeader>
      <div className="cervi-page-gutter flex h-11 shrink-0 items-end select-none">
        <Tabs
          value={section}
          onValueChange={(value) => navigate(`/settings/model-services/${value}`)}
        >
          <TabsList>
            {modelServiceSectionOrder.map((item) => (
              <TabsTrigger key={item} value={item}>
                {t(modelServiceSectionConfigs[item].nameKey)}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      </div>
      <ResourceListLayout
        loading={showLoading}
        error={Boolean(error)}
        errorMessage={t("modelServices.list.loadError")}
        onRetry={() => void refresh()}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "provider",
              header: t("modelServices.list.columns.name"),
              cell: (provider) => (
                <ProviderCell
                  brand={t(aiProviderBrandConfigs[provider.brand].nameKey)}
                  name={provider.name}
                  models={provider.models.filter(
                    (model) => model.type === sectionConfig.modelType,
                  )}
                />
              ),
            },
          ]}
          rows={visibleProviders}
          rowKey={(provider) => provider.id}
          empty={t("modelServices.list.empty", {
            type: t(sectionConfig.nameKey),
          })}
          onRowActivate={(provider) =>
            navigate(`/settings/model-services/${section}/${provider.id}`)
          }
          rowActions={(provider) => [
            {
              key: "delete",
              label: t("common:actions.delete"),
              destructive: true,
              separatorBefore: true,
              onSelect: () => deletion.select(provider),
            },
          ]}
        />
      </ResourceListLayout>

      <ConfirmationDialog
        open={deletion.item !== null}
        pending={deletion.pending}
        title={
          deletion.item
            ? t("modelServices.delete.title", { name: deletion.item.name })
            : ""
        }
        description={t("modelServices.delete.description")}
        pendingLabel={t("common:actions.deleting")}
        onOpenChange={(open) => {
          if (!open) deletion.select(null)
        }}
        onConfirm={() => void deletion.confirm()}
      />
    </div>
  )
}
