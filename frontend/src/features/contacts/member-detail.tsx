/** 企业成员详情和字段级编辑。 */
import { useEffect, useMemo, useState, type ReactNode } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  UserStatus,
  isApiError,
  isNotFoundApiError,
  updateUser,
  type UserData,
  type RoleData,
  type Team,
} from "@/api"
import { Field, FieldDescription } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { StatusBadge } from "@/components/status-badge"
import { userStatusLabel } from "@/features/contacts/contact-labels"
import {
  DetailEditActions,
  DetailEditRow,
} from "@/features/contacts/detail-edit-row"
import {
  createMemberSchema,
  type MemberFormValues,
} from "@/features/contacts/member-schema"
import { WorkStatusBadge } from "@/features/users/work-status"
import { useDateTime } from "@/hooks/use-date-time"
import { apiErrorMessage } from "@/lib/form-errors"
import { useSessionRecovery } from "@/lib/session-navigation"
import { roleDisplayName } from "@/features/roles/role-labels"

type EditingField = "name" | "email" | "role" | "teams" | null

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
function ReadonlyDetailRow({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
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
  const recoverSession = useSessionRecovery()
  const { formatDateTime } = useDateTime()
  const [editing, setEditing] = useState<EditingField>(null)
  const [saving, setSaving] = useState(false)
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
    setEditing(null)
  }, [form, user])

  /** 取消当前字段编辑。 */
  function cancelEdit() {
    form.reset(valuesFromUser(user))
    setEditing(null)
  }

  /** 开始编辑指定成员字段。 */
  function startEditing(field: Exclude<EditingField, null>) {
    form.reset(valuesFromUser(user))
    setEditing(field)
  }

  const save = form.handleSubmit(async (values) => {
    setSaving(true)
    try {
      const saved = await updateUser(user.id, {
        displayName: values.displayName,
        email: values.email,
        roleId: values.roleId,
        teamIds: values.teamIds,
      })
      toast.success(t("members.form.updated"))
      onSaved(saved)
    } catch (error) {
      if (recoverSession(error)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
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
            ])
          : t("members.form.networkError"),
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
            value={user.displayName || empty}
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

          <DetailEditRow
            label={t("columns.email")}
            value={user.email || empty}
            editing={editing === "email"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("email")}
          >
            <Input {...form.register("email")} type="email" autoFocus />
            <DetailEditActions
              saving={saving}
              onSave={() => void save()}
              onCancel={cancelEdit}
            />
          </DetailEditRow>

          <DetailEditRow
            label={t("columns.role")}
            value={roleDisplayName(user.role, tCommon)}
            editing={editing === "role"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("role")}
          >
            <NativeSelect {...form.register("roleId")} autoFocus>
              {roles.map((role) => (
                <option key={role.id} value={role.id}>
                  {roleDisplayName(role, tCommon)}
                </option>
              ))}
            </NativeSelect>
            <DetailEditActions
              saving={saving}
              onSave={() => void save()}
              onCancel={cancelEdit}
            />
          </DetailEditRow>

          <ReadonlyDetailRow label={t("columns.status")}>
            <StatusBadge
              showDot={false}
              variant={
                user.status === UserStatus.UserStatusActive
                  ? "success"
                  : "muted"
              }
            >
              {userStatusLabel(user.status, t)}
            </StatusBadge>
          </ReadonlyDetailRow>

          <ReadonlyDetailRow label={t("columns.workStatus")}>
            <WorkStatusBadge status={workStatus} showDot={false} />
          </ReadonlyDetailRow>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("columns.teams")}</h3>
        <DetailEditRow
          label={t("columns.teams")}
          value={
            user.teams.map((team) => team.name).join("、") || empty
          }
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
                  <FieldDescription>
                    {t("members.form.noTeams")}
                  </FieldDescription>
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
            <dd>{formatDateTime(user.createdAt)}</dd>
          </div>
        </dl>
      </section>
    </div>
  )
}
