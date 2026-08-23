/** 展示并编辑 AI 员工详情。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  UserStatus,
  isApiError,
  isNotFoundApiError,
  updateAgent,
  type AgentData,
  type Team,
} from "@/api"
import { StatusBadge } from "@/components/status-badge"
import { Field, FieldDescription } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createAgentSchema,
  type AgentFormValues,
} from "@/features/contacts/agent-schema"
import { userStatusLabel } from "@/features/contacts/contact-labels"
import {
  DetailEditActions,
  DetailEditRow,
} from "@/features/contacts/detail-edit-row"
import { useDateTime } from "@/hooks/use-date-time"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type EditingField = "name" | "teams" | null

/** 把 AI 员工详情转换为编辑表单值。 */
function valuesFromAgent(agent: AgentData): AgentFormValues {
  return {
    displayName: agent.displayName,
    teamIds: agent.teams.map((team) => team.id),
  }
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
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [editing, setEditing] = useState<EditingField>(null)
  const [saving, setSaving] = useState(false)
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

  useEffect(() => {
    form.reset(valuesFromAgent(agent))
    setEditing(null)
  }, [agent, form])

  /** 取消当前字段编辑。 */
  function cancelEdit() {
    form.reset(valuesFromAgent(agent))
    setEditing(null)
  }

  /** 开始编辑指定 AI 员工字段。 */
  function startEditing(field: Exclude<EditingField, null>) {
    form.reset(valuesFromAgent(agent))
    setEditing(field)
  }

  const save = form.handleSubmit(async (values) => {
    setSaving(true)
    try {
      const saved = await updateAgent(agent.id, values)
      console.info("AI 员工已保存", { agent_id: saved.id })
      toast.success(t("agents.form.updated"))
      onSaved(saved)
    } catch (error) {
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
      setSaving(false)
    }
  })

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
            <Input {...form.register("displayName")} autoFocus />
            <DetailEditActions
              saving={saving}
              onSave={() => void save()}
              onCancel={cancelEdit}
            />
          </DetailEditRow>

          <div className="flex items-start gap-3 px-2 py-3 text-sm">
            <div className="w-28 shrink-0 text-muted-foreground">
              {t("columns.status")}
            </div>
            <div className="min-w-0 flex-1">
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
            </div>
          </div>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("columns.teams")}</h3>
        <DetailEditRow
          label={t("columns.teams")}
          value={agent.teams.map((team) => team.name).join("、") || empty}
          editing={editing === "teams"}
          editEnabled={editing === null && !saving}
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
                  <div className="grid gap-2 rounded-md border p-3 sm:grid-cols-2">
                    {teams.map((team) => (
                      <label
                        key={team.id}
                        className="flex items-center gap-2 text-sm"
                      >
                        <input
                          type="checkbox"
                          className="size-4 accent-primary"
                          checked={field.value.includes(team.id)}
                          onChange={(event) =>
                            field.onChange(
                              event.target.checked
                                ? [...field.value, team.id]
                                : field.value.filter((id) => id !== team.id),
                            )
                          }
                        />
                        <span>{team.name}</span>
                      </label>
                    ))}
                  </div>
                )}
              </Field>
            )}
          />
          <DetailEditActions
            saving={saving}
            onSave={() => void save()}
            onCancel={cancelEdit}
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
