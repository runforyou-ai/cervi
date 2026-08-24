/** 展示并编辑 AI 员工详情。 */
import {
  useEffect,
  useMemo,
  useRef,
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
  updateAgentWorkStatus,
  type AgentData,
  type Team,
} from "@/api"
import { StatusBadge } from "@/components/status-badge"
import { Field, FieldDescription } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import {
  agentWorkStatusSchema,
  createAgentSchema,
  type AgentWorkStatusFormValues,
  type AgentFormValues,
} from "@/features/contacts/agent-schema"
import {
  accountStatusSchema,
  type AccountStatusFormValues,
} from "@/features/contacts/account-status-schema"
import { userStatusLabel } from "@/features/contacts/contact-labels"
import { DetailEditRow } from "@/features/contacts/detail-edit-row"
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
  | null

const accountStatuses = [
  UserStatus.UserStatusActive,
  UserStatus.UserStatusInactive,
] as const

/** 把 AI 员工详情转换为编辑表单值。 */
function valuesFromAgent(agent: AgentData): AgentFormValues {
  return {
    displayName: agent.displayName,
    teamIds: agent.teams.map((team) => team.id),
  }
}

/** 按当前顺序判断两个团队编号列表是否一致。 */
function sameTeamIDs(left: string[], right: string[]) {
  return (
    left.length === right.length &&
    left.every((teamID, index) => teamID === right[index])
  )
}

/** 展示并逐字段编辑 AI 员工资料。 */
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
  const [saving, setSaving] = useState(false)
  const savingRef = useRef(false)
  const requestVersionRef = useRef(0)
  const previousAgentIDRef = useRef(agent.id)
  const currentAgentIDRef = useRef(agent.id)
  currentAgentIDRef.current = agent.id
  const schema = useMemo(
    () =>
      createAgentSchema({
        nameRequired: t("agents.validation.nameRequired"),
      }),
    [t],
  )
  const form = useForm<AgentFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: valuesFromAgent(agent),
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
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
    if (previousAgentIDRef.current !== agent.id) {
      requestVersionRef.current += 1
      savingRef.current = false
      setSaving(false)
      setEditing(null)
    }
    previousAgentIDRef.current = agent.id
  }, [accountStatusForm, agent, form, workStatusForm])

  useEffect(
    () => () => {
      requestVersionRef.current += 1
      savingRef.current = false
    },
    [],
  )

  /** 放弃尚未提交的修改并退出编辑。 */
  function cancelEdit() {
    form.reset(valuesFromAgent(agent))
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
    setEditing(null)
  }

  /** 开始编辑指定 AI 员工字段。 */
  function startEditing(field: Exclude<EditingField, null>) {
    form.reset(valuesFromAgent(agent))
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
    setEditing(field)
  }

  /** 标记一次保存开始并返回用于忽略过期结果的版本号。 */
  function beginSaving() {
    if (savingRef.current) return null
    savingRef.current = true
    setSaving(true)
    requestVersionRef.current += 1
    return requestVersionRef.current
  }

  /** 判断保存结果是否仍属于当前详情。 */
  function isCurrentRequest(
    version: number,
    agentID = currentAgentIDRef.current,
  ) {
    return (
      requestVersionRef.current === version &&
      currentAgentIDRef.current === agentID
    )
  }

  /** 结束仍有效的保存状态。 */
  function finishSaving(
    version: number,
    agentID = currentAgentIDRef.current,
  ) {
    if (!isCurrentRequest(version, agentID)) return
    savingRef.current = false
    setSaving(false)
  }

  /** 保存失败时恢复服务端返回的 AI 员工资料。 */
  function rollbackEdit() {
    form.reset(valuesFromAgent(agent))
    accountStatusForm.reset({ status: agent.status })
    workStatusForm.reset({ workStatus: agent.workStatus })
    setEditing(null)
  }

  /** 保存当前字段修改，并按字段交互决定是否退出编辑。 */
  async function saveAgent(
    draft: AgentFormValues = form.getValues(),
    closeAfterSave = true,
  ) {
    if (savingRef.current) return
    const agentID = agent.id
    const requestVersion = beginSaving()
    if (requestVersion === null) return
    const valid = await form.trigger()
    if (!isCurrentRequest(requestVersion, agentID)) return
    if (!valid) {
      finishSaving(requestVersion, agentID)
      return
    }
    const parsed = schema.safeParse(draft)
    if (!parsed.success) {
      finishSaving(requestVersion, agentID)
      return
    }
    const current = valuesFromAgent(agent)
    if (
      parsed.data.displayName === current.displayName &&
      sameTeamIDs(parsed.data.teamIds, current.teamIds)
    ) {
      setEditing(null)
      finishSaving(requestVersion, agentID)
      return
    }

    try {
      const saved = await updateAgent(agentID, parsed.data)
      if (!isCurrentRequest(requestVersion, agentID)) return
      if (closeAfterSave) setEditing(null)
      onSaved(saved)
    } catch (error) {
      if (!isCurrentRequest(requestVersion, agentID)) return
      rollbackEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("保存 AI 员工失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["displayName", "teamIds"])
          : t("agents.form.networkError"),
      )
    } finally {
      finishSaving(requestVersion, agentID)
    }
  }

  /** 立即修改 AI 员工的账号状态。 */
  async function saveAccountStatus(
    status: AccountStatusFormValues["status"],
  ) {
    if (status === agent.status) {
      setEditing(null)
      return
    }
    const agentID = agent.id
    const requestVersion = beginSaving()
    if (requestVersion === null) return
    const valid = await accountStatusForm.trigger()
    if (!isCurrentRequest(requestVersion, agentID)) return
    if (!valid) {
      finishSaving(requestVersion, agentID)
      return
    }
    const parsed = accountStatusSchema.safeParse({ status })
    if (!parsed.success) {
      finishSaving(requestVersion, agentID)
      return
    }

    try {
      const saved =
        parsed.data.status === UserStatus.UserStatusInactive
          ? await deactivateAgent(agentID)
          : await reactivateAgent(agentID)
      if (!isCurrentRequest(requestVersion, agentID)) return
      setEditing(null)
      onSaved(saved)
    } catch (error) {
      if (!isCurrentRequest(requestVersion, agentID)) return
      rollbackEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("修改 AI 员工账号状态失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error)
          : t("agents.status.error"),
      )
    } finally {
      finishSaving(requestVersion, agentID)
    }
  }

  /** 立即保存 AI 员工的工作状态。 */
  async function saveWorkStatus(
    draft: AgentWorkStatusFormValues = workStatusForm.getValues(),
  ) {
    if (savingRef.current) return
    const agentID = agent.id
    const requestVersion = beginSaving()
    if (requestVersion === null) return
    const valid = await workStatusForm.trigger()
    if (!isCurrentRequest(requestVersion, agentID)) return
    if (!valid) {
      finishSaving(requestVersion, agentID)
      return
    }
    const parsed = agentWorkStatusSchema.safeParse(draft)
    if (!parsed.success) {
      finishSaving(requestVersion, agentID)
      return
    }

    try {
      const saved = await updateAgentWorkStatus(agentID, parsed.data)
      if (!isCurrentRequest(requestVersion, agentID)) return
      setEditing(null)
      onSaved(saved)
    } catch (error) {
      if (!isCurrentRequest(requestVersion, agentID)) return
      rollbackEdit()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn("修改 AI 员工工作状态失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["workStatus"])
          : t("agents.workStatus.error"),
      )
    } finally {
      finishSaving(requestVersion, agentID)
    }
  }

  /** 处理文本字段的回车保存和退出编辑。 */
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

  /** 允许选择字段通过 Escape 放弃本次编辑。 */
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
                    if (!savingRef.current) cancelEdit()
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
                    if (!savingRef.current) cancelEdit()
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
                      if (savingRef.current) {
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
                          className="size-4 accent-primary aria-disabled:cursor-wait aria-disabled:opacity-60"
                          aria-disabled={saving}
                          checked={field.value.includes(team.id)}
                          onClick={(event) => {
                            if (savingRef.current) event.preventDefault()
                          }}
                          onChange={(event) => {
                            if (savingRef.current) return
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
