/** 展示并编辑 AI 员工详情。 */
import {
  useEffect,
  useMemo,
  useState,
  type KeyboardEvent,
} from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  UserStatus,
  deactivateAgent,
  isApiError,
  isNotFoundApiError,
  reactivateAgent,
  updateAgent,
  updateAgentExecution,
  updateAgentWorkStatus,
  type AgentData,
  type Team,
} from "@/api"
import { StatusBadge } from "@/components/status-badge"
import { Field, FieldDescription } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import {
  agentWorkStatusSchema,
  createAgentManagedExecutionSchema,
  createAgentProfileSchema,
  type AgentManagedExecutionFormValues,
  type AgentProfileFormValues,
  type AgentWorkStatusFormValues,
} from "@/features/contacts/agent-schema"
import { AgentModelField } from "@/features/contacts/agent-model-field"
import {
  agentModelSelection,
  parseAgentModelSelection,
} from "@/features/contacts/agent-model-selection"
import {
  accountStatuses,
  accountStatusSchema,
  type AccountStatusFormValues,
} from "@/features/contacts/account-status-schema"
import { userStatusLabel } from "@/features/contacts/contact-labels"
import { DetailEditRow } from "@/features/contacts/detail-edit-row"
import {
  sameIDs,
  useImmediateSave,
} from "@/features/contacts/use-immediate-save"
import {
  selectableWorkStatuses,
  WorkStatusBadge,
  workStatusLabel,
} from "@/features/users/work-status"
import { useDateTime } from "@/hooks/use-date-time"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type EditingField =
  | "name"
  | "accountStatus"
  | "workStatus"
  | "teams"
  | "executionModel"
  | "systemInstruction"
  | null

/** 把 AI 员工详情转换为编辑表单值。 */
function valuesFromAgent(agent: AgentData): AgentProfileFormValues {
  return {
    displayName: agent.displayName,
    teamIds: agent.teams.map((team) => team.id),
  }
}

/** 把 AI 员工平台托管执行配置转换为表单值。 */
function managedExecutionValuesFromAgent(
  agent: AgentData,
): AgentManagedExecutionFormValues {
  return {
    modelSelection: agentModelSelection(
      agent.execution.managed.providerId,
      agent.execution.managed.modelIdentifier,
    ),
    systemInstruction: agent.execution.managed.systemInstruction,
  }
}

/** 展示并编辑 AI 员工。 */
export function AgentDetailView({
  agent,
  teams,
  onSaved,
  onNotFound,
}: {
  agent: AgentData
  teams: Team[]
  onSaved: (agent: AgentData) => void
  onNotFound: () => void
}) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [editing, setEditing] = useState<EditingField>(null)
  const saveState = useImmediateSave()
  const { saving } = saveState
  const schema = useMemo(
    () =>
      createAgentProfileSchema({
        nameRequired: t("agents.validation.nameRequired"),
      }),
    [t],
  )
  const managedExecutionSchema = useMemo(
    () =>
      createAgentManagedExecutionSchema({
        modelRequired: t("agents.validation.modelRequired"),
        instructionRequired: t("agents.validation.instructionRequired"),
        instructionTooLong: t("agents.validation.instructionTooLong"),
      }),
    [t],
  )
  const form = useForm<AgentProfileFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: valuesFromAgent(agent),
  })
  const managedExecutionForm = useForm<AgentManagedExecutionFormValues>({
    resolver: zodResolver(managedExecutionSchema),
    shouldUseNativeValidation: true,
    defaultValues: managedExecutionValuesFromAgent(agent),
  })
  const accountStatusForm = useForm<AccountStatusFormValues>({
    resolver: zodResolver(accountStatusSchema),
    shouldUseNativeValidation: true,
    defaultValues: { status: agent.status },
  })
  const workStatusForm = useForm<AgentWorkStatusFormValues>({
    resolver: zodResolver(agentWorkStatusSchema),
    shouldUseNativeValidation: true,
    defaultValues: { workStatus: agent.workStatus },
  })

  useEffect(() => {
    form.reset(valuesFromAgent(agent))
    managedExecutionForm.reset(managedExecutionValuesFromAgent(agent))
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
  }, [accountStatusForm, agent, form, managedExecutionForm, workStatusForm])

  /** 放弃尚未提交的修改并退出编辑。 */
  function cancelEdit() {
    form.reset(valuesFromAgent(agent))
    managedExecutionForm.reset(managedExecutionValuesFromAgent(agent))
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
    setEditing(null)
  }

  /** 开始编辑指定 AI 员工字段。 */
  function startEditing(field: Exclude<EditingField, null>) {
    form.reset(valuesFromAgent(agent))
    managedExecutionForm.reset(managedExecutionValuesFromAgent(agent))
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
    setEditing(field)
  }

  /** 保存 AI 员工资料。 */
  async function saveAgent(
    draft: AgentProfileFormValues = form.getValues(),
    closeAfterSave = true,
  ) {
    const agentID = agent.id
    const request = saveState.begin()
    if (request === null) return
    const valid = await form.trigger()
    if (!saveState.isCurrent(request)) return
    if (!valid) {
      saveState.finish(request)
      return
    }
    const current = valuesFromAgent(agent)
    if (
      draft.displayName === current.displayName &&
      sameIDs(draft.teamIds, current.teamIds)
    ) {
      setEditing(null)
      saveState.finish(request)
      return
    }

    try {
      const saved = await updateAgent(agentID, draft)
      if (!saveState.isCurrent(request)) return
      if (closeAfterSave) setEditing(null)
      onSaved(saved)
    } catch (error) {
      if (!saveState.isCurrent(request)) return
      cancelEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("保存 AI 员工失败", { agent_id: agentID, error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["displayName", "teamIds"])
          : t("agents.form.networkError"),
      )
    } finally {
      saveState.finish(request)
    }
  }

  /** 保存 AI 员工平台托管执行配置。 */
  async function saveManagedExecution(
    draft: AgentManagedExecutionFormValues = managedExecutionForm.getValues(),
  ) {
    const request = saveState.begin()
    if (request === null) return
    const valid = await managedExecutionForm.trigger()
    if (!saveState.isCurrent(request)) return
    if (!valid) {
      saveState.finish(request)
      return
    }
    const current = managedExecutionValuesFromAgent(agent)
    if (
      draft.modelSelection === current.modelSelection &&
      draft.systemInstruction === current.systemInstruction
    ) {
      setEditing(null)
      saveState.finish(request)
      return
    }
    try {
      const model = parseAgentModelSelection(draft.modelSelection)
      const saved = await updateAgentExecution(agent.id, {
        mode: agent.execution.mode,
        managed: {
          ...model,
          systemInstruction: draft.systemInstruction,
        },
      })
      if (!saveState.isCurrent(request)) return
      setEditing(null)
      onSaved(saved)
      console.info("AI 员工运行配置已保存", {
        agent_id: saved.id,
        execution_mode: saved.execution.mode,
        revision_id: saved.execution.revisionId,
        provider_id: saved.execution.managed.providerId,
        model_identifier: saved.execution.managed.modelIdentifier,
      })
    } catch (error) {
      if (!saveState.isCurrent(request)) return
      cancelEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("保存 AI 员工运行配置失败", {
        agent_id: agent.id,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "execution",
              "providerId",
              "modelIdentifier",
              "systemInstruction",
            ])
          : t("agents.capability.saveError"),
      )
    } finally {
      saveState.finish(request)
    }
  }

  /** 修改 AI 员工账号状态。 */
  async function saveAccountStatus(
    status: AccountStatusFormValues["status"],
  ) {
    if (status === agent.status) {
      setEditing(null)
      return
    }
    const agentID = agent.id
    const request = saveState.begin()
    if (request === null) return
    const valid = await accountStatusForm.trigger()
    if (!saveState.isCurrent(request)) return
    if (!valid) {
      saveState.finish(request)
      return
    }

    try {
      const saved =
        status === UserStatus.UserStatusInactive
          ? await deactivateAgent(agentID)
          : await reactivateAgent(agentID)
      if (!saveState.isCurrent(request)) return
      setEditing(null)
      onSaved(saved)
    } catch (error) {
      if (!saveState.isCurrent(request)) return
      cancelEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("修改 AI 员工账号状态失败", {
        agent_id: agentID,
        status,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error)
          : t("agents.status.error"),
      )
    } finally {
      saveState.finish(request)
    }
  }

  /** 保存 AI 员工工作状态。 */
  async function saveWorkStatus(
    draft: AgentWorkStatusFormValues = workStatusForm.getValues(),
  ) {
    const agentID = agent.id
    const request = saveState.begin()
    if (request === null) return
    const valid = await workStatusForm.trigger()
    if (!saveState.isCurrent(request)) return
    if (!valid) {
      saveState.finish(request)
      return
    }

    try {
      const saved = await updateAgentWorkStatus(agentID, draft)
      if (!saveState.isCurrent(request)) return
      setEditing(null)
      onSaved(saved)
    } catch (error) {
      if (!saveState.isCurrent(request)) return
      cancelEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("修改 AI 员工工作状态失败", {
        agent_id: agentID,
        work_status: draft.workStatus,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["workStatus"])
          : t("agents.workStatus.error"),
      )
    } finally {
      saveState.finish(request)
    }
  }

  /** 处理文本字段快捷键。 */
  function handleTextKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Escape") {
      event.preventDefault()
      event.stopPropagation()
      cancelEdit()
      return
    }
    if (event.key === "Enter") {
      event.preventDefault()
      event.currentTarget.blur()
    }
  }

  /** 处理多行文本字段快捷键。 */
  function handleTextareaKeyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.key !== "Escape") return
    event.preventDefault()
    event.stopPropagation()
    cancelEdit()
  }

  /** 处理选择字段快捷键。 */
  function handleSelectKeyDown(event: KeyboardEvent<HTMLSelectElement>) {
    if (event.key !== "Escape") return
    event.preventDefault()
    event.stopPropagation()
    cancelEdit()
  }

  const empty = (
    <span className="text-muted-foreground">{t("detail.empty")}</span>
  )

  return (
    <div className="flex flex-col gap-7">
      <section>
        <h3 className="mb-2 text-sm font-medium">
          {t("detail.basicInformation")}
        </h3>
        <div className="divide-y">
          <DetailEditRow
            label={t("columns.name")}
            value={agent.displayName || empty}
            editing={editing === "name"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("name")}
          >
            <Controller
              name="displayName"
              control={form.control}
              render={({ field }) => (
                <Input
                  {...field}
                  autoFocus
                  disabled={saving}
                  onBlur={() => {
                    field.onBlur()
                    void saveAgent()
                  }}
                  onKeyDown={handleTextKeyDown}
                />
              )}
            />
          </DetailEditRow>

          <DetailEditRow
            label={t("columns.accountStatus")}
            value={
              <StatusBadge
                showDot={false}
                variant={
                  agent.status === UserStatus.UserStatusActive
                    ? "success"
                    : "muted"
                }
              >
                {userStatusLabel(agent.status, t)}
              </StatusBadge>
            }
            editing={editing === "accountStatus"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("accountStatus")}
          >
            <Controller
              name="status"
              control={accountStatusForm.control}
              render={({ field }) => (
                <NativeSelect
                  {...field}
                  autoFocus
                  disabled={saving}
                  onChange={(event) => {
                    const status = event.target
                      .value as AccountStatusFormValues["status"]
                    field.onChange(status)
                    void saveAccountStatus(status)
                  }}
                  onBlur={() => {
                    field.onBlur()
                    if (!saveState.isSaving()) cancelEdit()
                  }}
                  onKeyDown={handleSelectKeyDown}
                >
                  {accountStatuses.map((status) => (
                    <option key={status} value={status}>
                      {userStatusLabel(status, t)}
                    </option>
                  ))}
                </NativeSelect>
              )}
            />
          </DetailEditRow>

          <DetailEditRow
            label={t("columns.workStatus")}
            value={<WorkStatusBadge status={agent.workStatus} />}
            editing={editing === "workStatus"}
            editEnabled={
              editing === null &&
              !saving &&
              agent.status === UserStatus.UserStatusActive
            }
            onEdit={() => startEditing("workStatus")}
          >
            <Controller
              name="workStatus"
              control={workStatusForm.control}
              render={({ field }) => (
                <NativeSelect
                  {...field}
                  disabled={saving}
                  autoFocus
                  onChange={(event) => {
                    const workStatus = event.target
                      .value as AgentWorkStatusFormValues["workStatus"]
                    field.onChange(workStatus)
                    void saveWorkStatus({ workStatus })
                  }}
                  onBlur={() => {
                    field.onBlur()
                    if (!saveState.isSaving()) cancelEdit()
                  }}
                  onKeyDown={handleSelectKeyDown}
                >
                  {selectableWorkStatuses.map((status) => (
                    <option key={status} value={status}>
                      {workStatusLabel(status, tCommon)}
                    </option>
                  ))}
                </NativeSelect>
              )}
            />
          </DetailEditRow>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">
          {t("agents.capability.title")}
        </h3>
        <div className="divide-y">
          <DetailEditRow
            label={t("agents.capability.model")}
            value={`${agent.execution.managed.providerName} · ${agent.execution.managed.modelName}`}
            editing={editing === "executionModel"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("executionModel")}
          >
            <AgentModelField
              control={managedExecutionForm.control}
              name="modelSelection"
              disabled={saving}
              hideLabel
              autoFocus
              onValueChange={(modelSelection) =>
                void saveManagedExecution({
                  ...managedExecutionForm.getValues(),
                  modelSelection,
                })
              }
              onBlur={() => {
                if (!saveState.isSaving()) cancelEdit()
              }}
              onKeyDown={handleSelectKeyDown}
            />
          </DetailEditRow>
          <DetailEditRow
            label={t("agents.capability.instruction")}
            value={
              <span className="whitespace-pre-wrap">
                {agent.execution.managed.systemInstruction}
              </span>
            }
            editing={editing === "systemInstruction"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("systemInstruction")}
          >
            <Controller
              name="systemInstruction"
              control={managedExecutionForm.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <Textarea
                    {...field}
                    rows={8}
                    required
                    autoFocus
                    disabled={saving}
                    aria-label={t("agents.capability.instruction")}
                    aria-invalid={fieldState.invalid}
                    onBlur={() => {
                      field.onBlur()
                      void saveManagedExecution()
                    }}
                    onKeyDown={handleTextareaKeyDown}
                  />
                </Field>
              )}
            />
          </DetailEditRow>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("columns.teams")}</h3>
        <DetailEditRow
          label={t("columns.teams")}
          value={agent.teams.map((team) => team.name).join("、") || empty}
          editing={editing === "teams"}
          editEnabled={editing === null && !saving && teams.length > 0}
          onEdit={() => startEditing("teams")}
        >
          <Controller
            name="teamIds"
            control={form.control}
            render={({ field }) => (
              <Field>
                {teams.length === 0 ? (
                  <FieldDescription>{t("agents.form.noTeams")}</FieldDescription>
                ) : (
                  <div
                    className="grid gap-2 rounded-md border p-3 sm:grid-cols-2"
                    onBlur={(event) => {
                      if (event.currentTarget.contains(event.relatedTarget)) {
                        return
                      }
                      if (saveState.isSaving()) {
                        setEditing(null)
                        return
                      }
                      cancelEdit()
                    }}
                    onKeyDown={(event) => {
                      if (event.key !== "Escape") return
                      event.preventDefault()
                      event.stopPropagation()
                      cancelEdit()
                    }}
                  >
                    {teams.map((team) => (
                      <label
                        key={team.id}
                        className="flex items-center gap-2 text-sm"
                      >
                        <input
                          type="checkbox"
                          className="size-4 accent-primary disabled:cursor-wait disabled:opacity-60"
                          disabled={saving}
                          checked={field.value.includes(team.id)}
                          onChange={(event) => {
                            const teamIds = event.target.checked
                              ? [...field.value, team.id]
                              : field.value.filter((id) => id !== team.id)
                            field.onChange(teamIds)
                            void saveAgent(
                              {
                                ...form.getValues(),
                                teamIds,
                              },
                              false,
                            )
                          }}
                        />
                        <span>{team.name}</span>
                      </label>
                    ))}
                  </div>
                )}
              </Field>
            )}
          />
        </DetailEditRow>
      </section>

      <section>
        <h3 className="mb-3 text-sm font-medium">
          {t("detail.otherInformation")}
        </h3>
        <dl className="grid gap-4 px-2 text-sm">
          <div className="flex gap-3">
            <dt className="w-28 shrink-0 text-muted-foreground">
              {t("columns.createdAt")}
            </dt>
            <dd>{formatDateTime(agent.createdAt)}</dd>
          </div>
        </dl>
      </section>
    </div>
  )
}
