/** 助理单聊与助理聊天草稿的工作区设置浮层。 */
import { FolderIcon, LoaderCircleIcon } from "lucide-react"
import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  addLocalWorkspace,
  clearConversationAssistantWorkspace,
  currentDevice,
  isApiError,
  listAssistants,
  listConversationAssistantWorkspaces,
  listDeviceWorkspaces,
  setConversationAssistantWorkspace,
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
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 在助理单聊的会话头展示工作区入口：在助理绑定的电脑上可选择或添加工作区，其他桌面端、Web 与移动端展示当前工作区并可清除；助理未绑定电脑时不展示。 */
export function ConversationAssistantWorkspace({
  conversationID,
  assistantIdentityID,
}: {
  conversationID: string
  assistantIdentityID: string
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [pending, setPending] = useState(false)
  const { data, error, refresh } = useResource(
    resourceKeys.conversationAssistantWorkspaces(conversationID),
    () => listConversationAssistantWorkspaces(conversationID),
  )
  const entry = data?.find((item) => item.assistantIdentityId === assistantIdentityID)
  const mobile = resolveAppPlatform() === "mobile"
  const { localDeviceID, workspaces } = useLocalWorkspaces()
  // 只有助理绑定的电脑能选择本机目录作为工作区。
  const onBoundDevice = entry !== undefined && entry.deviceId === localDeviceID

  /** 执行一次工作区变更，成功后重读，失败时提示。 */
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

  /** 选择本机目录注册为工作区并立即指定给助理，用户取消选择时不做变更。 */
  async function addWorkspace() {
    await change(async () => {
      const workspace = await addLocalWorkspace()
      if (!workspace.id) return
      void invalidate(resourceKeys.deviceWorkspaces(localDeviceID))
      await setConversationAssistantWorkspace(conversationID, assistantIdentityID, { workspaceId: workspace.id })
    }, t("assistantWorkspaceAddError"))
  }

  // 助理未绑定电脑时没有工作区记录，不展示入口。
  if (data && !entry) return null

  return (
    <WorkspacePopover
      workspaceLabel={entry?.workspaceLabel ?? ""}
      selectable={onBoundDevice}
      pending={pending}
    >
      {error && !data ? (
        <p className="text-sm text-muted-foreground">{t("assistantWorkspaceLoadError")}</p>
      ) : null}
      {entry && onBoundDevice ? (
        <>
          <label className="grid gap-1.5">
            <span className="text-xs text-muted-foreground">{t("assistantWorkspaceLabel")}</span>
            <NativeSelect
              value={entry.workspaceId}
              disabled={pending}
              onChange={(event) => {
                const workspaceID = event.target.value
                void change(
                  () => workspaceID
                    ? setConversationAssistantWorkspace(conversationID, assistantIdentityID, { workspaceId: workspaceID })
                    : clearConversationAssistantWorkspace(conversationID, assistantIdentityID),
                  workspaceID ? t("assistantWorkspaceSetError") : t("assistantWorkspaceClearError"),
                )
              }}
            >
              <option value="">{t("assistantWorkspaceNone")}</option>
              {workspaces.map((workspace) => (
                <option key={workspace.id} value={workspace.id}>{workspace.label}</option>
              ))}
            </NativeSelect>
          </label>
          <Button
            variant="outline"
            size="sm"
            className="justify-self-start"
            disabled={pending}
            onClick={() => void addWorkspace()}
          >
            {t("assistantWorkspaceAdd")}
          </Button>
        </>
      ) : null}
      {entry && !onBoundDevice ? (
        <>
          <p className="text-sm">
            {entry.workspaceId
              ? t("assistantWorkspaceOnDevice", { device: entry.deviceName, workspace: entry.workspaceLabel })
              : t("assistantWorkspaceUnset")}
          </p>
          <p className="text-sm text-muted-foreground">
            {t("assistantWorkspaceBoundDevice", { device: entry.deviceName })}
          </p>
          {entry.workspaceId ? (
            <Button
              variant="outline"
              size="sm"
              className={cn("justify-self-start", mobile && "min-h-11")}
              disabled={pending}
              onClick={() => void change(
                () => clearConversationAssistantWorkspace(conversationID, assistantIdentityID),
                t("assistantWorkspaceClearError"),
              )}
            >
              {t("assistantWorkspaceClear")}
            </Button>
          ) : null}
        </>
      ) : null}
    </WorkspacePopover>
  )
}

/** 在助理聊天草稿头部选择首条消息使用的工作区，首次发送时随消息一起指定；只有助理绑定的电脑可以选择。 */
export function DraftAssistantWorkspace({
  assistantIdentityID,
  workspaceID,
  onChange,
}: {
  assistantIdentityID: string
  workspaceID: string
  onChange: (workspaceID: string) => void
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [pending, setPending] = useState(false)
  const { localDeviceID, workspaces } = useLocalWorkspaces()
  const { data: owned } = useResource(resourceKeys.assistants(), () => listAssistants())
  const assistant = owned?.assistants.find((item) => item.identityId === assistantIdentityID)
  const onBoundDevice = assistant !== undefined && assistant.device.id === localDeviceID
  const selected = workspaces.find((workspace) => workspace.id === workspaceID)

  /** 选择本机目录注册为工作区并选为首条消息的工作区，用户取消选择时不做变更。 */
  async function addWorkspace() {
    if (pending) return
    setPending(true)
    try {
      const workspace = await addLocalWorkspace()
      if (!workspace.id) return
      void invalidate(resourceKeys.deviceWorkspaces(localDeviceID))
      onChange(workspace.id)
    } catch (error) {
      if (recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("assistantWorkspaceAddError"))
    } finally {
      setPending(false)
    }
  }

  return (
    <WorkspacePopover
      workspaceLabel={selected?.label ?? ""}
      selectable={onBoundDevice}
      pending={pending}
    >
      {onBoundDevice ? (
        <>
          <label className="grid gap-1.5">
            <span className="text-xs text-muted-foreground">{t("assistantWorkspaceLabel")}</span>
            <NativeSelect
              value={workspaceID}
              disabled={pending}
              onChange={(event) => onChange(event.target.value)}
            >
              <option value="">{t("assistantWorkspaceNone")}</option>
              {workspaces.map((workspace) => (
                <option key={workspace.id} value={workspace.id}>{workspace.label}</option>
              ))}
            </NativeSelect>
          </label>
          <Button
            variant="outline"
            size="sm"
            className="justify-self-start"
            disabled={pending}
            onClick={() => void addWorkspace()}
          >
            {t("assistantWorkspaceAdd")}
          </Button>
        </>
      ) : assistant ? (
        <p className="text-sm text-muted-foreground">
          {t("assistantWorkspaceBoundDevice", { device: assistant.device.name })}
        </p>
      ) : null}
    </WorkspacePopover>
  )
}

/** 读取本机设备编号及其已注册的工作区；移动端界面不作为助理绑定电脑，设备编号为空。 */
function useLocalWorkspaces() {
  const mobile = resolveAppPlatform() === "mobile"
  const { data: local } = useResource(resourceKeys.currentDevice(), () => currentDevice(), {
    enabled: !mobile,
  })
  const localDeviceID = mobile ? "" : (local?.deviceId ?? "")
  const { data: workspaceList } = useResource(
    resourceKeys.deviceWorkspaces(localDeviceID),
    () => listDeviceWorkspaces(localDeviceID),
    { enabled: localDeviceID !== "" },
  )
  return { localDeviceID, workspaces: workspaceList?.workspaces ?? [] }
}

/** 工作区入口按钮与设置浮层，已指定工作区时以圆点标记并在提示中显示工作区名称，不可选择时入口名称为「工作区」；移动端使用触控尺寸按钮，不显示悬停提示。 */
function WorkspacePopover({
  workspaceLabel,
  selectable,
  pending,
  children,
}: {
  workspaceLabel: string
  selectable: boolean
  pending: boolean
  children: ReactNode
}) {
  const { t } = useTranslation("inbox")
  const mobile = resolveAppPlatform() === "mobile"
  const label = workspaceLabel
    ? t("assistantWorkspaceActive", { workspace: workspaceLabel })
    : selectable
      ? t("assistantWorkspaceSelect")
      : t("assistantWorkspaceLabel")
  const trigger = (
    <PopoverTrigger asChild>
      <Button
        type="button"
        variant="ghost"
        size={mobile ? "icon-lg" : "icon-sm"}
        className={cn("relative shrink-0", !mobile && "text-muted-foreground")}
        aria-label={label}
      >
        {pending ? <LoaderCircleIcon className="animate-spin" /> : <FolderIcon />}
        {workspaceLabel ? (
          <span
            aria-hidden="true"
            className={cn(
              "absolute size-1.5 rounded-full bg-primary",
              mobile ? "top-2 right-2" : "top-1 right-1",
            )}
          />
        ) : null}
      </Button>
    </PopoverTrigger>
  )
  return (
    <Popover>
      {mobile ? (
        trigger
      ) : (
        <Tooltip>
          <TooltipTrigger asChild>{trigger}</TooltipTrigger>
          <TooltipContent>{label}</TooltipContent>
        </Tooltip>
      )}
      <PopoverContent align="end" className="grid w-72 gap-3">
        {children}
      </PopoverContent>
    </Popover>
  )
}
