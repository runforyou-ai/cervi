/** AI 员工运行配置表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, updateAgentExecution, type AgentData } from "@/api"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
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
import { recoverSession } from "@/lib/session-navigation"

/** 整体保存模型、指令、知识库和 MCP 服务绑定。 */
export function AgentExecutionForm({
  agent,
  onSaved,
  onCancel,
}: {
  agent: AgentData
  onSaved: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation(["contacts", "common"])
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createAgentExecutionSchema({
        modelRequired: t("agents.validation.modelRequired"),
        instructionRequired: t("agents.validation.instructionRequired"),
        instructionTooLong: t("agents.validation.instructionTooLong"),
      }),
    [t],
  )
  const managed = agent.execution.managed
  const form = useForm<AgentExecutionFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
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
  const { mounted, dirty } = useFormLifetime(form.formState.isDirty)

  // 外部删除服务后只同步未编辑的绑定，保留其他表单草稿。
  useEffect(() => {
    if (!form.getFieldState("mcpServerIds").isDirty) {
      form.resetField("mcpServerIds", { defaultValue: agent.execution.mcpServerIds })
    }
  }, [agent.execution.mcpServerIds, form])

  /** 提交当前运行配置并生成一个生效版本。 */
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
      if (!mounted.current) return
      dirty.current = false
      form.reset({ ...values, mcpServerIds: saved.execution.mcpServerIds })
      console.info("AI 员工运行配置已保存", {
        agent_id: saved.id,
        revision_id: saved.execution.revisionId,
      })
      toast.success(t("agents.form.saved"))
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
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
    }
  }

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
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
              <FieldLabel htmlFor="agent-execution-instruction" required>
                {t("agents.execution.instruction")}
              </FieldLabel>
              <Textarea
                {...field}
                id="agent-execution-instruction"
                rows={10}
                required
                aria-invalid={fieldState.invalid}
                disabled={form.formState.isSubmitting}
              />
            </Field>
          )}
        />
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
      <div className="flex items-center gap-2">
        <Button
          type="submit"
          disabled={form.formState.isSubmitting}
        >
          {t(
            form.formState.isSubmitting
              ? "common:actions.saving"
              : "common:actions.save",
          )}
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={form.formState.isSubmitting}
          onClick={onCancel}
        >
          {t("common:actions.cancel")}
        </Button>
      </div>
    </form>
  )
}
