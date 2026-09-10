/** 企业成员详情和字段级编辑。 */
import { useEffect, useMemo, useState, type KeyboardEvent, type ReactNode } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  UserStatus,
  deactivateUser,
  isApiError,
  isNotFoundApiError,
  reactivateUser,
  updateUser,
  type UserData,
  type RoleData,
  type Team,
} from "@/api"
import { DetailEditRow } from "@/components/form/detail-edit-row"
import { WorkStatusBadge } from "@/components/work-status"
import { Field, FieldDescription } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { AccountStatusEditRow } from "@/features/contacts/account-status-edit-row"
import { TeamCheckboxOptions } from "@/features/contacts/team-checkbox-options"
import {
  createMemberSchema,
  type MemberFormValues,
} from "@/features/contacts/members/member-schema"
import { roleDisplayName } from "@/lib/role-labels"
import { useDateTime } from "@/hooks/use-date-time"
import { sameIDs, useImmediateSave } from "@/hooks/use-immediate-save"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type EditingField = "name" | "email" | "role" | "accountStatus" | "teams" | null

/** 把企业成员详情转换为编辑表单值。 */
function valuesFromUser(user: UserData): MemberFormValues {
  return {
    displayName: user.displayName,
    email: user.email,
    password: "",
    roleId: user.role.id,
    teamIds: user.teams.map((team) => team.id),
  }
}

/** 显示企业成员只读字段。 */
function ReadonlyDetailRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-start gap-3 px-2 py-3 text-sm">
      <div className="w-28 shrink-0 text-muted-foreground">{label}</div>
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  )
}

/** 按字段展示和编辑企业成员详情。 */
export function MemberDetailView({
  user,
  teams,
  roles,
  workStatus,
  onSaved,
  onNotFound,
}: {
  user: UserData
  teams: Team[]
  roles: RoleData[]
  workStatus: UserData["workStatus"]
  onSaved: (user: UserData) => void
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
      createMemberSchema(
        {
          nameRequired: t("members.validation.nameRequired"),
          emailRequired: t("members.validation.emailRequired"),
          emailInvalid: t("members.validation.emailInvalid"),
          passwordRequired: t("members.validation.passwordRequired"),
          passwordTooShort: t("members.validation.passwordTooShort"),
          passwordTooLong: t("members.validation.passwordTooLong"),
          roleRequired: t("members.validation.roleRequired"),
        },
        true,
      ),
    [t],
  )
  const form = useForm<MemberFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: valuesFromUser(user),
  })
  useEffect(() => {
    form.reset(valuesFromUser(user))
  }, [form, user])

  /** 放弃尚未提交的修改并退出编辑。 */
  function cancelEdit() {
    form.reset(valuesFromUser(user))
    setEditing(null)
  }

  /** 开始编辑指定成员字段。 */
  function startEditing(field: Exclude<EditingField, null>) {
    form.reset(valuesFromUser(user))
    setEditing(field)
  }

  /** 保存成员字段。 */
  async function saveMember(
    draft: MemberFormValues = form.getValues(),
    closeAfterSave = true,
  ) {
    const userID = user.id
    const request = saveState.begin()
    if (request === null) return
    const valid = await form.trigger()
    if (!saveState.isCurrent(request)) return
    if (!valid) {
      saveState.finish(request)
      return
    }
    const current = valuesFromUser(user)
    if (
      draft.displayName === current.displayName &&
      draft.email === current.email &&
      draft.roleId === current.roleId &&
      sameIDs(draft.teamIds, current.teamIds)
    ) {
      setEditing(null)
      saveState.finish(request)
      return
    }

    try {
      const saved = await updateUser(userID, {
        displayName: draft.displayName,
        email: draft.email,
        roleId: draft.roleId,
        teamIds: draft.teamIds,
      })
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
      console.warn("保存企业成员失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["displayName", "email", "roleId", "teamIds"])
          : t("members.form.networkError"),
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

  /** 处理选择字段快捷键。 */
  function handleSelectKeyDown(event: KeyboardEvent<HTMLSelectElement>) {
    if (event.key !== "Escape") return
    event.preventDefault()
    event.stopPropagation()
    cancelEdit()
  }

  const empty = <span className="text-muted-foreground">{t("detail.empty")}</span>

  return (
    <div className="flex flex-col gap-7">
      <section>
        <h3 className="mb-2 text-sm font-medium">{t("detail.basicInformation")}</h3>
        <div className="divide-y">
          <DetailEditRow
            label={t("columns.name")}
            value={user.displayName || empty}
            editing={editing === "name"}
            editEnabled={editing === null && !saving}
            required
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
                    void saveMember()
                  }}
                  onKeyDown={handleTextKeyDown}
                />
              )}
            />
          </DetailEditRow>

          <DetailEditRow
            label={t("columns.email")}
            value={user.email || empty}
            editing={editing === "email"}
            editEnabled={editing === null && !saving}
            required
            onEdit={() => startEditing("email")}
          >
            <Controller
              name="email"
              control={form.control}
              render={({ field }) => (
                <Input
                  {...field}
                  type="email"
                  autoFocus
                  disabled={saving}
                  onBlur={() => {
                    field.onBlur()
                    void saveMember()
                  }}
                  onKeyDown={handleTextKeyDown}
                />
              )}
            />
          </DetailEditRow>

          <DetailEditRow
            label={t("columns.role")}
            value={roleDisplayName(user.role, tCommon)}
            editing={editing === "role"}
            editEnabled={editing === null && !saving}
            required
            onEdit={() => startEditing("role")}
          >
            <Controller
              name="roleId"
              control={form.control}
              render={({ field }) => (
                <NativeSelect
                  {...field}
                  autoFocus
                  disabled={saving}
                  onChange={(event) => {
                    const roleId = event.target.value
                    field.onChange(roleId)
                    void saveMember({
                      ...form.getValues(),
                      roleId,
                    })
                  }}
                  onBlur={() => {
                    field.onBlur()
                    if (!saveState.isSaving()) cancelEdit()
                  }}
                  onKeyDown={handleSelectKeyDown}
                >
                  {roles.map((role) => (
                    <option key={role.id} value={role.id}>
                      {roleDisplayName(role, tCommon)}
                    </option>
                  ))}
                </NativeSelect>
              )}
            />
          </DetailEditRow>

          <AccountStatusEditRow
            status={user.status}
            editing={editing === "accountStatus"}
            editEnabled={editing === null && !saving}
            saveState={saveState}
            onEdit={() => startEditing("accountStatus")}
            onCancel={cancelEdit}
            onSave={(status) =>
              status === UserStatus.UserStatusInactive
                ? deactivateUser(user.id)
                : reactivateUser(user.id)
            }
            onSaved={onSaved}
            onNotFound={onNotFound}
            entityName="企业成员"
            entityId={user.id}
            errorMessage={t("members.status.error")}
          />

          <ReadonlyDetailRow label={t("columns.workStatus")}>
            <WorkStatusBadge status={workStatus} />
          </ReadonlyDetailRow>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("columns.teams")}</h3>
        <DetailEditRow
          label={t("columns.teams")}
          value={user.teams.map((team) => team.name).join("、") || empty}
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
                  <FieldDescription>{t("members.form.noTeams")}</FieldDescription>
                ) : (
                  <TeamCheckboxOptions
                    teams={teams}
                    value={field.value}
                    disabled={saving}
                    onChange={(teamIds) => {
                      field.onChange(teamIds)
                      void saveMember({ ...form.getValues(), teamIds }, false)
                    }}
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
                  />
                )}
              </Field>
            )}
          />
        </DetailEditRow>
      </section>

      <section>
        <h3 className="mb-3 text-sm font-medium">{t("detail.otherInformation")}</h3>
        <dl className="grid gap-4 px-2 text-sm">
          <div className="flex gap-3">
            <dt className="w-28 shrink-0 text-muted-foreground">
              {t("columns.createdAt")}
            </dt>
            <dd>{formatDateTime(user.createdAt)}</dd>
          </div>
        </dl>
      </section>
    </div>
  )
}
