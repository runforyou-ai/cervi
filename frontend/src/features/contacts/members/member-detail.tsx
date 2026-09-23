/** 企业成员详情和字段级编辑。 */
import { useEffect, useMemo, useState, type KeyboardEvent } from "react"
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
import {
  DetailEditRow,
  ReadonlyDetailRow,
} from "@/components/form/detail-edit-row"
import { InlineEditField } from "@/components/form/inline-edit-field"
import { WorkStatusBadge } from "@/components/work-status"
import { Field, FieldDescription } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Switch } from "@/components/ui/switch"
import { AccountStatusEditRow } from "@/features/contacts/account-status-edit-row"
import { MemberAssistantsSection } from "@/features/contacts/assistants/member-assistants-section"
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

type EditingField = "name" | "email" | "role" | "accountStatus" | "maxServiceSessions" | "teams" | null

/** 把企业成员详情转换为编辑表单值。 */
function valuesFromUser(user: UserData): MemberFormValues {
  return {
    displayName: user.displayName,
    email: user.email,
    password: "",
    roleId: user.role.id,
    teamIds: user.teams.map((team) => team.id),
    handlesCustomers: user.handlesCustomers,
    maxServiceSessions: String(user.maxServiceSessions),
  }
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
          nameInvalid: t("members.validation.nameInvalid"),
          emailRequired: t("members.validation.emailRequired"),
          emailInvalid: t("members.validation.emailInvalid"),
          passwordRequired: t("members.validation.passwordRequired"),
          passwordTooShort: t("members.validation.passwordTooShort"),
          passwordTooLong: t("members.validation.passwordTooLong"),
          roleRequired: t("members.validation.roleRequired"),
          maxServiceSessionsInvalid: t("members.validation.maxServiceSessionsInvalid"),
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

  /** 保存成员字段：先在可编辑状态下校验；请求发出后即使详情关闭也写完并刷新缓存，只在仍打开时更新编辑状态。失败时文本输入保留草稿和编辑态，传入 draft 的即选即存控件恢复为已保存的值。 */
  async function saveMember(
    changed?: MemberFormValues,
    closeAfterSave = true,
  ) {
    const draft = changed ?? form.getValues()
    const userID = user.id
    if (saveState.isSaving()) return
    if (!(await form.trigger(undefined, { shouldFocus: true }))) return
    const current = valuesFromUser(user)
    if (
      draft.displayName === current.displayName &&
      draft.email === current.email &&
      draft.roleId === current.roleId &&
      draft.handlesCustomers === current.handlesCustomers &&
      draft.maxServiceSessions.trim() === current.maxServiceSessions &&
      sameIDs(draft.teamIds, current.teamIds)
    ) {
      setEditing(null)
      return
    }

    const request = saveState.begin()
    if (request === null) return
    try {
      const saved = await updateUser(userID, {
        displayName: draft.displayName,
        email: draft.email,
        roleId: draft.roleId,
        teamIds: draft.teamIds,
        handlesCustomers: draft.handlesCustomers,
        maxServiceSessions: Number(draft.maxServiceSessions),
      })
      onSaved(saved)
      if (closeAfterSave && saveState.isCurrent(request)) setEditing(null)
    } catch (error) {
      if (changed && saveState.isCurrent(request)) form.reset(valuesFromUser(user))
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        // 只在详情仍是这次请求所属的成员时关闭，迟到的结果不影响之后打开的详情。
        if (saveState.isCurrent(request)) onNotFound()
        return
      }
      console.warn("保存企业成员失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "displayName",
              "email",
              "roleId",
              "teamIds",
              "handlesCustomers",
              "maxServiceSessions",
            ])
          : t("members.form.networkError"),
      )
    } finally {
      saveState.finish(request)
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

  /** 文本字段共用的就地编辑属性：失焦或回车保存，Esc 放弃修改。 */
  function textFieldProps(field: "name" | "email" | "maxServiceSessions") {
    return {
      control: form.control,
      empty,
      disabled: saving,
      editing: editing === field,
      editEnabled: editing === null && !saving,
      onEditingChange: (next: boolean) => {
        if (next) startEditing(field)
      },
      onCommit: () => void saveMember(),
      onCancel: cancelEdit,
    }
  }

  return (
    <div className="flex flex-col gap-7">
      <section>
        <h3 className="mb-2 text-sm font-medium">{t("detail.basicInformation")}</h3>
        <div className="divide-y">
          <InlineEditField
            {...textFieldProps("name")}
            name="displayName"
            label={t("columns.name")}
            required
          />

          <InlineEditField
            {...textFieldProps("email")}
            name="email"
            type="email"
            label={t("columns.email")}
            required
          />

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

          <ReadonlyDetailRow label={t("columns.handlesCustomers")}>
            <Switch
              id="member-handles-customers"
              checked={user.handlesCustomers}
              disabled={saving || editing !== null}
              onCheckedChange={(checked) =>
                void saveMember(
                  { ...form.getValues(), handlesCustomers: checked },
                  false,
                )
              }
            />
          </ReadonlyDetailRow>

          {user.handlesCustomers ? (
            <InlineEditField
              {...textFieldProps("maxServiceSessions")}
              name="maxServiceSessions"
              type="number"
              inputMode="numeric"
              min={1}
              step={1}
              label={t("columns.maxServiceSessions")}
              required
            />
          ) : null}

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

      <MemberAssistantsSection userId={user.id} />

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
