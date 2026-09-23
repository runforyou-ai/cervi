/** 我的助理列表、在线状态与管理操作面板。 */
import { PlusIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  AssistantPresence,
  UserStatus,
  currentDevice,
  deactivateAssistant,
  isApiError,
  listAssistants,
  moveAssistant,
  pauseAssistant,
  reactivateAssistant,
  resumeAssistant,
  type AssistantData,
  type ChannelOption,
  type Team,
} from "@/api"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { ListToolbarSearch } from "@/components/list-toolbar"
import { ResourceRowIdentity } from "@/components/resource-row-identity"
import { ResourceTable } from "@/components/resource-table"
import { Button } from "@/components/ui/button"
import { useAccountStatusToggle } from "@/features/contacts/account-status-toggle"
import {
  assistantResourceKeys,
  useAssistantInvalidator,
} from "@/features/contacts/assistants/assistant-keys"
import { assistantPresenceLabel } from "@/features/contacts/assistants/assistant-presence"
import { ContactListSection } from "@/features/contacts/contact-list-section"
import { useContactSearch } from "@/features/contacts/use-contact-search"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 显示当前成员名下的助理并提供编辑、换电脑、暂停和启停操作。 */
export function AssistantsPanel({
  channels,
  teams,
}: {
  channels: ChannelOption[]
  teams: Team[]
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const invalidate = useAssistantInvalidator()
  const { setParameters, search, setSearch } = useContactSearch()
  const list = useResource(resourceKeys.assistants(), () => listAssistants())
  const { data: local } = useResource(resourceKeys.currentDevice(), () => currentDevice())
  const localDeviceID = local?.deviceId ?? ""
  const query = search.trim().toLowerCase()
  const assistants = (list.data?.assistants ?? []).filter((assistant) =>
    assistant.displayName.toLowerCase().includes(query),
  )
  const statusToggle = useAccountStatusToggle<AssistantData>({
    scope: "assistants",
    deactivate: deactivateAssistant,
    reactivate: reactivateAssistant,
    invalidateKeys: (assistant) => assistantResourceKeys(assistant.id),
    logLabel: "修改助理状态",
  })
  const move = useConfirmedAction<AssistantData>({
    action: (assistant) => moveAssistant(assistant.id, localDeviceID),
    invalidateKeys: (assistant) => assistantResourceKeys(assistant.id),
    successMessage: () => t("assistants.move.done"),
    errorMessage: () => t("assistants.move.error"),
    logLabel: "把助理换到这台电脑",
  })
  const pauseSave = useImmediateSave()

  /** 暂停或恢复助理，成功后刷新列表。 */
  async function togglePaused(assistant: AssistantData) {
    const request = pauseSave.begin()
    if (request === null) return
    const paused = assistant.presence === AssistantPresence.AssistantPresencePaused
    try {
      await (paused ? resumeAssistant(assistant.id) : pauseAssistant(assistant.id))
      void invalidate(assistant.id)
      if (pauseSave.isCurrent(request)) toast.success(t(paused ? "assistants.pause.resumed" : "assistants.pause.paused"))
    } catch (error) {
      if (!pauseSave.isCurrent(request) || recoverSession(error, navigate)) return
      console.warn("修改助理暂停状态失败", { assistant_id: assistant.id, error })
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("assistants.pause.error"))
    } finally {
      pauseSave.finish(request)
    }
  }

  const createLabel = localDeviceID ? t("add.assistant") : t("assistants.createOnDesktop")

  return (
    <>
      <ContactListSection
        title={t("scopes.assistants")}
        description={t("scopeDescriptions.assistants")}
        scope={{ scope: "assistants", teams, channels }}
        headerActions={
          localDeviceID ? (
            <Button variant="ghost" size="icon-sm" asChild>
              <Link to="/contacts/assistants/new" aria-label={createLabel} title={createLabel}>
                <PlusIcon />
              </Link>
            </Button>
          ) : (
            <Button variant="ghost" size="icon-sm" disabled aria-label={createLabel} title={createLabel}>
              <PlusIcon />
            </Button>
          )
        }
        toolbar={
          <ListToolbarSearch
            value={search}
            aria-label={t("search.assistants")}
            onChange={(event) => setSearch(event.target.value)}
          />
        }
        list={list}
        page={{ number: 1, size: assistants.length, total: assistants.length }}
        setParameters={setParameters}
      >
        <ResourceTable
          hideHeader
          columns={[
            {
              key: "name",
              header: t("columns.name"),
              cell: (assistant) => (
                <ResourceRowIdentity
                  avatar={{ imageURL: assistant.avatarUrl, name: assistant.displayName, fallback: "agent" }}
                  name={assistant.displayName}
                  secondary={assistant.device.name}
                  description={assistantPresenceLabel(assistant.presence, t)}
                />
              ),
            },
            {
              key: "model",
              header: t("columns.model"),
              cellClassName: "max-w-xs text-muted-foreground",
              cell: (assistant) => (
                <span className="block truncate">
                  {assistant.execution.managed.providerName} · {assistant.execution.managed.modelName}
                </span>
              ),
            },
          ]}
          rows={assistants}
          rowKey={(assistant) => assistant.id}
          empty={t("assistants.empty")}
          onRowActivate={(assistant) => navigate(`/contacts/assistants/${assistant.id}`)}
          rowActions={(assistant) => {
            const active = assistant.status === UserStatus.UserStatusActive
            const paused = assistant.presence === AssistantPresence.AssistantPresencePaused
            return [
              {
                key: "message",
                label: t("sendMessage"),
                disabled: !active,
                onSelect: () => navigate(`/inbox?scope=internal&target=${assistant.identityId}`),
              },
              {
                key: "edit",
                label: t("assistants.actions.edit"),
                onSelect: () => navigate(`/contacts/assistants/${assistant.id}`),
              },
              // 换到这台电脑只在桌面端出现。
              ...(localDeviceID
                ? [{
                    key: "move",
                    label: t("assistants.actions.move"),
                    disabled: assistant.device.id === localDeviceID && assistant.presence !== AssistantPresence.AssistantPresenceUnbound,
                    onSelect: () => move.select(assistant),
                  }]
                : []),
              {
                key: "pause",
                label: t(paused ? "assistants.actions.resume" : "assistants.actions.pause"),
                // 已禁用或未绑定电脑的助理不接收请求，暂停没有意义。
                disabled: !active || assistant.presence === AssistantPresence.AssistantPresenceUnbound || pauseSave.saving,
                onSelect: () => void togglePaused(assistant),
              },
              statusToggle.rowAction(assistant),
            ]
          }}
        />
      </ContactListSection>

      <ConfirmationDialog
        {...move.dialog}
        title={t("assistants.move.title", { name: move.item?.displayName ?? "" })}
        description={t("assistants.move.description")}
        pendingLabel={t("assistants.move.saving")}
      />
      <ConfirmationDialog {...statusToggle.dialog} />
    </>
  )
}
