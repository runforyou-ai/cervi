/** 个人设置中的设备列表。 */
import { useState } from "react"
import { LaptopIcon } from "lucide-react"
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
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ResourceContent } from "@/components/resource-content"
import { ResourceListFrame } from "@/components/resource-list"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { StatusBadge } from "@/components/status-badge"
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
    retrying,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.devices(), () => listDevices(), {
    staleTime: 0,
    refetchOnWindowFocus: true,
  })
  const { data: local } = useResource(
    resourceKeys.currentDevice(),
    () => currentDevice(),
    { staleTime: 0, refetchOnWindowFocus: true },
  )
  const devices = data?.devices ?? []
  const showLoading = loading || (retrying && !data)

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
        <ResourceListFrame>
          <ResourceTable
            hideHeader
            columns={[
              {
                key: "device",
                header: t("devices.list.columns.name"),
                cellClassName: "min-w-0",
                cell: (device) => (
                  <ResourceRowIdentity
                    icon={LaptopIcon}
                    name={device.name}
                    secondary={t(`devices.platforms.${device.platform}`)}
                    badge={
                      device.id === local?.deviceId ? (
                        <StatusBadge variant="muted">{t("devices.list.current")}</StatusBadge>
                      ) : null
                    }
                    description={t("devices.list.registeredAt", {
                      time: formatDateTime(device.createdAt),
                    })}
                  />
                ),
              },
            ]}
            rows={devices}
            rowKey={(device) => device.id}
            empty={t("devices.list.empty")}
            rowActions={(device) => [
              {
                key: "revoke",
                label: t("devices.revoke.action"),
                destructive: true,
                separatorBefore: true,
                onSelect: () => setRevoking(device),
              },
            ]}
          />
        </ResourceListFrame>
      </ResourceContent>

      <ConfirmationDialog
        open={revoking !== null}
        pending={save.saving}
        title={t("devices.revoke.title", { name: revoking?.name ?? "" })}
        description={t("devices.revoke.description")}
        pendingLabel={t("devices.revoke.pending")}
        onOpenChange={(open) => {
          if (!open) setRevoking(null)
        }}
        onConfirm={() => void confirmRevoke()}
      />
    </>
  )
}
