/** 业务系统列表页。 */
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { deleteBusinessSystem, listBusinessSystems, type BusinessSystem } from "@/api"
import { ResourceContent } from "@/components/resource-content"
import {
  ResourceTable,
  ResourceTableActions,
} from "@/components/resource-table"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { SelectableText } from "@/components/selectable-text"
import { StatusBadge } from "@/components/status-badge"
import { DeleteConfirmationDialog } from "@/components/delete-confirmation-dialog"
import { Button } from "@/components/ui/button"
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { TableCell } from "@/components/ui/table"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { useIntegrationDeletion } from "@/features/integrations/use-integration-deletion"

/** 显示当前企业配置的业务系统。 */
export function BusinessSystemListPage() {
  const { t } = useTranslation(["integrations", "common"])
  const {
    data,
    loading,
    refreshing,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.businessSystems(), () => listBusinessSystems())
  const showLoading = loading || (Boolean(loadError) && refreshing)
  const businessSystems = data?.businessSystems ?? []

  const deletion = useIntegrationDeletion<BusinessSystem>({
    deleteItem: deleteBusinessSystem,
    listKey: resourceKeys.businessSystems(),
    detailKey: resourceKeys.businessSystem,
    entityName: "业务系统",
    successMessage: t("businessSystem.delete.success"),
    errorMessage: t("businessSystem.delete.error"),
  })

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("businessSystem.title")}>
        <Button size="sm" asChild>
          <Link to="/integrations/business-systems/new">
            {t("businessSystem.list.create")}
          </Link>
        </Button>
      </PageHeader>
      <PageContent>
        <ResourceContent
          loading={showLoading}
          error={Boolean(loadError)}
          errorMessage={t("businessSystem.list.loadError")}
          onRetry={() => void refresh()}
        >
          <div className="overflow-hidden rounded-lg border bg-card">
            <ResourceTable
              columns={[
                { key: "name", header: t("businessSystem.list.columns.name") },
                { key: "url", header: t("businessSystem.list.columns.url") },
                {
                  key: "status",
                  header: t("businessSystem.list.columns.status"),
                },
                {
                  key: "actions",
                  header: t("common:table.actions"),
                  className: "w-px",
                },
              ]}
              rows={businessSystems}
              rowKey={(businessSystem) => businessSystem.id}
              empty={t("businessSystem.list.empty")}
            >
              {(businessSystem) => (
                <>
                  <TableCell className="font-medium">
                    <SelectableText>{businessSystem.name}</SelectableText>
                  </TableCell>
                  <TableCell className="max-w-xl text-muted-foreground">
                    <SelectableText>{businessSystem.url}</SelectableText>
                  </TableCell>
                  <TableCell>
                    <StatusBadge
                      variant={businessSystem.enabled ? "success" : "muted"}
                      showDot={false}
                    >
                      {businessSystem.enabled
                        ? t("businessSystem.status.enabled")
                        : t("businessSystem.status.disabled")}
                    </StatusBadge>
                  </TableCell>
                  <ResourceTableActions
                    menu={
                      <DropdownMenuItem
                        destructive
                        onSelect={() => deletion.select(businessSystem)}
                      >
                        {t("common:actions.delete")}
                      </DropdownMenuItem>
                    }
                  >
                    <Button variant="outline" size="sm" asChild>
                      <Link
                        to={`/integrations/business-systems/${businessSystem.id}`}
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
      </PageContent>

      <DeleteConfirmationDialog
        open={deletion.item !== null}
        pending={deletion.pending}
        title={
          deletion.item
            ? t("businessSystem.delete.title", { name: deletion.item.name })
            : ""
        }
        description={t("businessSystem.delete.description")}
        onOpenChange={(open) => {
          if (!open) deletion.select(null)
        }}
        onConfirm={() => void deletion.confirm()}
      />
    </div>
  )
}
