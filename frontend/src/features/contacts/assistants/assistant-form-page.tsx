/** 助理独立创建页与编辑页。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"

import { currentDevice, getAssistant, isNotFoundApiError, listDevices } from "@/api"
import { PageBackButton } from "@/components/page-back-button"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import {
  AssistantCreateForm,
  AssistantEditForm,
} from "@/features/contacts/assistants/assistant-form"
import { useAssistantInvalidator } from "@/features/contacts/assistants/assistant-keys"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

const listPath = "/contacts/assistants"

/** 在本机创建助理，或编辑当前成员名下的助理。 */
export function AssistantFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("contacts")
  const { assistantId = "" } = useParams()
  const navigate = useNavigate()
  const invalidate = useAssistantInvalidator()
  const local = useResource(resourceKeys.currentDevice(), () => currentDevice(), { enabled: mode === "create" })
  const devices = useResource(resourceKeys.devices(), () => listDevices(), { enabled: mode === "create" })
  const detail = useResource(resourceKeys.assistant(assistantId), () => getAssistant(assistantId), {
    enabled: mode === "edit",
  })
  const localDeviceID = local.data?.deviceId ?? ""
  const deviceName = devices.data?.devices.find((device) => device.id === localDeviceID)?.name ?? ""

  // 助理不存在或不属于本人时返回列表。
  useEffect(() => {
    if (mode !== "edit" || !isNotFoundApiError(detail.error)) return
    console.warn("助理不存在", { assistant_id: assistantId })
    navigate(listPath, { replace: true })
  }, [assistantId, detail.error, mode, navigate])

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={
          mode === "create"
            ? t("assistants.create")
            : detail.data
              ? t("assistants.edit", { name: detail.data.assistant.displayName })
              : t("assistants.editTitle")
        }
        description={t(mode === "create" ? "assistants.createDescription" : "assistants.editDescription")}
      >
        {mode === "edit" ? <PageBackButton to={listPath} /> : null}
      </PageHeader>
      <PageContent variant="form">
        <ResourceContent
          resources={mode === "edit" ? [detail] : [local, devices]}
          errorMessage={t("assistants.loadError")}
        >
          {mode === "create" ? (
            localDeviceID ? (
              <AssistantCreateForm
                deviceID={localDeviceID}
                deviceName={deviceName}
                onCancel={() => navigate(listPath)}
                onSaved={() => navigate(listPath, { replace: true })}
              />
            ) : (
              <p className="text-sm text-muted-foreground">{t("assistants.createOnDesktop")}</p>
            )
          ) : detail.data ? (
            <AssistantEditForm
              key={detail.data.assistant.id}
              detail={detail.data}
              onSaved={() => void invalidate(assistantId)}
            />
          ) : null}
        </ResourceContent>
      </PageContent>
    </div>
  )
}
