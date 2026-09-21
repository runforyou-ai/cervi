/** 个人设置中的设备列表。 */
import { useRef, useState } from "react"
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
import { ListActionButton } from "@/components/list-action-button"
import { ResourceContent } from "@/components/resource-content"
import { ResourceListFrame } from "@/components/resource-list"
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
  const actionButtons = useRef(new Map<string, HTMLButtonElement>())
  // 确认框关闭后把焦点交回该行的撤销按钮；设备已撤销时该行不再存在，按默认行为处理。
  const returnFocusTo = useRef<string | null>(null)
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
    { staleTime: 0, refetchOnWindowFocus: true },
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
      returnFocusTo.current = null
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
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-muted-foreground">
                      <LaptopIcon className="size-4.5" aria-hidden="true" />
                    </span>
                    <span className="grid min-w-0 gap-0.5 leading-tight">
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="truncate">
                          <span className="font-medium">{device.name}</span>
                          <span aria-hidden="true" className="mx-1.5 text-muted-foreground">·</span>
                          <span className="text-muted-foreground">
                            {t(`devices.platforms.${device.platform}`)}
                          </span>
                        </span>
                        {device.id === local?.deviceId ? (
                          <StatusBadge variant="muted">{t("devices.list.current")}</StatusBadge>
                        ) : null}
                      </span>
                      <span className="truncate text-xs text-muted-foreground">
                        {t("devices.list.registeredAt", {
                          time: formatDateTime(device.createdAt),
                        })}
                      </span>
                    </span>
                  </div>
                ),
              },
            ]}
            rows={devices}
            rowKey={(device) => device.id}
            empty={t("devices.list.empty")}
            actions={(device) => ({
              primary: (
                <div className="ml-auto flex items-center gap-1">
                  <ListActionButton
                    tone="destructive"
                    ref={(node) => {
                      if (node) actionButtons.current.set(device.id, node)
                      else actionButtons.current.delete(device.id)
                    }}
                    onClick={() => {
                      returnFocusTo.current = device.id
                      setRevoking(device)
                    }}
                  >
                    {t("devices.revoke.action")}
                  </ListActionButton>
                </div>
              ),
            })}
          />
        </ResourceListFrame>
      </ResourceContent>

      <AlertDialog
        open={revoking !== null}
        onOpenChange={(open) => {
          if (!open && !save.saving) setRevoking(null)
        }}
      >
        <AlertDialogContent
          onCloseAutoFocus={(event) => {
            const trigger = returnFocusTo.current
              ? actionButtons.current.get(returnFocusTo.current)
              : undefined
            if (!trigger) return
            event.preventDefault()
            trigger.focus()
          }}
        >
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
