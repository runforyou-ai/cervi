/** 新建和编辑企业成员表单。 */
import { useEffect, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  FilePurpose,
  RoleKind,
  createUser,
  isApiError,
  isNotFoundApiError,
  updateUser,
  type RoleData,
  type Team,
  type UserData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { SwitchCardField } from "@/components/form/switch-card-field"
import { ImagePicker } from "@/components/image-picker"
import { FormActions } from "@/components/form/form-actions"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useFormLifetime } from "@/hooks/use-form-lifetime"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { RoleSelectField } from "@/features/contacts/role-select-field"
import { TeamSelectField } from "@/features/contacts/team-select-field"
import {
  createMemberSchema,
  type MemberFormValues,
} from "@/features/settings/members/member-schema"

const errorFields = [
  "avatarFileId",
  "displayName",
  "email",
  "password",
  "roleId",
  "teamIds",
  "handlesCustomers",
  "maxServiceSessions",
]

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

/** 创建企业成员，或边改边存已有成员的头像、资料、角色、接待设置和所属团队。 */
export function MemberForm({
  user,
  teams,
  roles,
  onSaved,
  onCancel,
  onNotFound,
}: {
  user?: UserData
  teams: Team[]
  roles: RoleData[]
  onSaved: (user: UserData) => void
  onCancel?: () => void
  onNotFound?: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const editing = Boolean(user)
  const defaultRoleID =
    roles.find((role) => role.kind === RoleKind.RoleKindMember)?.id ??
    roles[0]?.id ??
    ""
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
        editing,
      ),
    [editing, t],
  )
  const form = useForm<MemberFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    // 编辑时离开字段即校验以便自动保存；新建时等提交再校验。
    mode: editing ? "onBlur" : "onSubmit",
    defaultValues: user
      ? valuesFromUser(user)
      : {
          displayName: "",
          email: "",
          password: "",
          roleId: defaultRoleID,
          teamIds: [],
          handlesCustomers: false,
          maxServiceSessions: "10",
        },
  })
  const displayName = useWatch({ control: form.control, name: "displayName" })
  const handlesCustomers = useWatch({ control: form.control, name: "handlesCustomers" })
  const avatar = usePendingImageUpload({
    purpose: FilePurpose.FilePurposeUserAvatar,
    onError: (error) => {
      console.warn("上传企业成员头像失败", error)
      if (!recoverSession(error, navigate)) toast.error(t("avatar.uploadError"))
    },
  })
  const { mounted, dirty, discarded } = useFormLifetime(
    form.formState.isDirty || avatar.pending !== null,
  )
  const { acceptSaved, markSaved, saveNow } = useAutoSave({
    form,
    schema,
    enabled: editing,
    save: update,
    discarded,
  })

  // 成员资料刷新时同步未修改的表单和自动保存基准，保留正在编辑的草稿。
  useEffect(() => {
    if (!user || dirty.current) return
    const values = valuesFromUser(user)
    form.reset(values)
    markSaved(values)
  }, [dirty, form, user])

  useEffect(() => {
    if (!user && !form.getValues("roleId") && defaultRoleID) {
      form.setValue("roleId", defaultRoleID)
    }
  }, [defaultRoleID, form, user])

  /** 展示成员保存失败。 */
  function reportError(error: unknown) {
    if (recoverSession(error, navigate)) return
    console.warn("保存企业成员失败", error)
    toast.error(
      isApiError(error)
        ? apiErrorMessage(error, errorFields)
        : t("members.form.networkError"),
    )
  }

  /** 上传待保存的头像后创建企业成员。 */
  async function create(values: MemberFormValues) {
    let uploadingAvatar = false
    try {
      uploadingAvatar = Boolean(avatar.pending && !avatar.pending.fileID)
      const avatarFileId = await avatar.ensureUploaded()
      uploadingAvatar = false
      const created = await createUser({
        displayName: values.displayName,
        email: values.email,
        password: values.password,
        roleId: values.roleId,
        teamIds: values.teamIds,
        handlesCustomers: values.handlesCustomers,
        maxServiceSessions: Number(values.maxServiceSessions),
        avatarFileId,
      })
      dirty.current = false
      toast.success(t("members.form.created"))
      onSaved(created)
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar) return
      reportError(error)
    }
  }

  /** 上传待保存的头像后保存已有成员，返回是否保存成功。 */
  async function update(values: MemberFormValues) {
    if (!user) return false
    let avatarFileId: string
    try {
      avatarFileId = await avatar.ensureUploaded()
    } catch {
      // 图片上传错误由上传回调展示。
      return false
    }
    try {
      const saved = await updateUser(user.id, {
        displayName: values.displayName,
        email: values.email,
        roleId: values.roleId,
        teamIds: values.teamIds,
        handlesCustomers: values.handlesCustomers,
        maxServiceSessions: Number(values.maxServiceSessions),
        avatarFileId,
      })
      onSaved(saved)
      if (!mounted.current) return true
      dirty.current = !acceptSaved(values, valuesFromUser(saved))
      avatar.clear(avatarFileId)
      return true
    } catch (error) {
      // 成员已不存在时只在表单仍打开时返回列表。
      if (isNotFoundApiError(error)) {
        if (mounted.current) onNotFound?.()
        return false
      }
      reportError(error)
      return false
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form
      className="space-y-9"
      onSubmit={form.handleSubmit(async (values) => {
        if (editing) saveNow(true)
        else await create(values)
      })}
      noValidate
    >
      <FieldGroup className="gap-5">
        <Field>
          <FieldLabel>{t("avatar.label")}</FieldLabel>
          <ImagePicker
            imageURL={avatar.pending?.previewURL || user?.avatarUrl}
            name={displayName}
            fallback="person"
            label={t("avatar.choose")}
            className="rounded-full"
            avatarClassName="rounded-full"
            disabled={isSubmitting}
            loading={avatar.pending?.status === "uploading"}
            onSelect={(file) => {
              avatar.select(file)
              if (editing) saveNow(true)
            }}
          />
        </Field>
        <FormInputField
          name="displayName"
          control={form.control}
          label={t("members.form.name")}
          autoFocus={!editing}
        />
        <FormInputField
          name="email"
          control={form.control}
          label={t("members.form.email")}
          type="email"
        />
        {editing ? null : (
          <FormInputField
            name="password"
            control={form.control}
            label={t("members.form.password")}
            type="password"
          />
        )}
        <Controller
          name="roleId"
          control={form.control}
          render={({ field, fieldState }) => (
            <RoleSelectField
              {...field}
              id={field.name}
              aria-invalid={fieldState.invalid}
              roles={roles}
            />
          )}
        />
        <Controller
          name="handlesCustomers"
          control={form.control}
          render={({ field }) => (
            <SwitchCardField
              id={field.name}
              name={field.name}
              label={t("members.form.handlesCustomers")}
              description={t("members.form.handlesCustomersHelp")}
              checked={field.value}
              onBlur={field.onBlur}
              onCheckedChange={field.onChange}
              ref={field.ref}
            />
          )}
        />
        {handlesCustomers ? (
          <FormInputField
            name="maxServiceSessions"
            control={form.control}
            label={t("members.form.maxServiceSessions")}
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
          />
        ) : null}
        <Controller
          name="teamIds"
          control={form.control}
          render={({ field }) => (
            <TeamSelectField
              teams={teams}
              label={t("members.form.teams")}
              emptyMessage={t("members.form.noTeams")}
              value={field.value}
              onChange={field.onChange}
              onBlur={field.onBlur}
            />
          )}
        />
      </FieldGroup>
      <FormActions saving={isSubmitting} onCancel={onCancel} submit={!editing} />
    </form>
  )
}
