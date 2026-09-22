/** AI 员工运行配置表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, updateAgentExecution, type AgentData } from "@/api"
import { AgentBehaviorSummary } from "@/components/agent-behavior-summary"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { AgentKnowledgeField } from "@/features/contacts/agents/agent-knowledge-field"
import { AgentMCPField } from "@/features/contacts/agents/agent-mcp-field"
import { AgentModelField } from "@/features/contacts/agents/agent-model-field"
import {
  agentModelSelection,
  parseAgentModelSelection,
} from "@/features/contacts/agents/agent-model-selection"
import {
  createAgentExecutionSchema,
  type AgentExecutionFormValues,
} from "@/features/contacts/agents/agent-schema"
import { useFormLifetime } from "@/hooks/use-form-lifetime"
import { apiErrorMessage } from "@/lib/form-errors"
import { useAutoSave } from "@/hooks/use-auto-save"
import { recoverSession } from "@/lib/session-navigation"

/** 整体保存模型、指令、知识库和 MCP 服务绑定。 */
export function AgentExecutionForm({
  agent,
  onSaved,
}: {
  agent: AgentData
  onSaved: () => void
}) {
  const { t } = useTranslation(["contacts", "common"])
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAgentExecutionSchema({
        modelRequired: t("agents.validation.modelRequired"),
        instructionTooLong: t("agents.validation.instructionTooLong"),
      }),
    [t],
  )
  const managed = agent.execution.managed
  const form = useForm<AgentExecutionFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      modelSelection: agentModelSelection(
        managed.providerId,
        managed.modelIdentifier,
      ),
      systemInstruction: managed.systemInstruction,
      knowledgeBaseIds: managed.knowledgeBaseIds,
      mcpServerIds: agent.execution.mcpServerIds,
    },
  })
  const { mounted, dirty, discarded } = useFormLifetime(form.formState.isDirty)

  // 外部删除服务后只同步未编辑的绑定，保留其他表单草稿。
  useEffect(() => {
    if (!form.getFieldState("mcpServerIds").isDirty) {
      form.resetField("mcpServerIds", { defaultValue: agent.execution.mcpServerIds })
    }
  }, [agent.execution.mcpServerIds, form])

  /** 提交当前运行配置并生成一个生效版本。 */
  const { markSaved } = useAutoSave({ form, schema, save: submit, discarded })

  async function submit(values: AgentExecutionFormValues) {
    try {
      const saved = await updateAgentExecution(agent.id, {
        mode: agent.execution.mode,
        mcpServerIds: values.mcpServerIds,
        managed: {
          ...parseAgentModelSelection(values.modelSelection),
          systemInstruction: values.systemInstruction,
          knowledgeBaseIds: values.knowledgeBaseIds,
        },
      })
      onSaved()
      if (!mounted.current) return true
      dirty.current = false
      const next = { ...values, mcpServerIds: saved.execution.mcpServerIds }
      form.reset(next)
      markSaved(next)
      return true
    } catch (error) {
      // 离开页面后提交的改动失败时同样提示。
      if (recoverSession(error, navigate)) return false
      console.warn("保存 AI 员工运行配置失败", { agent_id: agent.id, error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "execution",
              "providerId",
              "modelIdentifier",
              "systemInstruction",
              "knowledgeBaseIds",
              "mcpServerIds",
            ])
          : t("agents.execution.saveError"),
      )
      return false
    }
  }

  return (
    <form onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup>
        <AgentModelField
          control={form.control}
          name="modelSelection"
          disabled={form.formState.isSubmitting}
        />
        <Controller
          name="systemInstruction"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="agent-execution-instruction">
                {t("agents.execution.instruction")}
              </FieldLabel>
              <Textarea
                {...field}
                id="agent-execution-instruction"
                rows={10}
                aria-invalid={fieldState.invalid}
                disabled={form.formState.isSubmitting}
              />
              <FieldDescription>
                {t("agents.execution.instructionHelp")}
              </FieldDescription>
            </Field>
          )}
        />
        <Field>
          <FieldLabel>{t("agents.execution.behavior")}</FieldLabel>
          <AgentBehaviorSummary behavior={agent.behavior} />
        </Field>
        <Controller
          name="knowledgeBaseIds"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel>{t("agents.execution.knowledgeBases")}</FieldLabel>
              <AgentKnowledgeField
                value={field.value}
                onChange={field.onChange}
                disabled={form.formState.isSubmitting}
              />
            </Field>
          )}
        />
        <Controller
          name="mcpServerIds"
          control={form.control}
          render={({ field }) => (
            <Field>
              <FieldLabel>{t("agents.mcp.services")}</FieldLabel>
              <AgentMCPField value={field.value} onChange={field.onChange} disabled={form.formState.isSubmitting} />
            </Field>
          )}
        />
      </FieldGroup>
    </form>
  )
}
