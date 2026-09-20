/** 个人设置中的设备列表。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  currentDevice,
  isApiError,
  listDevices,
  revokeDevice,
  type DeviceData,
} from "@/api"
import { ResourceContent } from "@/components/resource-content"
import { ResourceTable } from "@/components/resource-table"
import { StatusBadge } from "@/components/status-badge"
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
import { DropdownMenuItem } from "@/components/ui/dropdown-menu"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 展示当前用户已注册的设备，并可撤销其中一台。 */
export function DeviceListPage() {
  const { t } = useTranslation(["settings", "common"])
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const invalidate = useResourceInvalidator()
  const save = useImmediateSave()
  const [revoking, setRevoking] = useState<DeviceData | null>(null)
  const {
    data,
    loading,
    refreshing,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.devices(), () => listDevices(), {
    staleTime: 0,
    refetchOnWindowFocus: true,
  })
  const { data: local } = useResource(
    resourceKeys.currentDevice(),
    () => currentDevice(),
    { staleTime: 0 },
  )
  const devices = data?.devices ?? []
  const showLoading = loading || (Boolean(loadError) && !data && refreshing)

  /** 撤销选中的设备，离开页面后仅更新共享缓存。 */
  async function confirmRevoke() {
    if (!revoking) return
    const request = save.begin()
    if (request === null) return
    try {
      await revokeDevice(revoking.id)
      void invalidate(resourceKeys.devices())
      void invalidate(resourceKeys.currentDevice())
      if (!save.isCurrent(request)) return
      setRevoking(null)
      toast.success(t("devices.revoke.success"))
    } catch (error) {
      if (!save.isCurrent(request) || recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("devices.revoke.error"))
    } finally {
      save.finish(request)
    }
  }

  return (
    <>
      <ResourceContent
        loading={showLoading}
        error={Boolean(loadError) && !data}
        errorMessage={t("devices.list.loadError")}
        onRetry={() => void refresh()}
      >
        <div className="overflow-hidden rounded-lg border bg-card">
          <ResourceTable
            columns={[
              {
                key: "name",
                header: t("devices.list.columns.name"),
                cellClassName: "font-medium",
                cell: (device) => (
                  <span className="flex items-center gap-2">
                    {device.name}
                    {device.id === local?.deviceId ? (
                      <StatusBadge variant="muted">{t("devices.list.current")}</StatusBadge>
                    ) : null}
                  </span>
                ),
              },
              {
                key: "platform",
                header: t("devices.list.columns.platform"),
                cell: (device) => t(`devices.platforms.${device.platform}`),
              },
              {
                key: "runtimeVersion",
                header: t("devices.list.columns.runtimeVersion"),
                cellClassName: "text-muted-foreground",
                cell: (device) => device.runtimeVersion || "—",
              },
              {
                key: "createdAt",
                header: t("devices.list.columns.createdAt"),
                cellClassName: "text-muted-foreground",
                cell: (device) => formatDateTime(device.createdAt),
              },
            ]}
            rows={devices}
            rowKey={(device) => device.id}
            empty={t("devices.list.empty")}
            actions={(device) => ({
              menu: (
                <DropdownMenuItem destructive onSelect={() => setRevoking(device)}>
                  {t("devices.revoke.action")}
                </DropdownMenuItem>
              ),
            })}
          />
        </div>
      </ResourceContent>

      <AlertDialog
        open={revoking !== null}
        onOpenChange={(open) => {
          if (!open && !save.saving) setRevoking(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("devices.revoke.title", { name: revoking?.name ?? "" })}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("devices.revoke.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={save.saving}>
              {t("common:actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={save.saving}
              onClick={(event) => {
                event.preventDefault()
                void confirmRevoke()
              }}
            >
              {save.saving ? t("devices.revoke.pending") : t("common:actions.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
