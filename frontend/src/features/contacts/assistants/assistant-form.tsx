/** 助理新建与编辑表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, type Control } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  FilePurpose,
  createAssistant,
  isApiError,
  updateAssistant,
  type AssistantData,
  type AssistantDetailData,
} from "@/api"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { ImagePicker } from "@/components/image-picker"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { AgentKnowledgeField } from "@/components/agent-fields/agent-knowledge-field"
import { AgentMCPField } from "@/components/agent-fields/agent-mcp-field"
import { AgentModelField } from "@/components/agent-fields/agent-model-field"
import {
  agentModelSelection,
  parseAgentModelSelection,
} from "@/lib/agent-model-selection"
import { useAssistantInvalidator } from "@/features/contacts/assistants/assistant-keys"
import {
  createAssistantSchema,
  type AssistantFormValues,
} from "@/features/contacts/assistants/assistant-schema"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useFormLifetime } from "@/hooks/use-form-lifetime"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const assistantErrorFields = ["displayName", "providerId", "modelIdentifier", "systemInstruction", "knowledgeBaseIds", "mcpServerIds"]

/** 创建助理表单的校验规则。 */
function useAssistantSchema() {
  const { t } = useTranslation("contacts")
  return useMemo(
    () =>
      createAssistantSchema({
        nameRequired: t("assistants.validation.nameRequired"),
        nameInvalid: t("assistants.validation.nameInvalid"),
        modelRequired: t("assistants.validation.modelRequired"),
        instructionTooLong: t("assistants.validation.instructionTooLong"),
      }),
    [t],
  )
}

/** 待保存头像的上传状态，上传失败时提示。 */
function useAssistantAvatar() {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  return usePendingImageUpload({
    purpose: FilePurpose.FilePurposeAgentAvatar,
    onError: (error) => {
      console.warn("上传助理头像失败", error)
      if (!recoverSession(error, navigate)) toast.error(t("avatar.uploadError"))
    },
  })
}

/** 把表单值转换为助理的资料、执行配置与企业 MCP 服务输入。 */
function assistantInput(values: AssistantFormValues, avatarFileId: string) {
  return {
    displayName: values.displayName,
    avatarFileId,
    execution: {
      ...parseAgentModelSelection(values.modelSelection),
      systemInstruction: values.systemInstruction,
      knowledgeBaseIds: values.knowledgeBaseIds,
    },
    mcpServerIds: values.mcpServerIds,
  }
}

/** 在本机创建助理，执行电脑固定为当前电脑。 */
export function AssistantCreateForm({
  deviceID,
  deviceName,
  onSaved,
  onCancel,
}: {
  deviceID: string
  deviceName: string
  onSaved: (assistant: AssistantData) => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const invalidate = useAssistantInvalidator()
  const schema = useAssistantSchema()
  const form = useForm<AssistantFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { displayName: "", modelSelection: "", systemInstruction: "", knowledgeBaseIds: [], mcpServerIds: [] },
  })
  const avatar = useAssistantAvatar()
  const { mounted, dirty } = useFormLifetime(form.formState.isDirty || avatar.pending !== null)

  /** 上传待保存的头像后创建助理。 */
  async function submit(values: AssistantFormValues) {
    let uploadingAvatar = false
    try {
      uploadingAvatar = Boolean(avatar.pending && !avatar.pending.fileID)
      const avatarFileId = await avatar.ensureUploaded()
      uploadingAvatar = false
      const created = await createAssistant({ ...assistantInput(values, avatarFileId), deviceId: deviceID })
      void invalidate()
      if (!mounted.current) return
      toast.success(t("assistants.form.created"))
      dirty.current = false
      form.reset(values)
      avatar.clear()
      onSaved(created)
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar) return
      if (!mounted.current || recoverSession(error, navigate)) return
      console.warn("创建助理失败", { error })
      toast.error(isApiError(error) ? apiErrorMessage(error, assistantErrorFields) : t("assistants.networkError"))
    }
  }

  return (
    <form className="w-full space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <AssistantFields
        control={form.control}
        disabled={form.formState.isSubmitting}
        avatarURL={avatar.pending?.previewURL}
        avatarLoading={avatar.pending?.status === "uploading"}
        onAvatarSelect={avatar.select}
        deviceName={deviceName}
        autoFocus
      />
      <FormActions saving={form.formState.isSubmitting} onCancel={onCancel} />
    </form>
  )
}

/** 编辑当前成员名下的助理，改动自动保存为新的配置版本。 */
export function AssistantEditForm({
  detail,
  onSaved,
}: {
  detail: AssistantDetailData
  onSaved: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const schema = useAssistantSchema()
  const { assistant, execution } = detail
  const values = useMemo<AssistantFormValues>(
    () => ({
      displayName: assistant.displayName,
      modelSelection: agentModelSelection(execution.managed.providerId, execution.managed.modelIdentifier),
      systemInstruction: execution.managed.systemInstruction,
      knowledgeBaseIds: execution.managed.knowledgeBaseIds,
      mcpServerIds: execution.mcpServerIds,
    }),
    [assistant.displayName, execution],
  )
  const form = useForm<AssistantFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: values,
  })
  const avatar = useAssistantAvatar()
  const { mounted, dirty, discarded } = useFormLifetime(form.formState.isDirty || avatar.pending !== null)

  // 详情刷新时同步未修改的表单，保留正在编辑的草稿。
  useEffect(() => {
    if (!dirty.current) form.reset(values)
  }, [values, dirty, form])

  const { acceptSaved, saveNow } = useAutoSave({ form, schema, save: submit, discarded })

  /** 提交资料、执行配置与待保存的头像。 */
  async function submit(next: AssistantFormValues) {
    let uploadingAvatar = false
    try {
      uploadingAvatar = Boolean(avatar.pending && !avatar.pending.fileID)
      const avatarFileId = await avatar.ensureUploaded()
      uploadingAvatar = false
      await updateAssistant(assistant.id, assistantInput(next, avatarFileId))
      onSaved()
      if (!mounted.current) return true
      dirty.current = !acceptSaved(next)
      avatar.clear(avatarFileId)
      return true
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar || recoverSession(error, navigate)) return false
      console.warn("保存助理失败", { assistant_id: assistant.id, error })
      toast.error(isApiError(error) ? apiErrorMessage(error, assistantErrorFields) : t("assistants.networkError"))
      return false
    }
  }

  return (
    <form onSubmit={form.handleSubmit(() => saveNow(true))} noValidate>
      <AssistantFields
        control={form.control}
        disabled={form.formState.isSubmitting}
        avatarURL={avatar.pending?.previewURL || assistant.avatarUrl}
        avatarLoading={avatar.pending?.status === "uploading"}
        onAvatarSelect={(file) => {
          avatar.select(file)
          // 头像不在表单值中，选择后立即排队保存。
          saveNow(true)
        }}
        deviceName={assistant.device.name}
      />
    </form>
  )
}

/** 渲染助理的头像、名称、对话模型、指令、知识库、企业 MCP 服务与只读的执行电脑。 */
function AssistantFields({
  control,
  disabled,
  avatarURL,
  avatarLoading,
  onAvatarSelect,
  deviceName,
  autoFocus = false,
}: {
  control: Control<AssistantFormValues>
  disabled: boolean
  avatarURL?: string
  avatarLoading: boolean
  onAvatarSelect: (file: File) => void
  deviceName: string
  autoFocus?: boolean
}) {
  const { t } = useTranslation(["contacts", "agents"])
  return (
    <FieldGroup>
      <Field>
        <FieldLabel>{t("avatar.label")}</FieldLabel>
        <ImagePicker
          imageURL={avatarURL}
          fallback="agent"
          label={t("avatar.choose")}
          disabled={disabled}
          loading={avatarLoading}
          onSelect={onAvatarSelect}
        />
      </Field>
      <FormInputField
        name="displayName"
        id="assistant-name"
        control={control}
        label={t("assistants.form.name")}
        autoFocus={autoFocus}
        disabled={disabled}
      />
      <AgentModelField control={control} name="modelSelection" disabled={disabled} />
      <Controller
        name="systemInstruction"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor="assistant-instruction">{t("assistants.form.instruction")}</FieldLabel>
            <Textarea
              {...field}
              id="assistant-instruction"
              rows={6}
              disabled={disabled}
              aria-invalid={fieldState.invalid}
            />
          </Field>
        )}
      />
      <Controller
        name="knowledgeBaseIds"
        control={control}
        render={({ field }) => (
          <Field>
            <FieldLabel>{t("agents:execution.knowledgeBases")}</FieldLabel>
            <AgentKnowledgeField value={field.value} onChange={field.onChange} disabled={disabled} />
          </Field>
        )}
      />
      <Controller
        name="mcpServerIds"
        control={control}
        render={({ field }) => (
          <Field>
            <FieldLabel>{t("agents:mcp.services")}</FieldLabel>
            <AgentMCPField
              value={field.value}
              onChange={field.onChange}
              disabled={disabled}
              allowCustomerScoped={false}
            />
            <FieldDescription>{t("assistants.form.mcpHelp")}</FieldDescription>
          </Field>
        )}
      />
      <Field>
        <FieldLabel htmlFor="assistant-device">{t("assistants.form.device")}</FieldLabel>
        <Input id="assistant-device" value={deviceName} readOnly disabled />
        <FieldDescription>{t("assistants.form.deviceHelp")}</FieldDescription>
      </Field>
    </FieldGroup>
  )
}
