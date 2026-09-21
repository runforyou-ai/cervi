/** AI 单聊的本机执行设置浮层。 */
import { LaptopIcon, LoaderCircleIcon } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  addLocalWorkspace,
  bindConversationDevice,
  currentDevice,
  getConversationDeviceBinding,
  isApiError,
  listDeviceWorkspaces,
  unbindConversationDevice,
} from "@/api"
import { Button } from "@/components/ui/button"
import { NativeSelect } from "@/components/ui/native-select"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 在会话头展示本机执行入口：桌面端可选择本机工作区或添加目录，绑定在其他设备时展示绑定位置并可关闭。 */
export function ConversationDeviceBinding({ conversationID }: { conversationID: string }) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [pending, setPending] = useState(false)
  const bindingKey = resourceKeys.conversationDeviceBinding(conversationID)
  const { data: binding, error: bindingError, refresh } = useResource(
    bindingKey,
    () => getConversationDeviceBinding(conversationID),
  )
  const { data: local } = useResource(resourceKeys.currentDevice(), () => currentDevice())
  const localDeviceID = local?.deviceId ?? ""
  const { data: workspaceList } = useResource(
    resourceKeys.deviceWorkspaces(localDeviceID),
    () => listDeviceWorkspaces(localDeviceID),
    { enabled: localDeviceID !== "" },
  )
  const workspaces = workspaceList?.workspaces ?? []
  const bound = binding?.bound ?? false
  // 当前绑定落在本机工作区时由选择框表达，否则单独说明绑定位置。
  const boundLocally = bound && binding?.deviceId === localDeviceID
  const label = bound ? t("localRunActive") : t("localRun")

  /** 执行一次绑定变更，成功后重读绑定，失败时提示。 */
  async function change(action: () => Promise<unknown>, errorMessage: string) {
    if (pending) return
    setPending(true)
    try {
      await action()
      void refresh()
    } catch (error) {
      if (recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : errorMessage)
    } finally {
      setPending(false)
    }
  }

  /** 选择本机目录注册为工作区并立即绑定到当前会话，用户取消选择时不做变更。 */
  async function addWorkspace() {
    await change(async () => {
      const workspace = await addLocalWorkspace()
      if (!workspace.id) return
      void invalidate(resourceKeys.deviceWorkspaces(localDeviceID))
      await bindConversationDevice(conversationID, { workspaceId: workspace.id })
    }, t("localRunAddWorkspaceError"))
  }

  return (
    <Popover>
      <Tooltip>
        <TooltipTrigger asChild>
          <PopoverTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="relative shrink-0 text-muted-foreground"
              aria-label={label}
            >
              {pending ? <LoaderCircleIcon className="animate-spin" /> : <LaptopIcon />}
              {bound ? (
                <span
                  aria-hidden="true"
                  className="absolute top-1 right-1 size-1.5 rounded-full bg-primary"
                />
              ) : null}
            </Button>
          </PopoverTrigger>
        </TooltipTrigger>
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
      <PopoverContent align="end" className="grid w-72 gap-3">
        {bindingError && !binding ? (
          <p className="text-sm text-muted-foreground">{t("localRunLoadError")}</p>
        ) : null}
        {bound && !boundLocally && binding ? (
          <p className="text-sm">
            {t("localRunBoundTo", { device: binding.deviceName, workspace: binding.workspaceLabel })}
          </p>
        ) : null}
        {localDeviceID ? (
          <label className="grid gap-1.5">
            <span className="text-xs text-muted-foreground">{t("localRunWorkspace")}</span>
            <NativeSelect
              value={boundLocally ? binding?.workspaceId : ""}
              disabled={pending || !binding}
              onChange={(event) => {
                const workspaceID = event.target.value
                void change(
                  () => workspaceID
                    ? bindConversationDevice(conversationID, { workspaceId: workspaceID })
                    : unbindConversationDevice(conversationID),
                  workspaceID ? t("localRunBindError") : t("localRunUnbindError"),
                )
              }}
            >
              <option value="">{t("localRunOff")}</option>
              {workspaces.map((workspace) => (
                <option key={workspace.id} value={workspace.id}>{workspace.label}</option>
              ))}
            </NativeSelect>
          </label>
        ) : null}
        {localDeviceID ? (
          <Button
            variant="outline"
            size="sm"
            className="justify-self-start"
            disabled={pending}
            onClick={() => void addWorkspace()}
          >
            {t("localRunAddWorkspace")}
          </Button>
        ) : null}
        {bound && !boundLocally ? (
          <Button
            variant="outline"
            size="sm"
            className="justify-self-start"
            disabled={pending}
            onClick={() => void change(() => unbindConversationDevice(conversationID), t("localRunUnbindError"))}
          >
            {t("localRunTurnOff")}
          </Button>
        ) : null}
        {!localDeviceID && !bound ? (
          <p className="text-sm text-muted-foreground">{t("localRunDesktopOnly")}</p>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}
