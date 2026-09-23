/** 新建企业成员表单。 */
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
  type RoleData,
  type Team,
  type UserData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { SwitchCardField } from "@/components/form/switch-card-field"
import { ImagePicker } from "@/components/image-picker"
import { FormActions } from "@/components/form/form-actions"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { RoleSelectField } from "@/features/contacts/role-select-field"
import { TeamSelectField } from "@/features/contacts/team-select-field"
import {
  createMemberSchema,
  type MemberFormValues,
} from "@/features/settings/members/member-schema"

/** 创建企业成员，可同时设置头像。 */
export function MemberForm({
  teams,
  roles,
  onSaved,
  onCancel,
}: {
  teams: Team[]
  roles: RoleData[]
  onSaved: (user: UserData) => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
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
        false,
      ),
    [t],
  )
  const form = useForm<MemberFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
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
  useEffect(() => {
    if (!form.getValues("roleId") && defaultRoleID) {
      form.setValue("roleId", defaultRoleID)
    }
  }, [defaultRoleID, form])

  /** 上传待保存的头像后提交企业成员表单。 */
  async function submit(values: MemberFormValues) {
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
      toast.success(t("members.form.created"))
      onSaved(created)
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar) return
      if (recoverSession(error, navigate)) return
      console.warn("保存企业成员失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "displayName",
              "email",
              "password",
              "roleId",
              "teamIds",
              "maxServiceSessions",
            ])
          : t("members.form.networkError"),
      )
    }
  }

  return (
    <form
      className="space-y-9"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <FieldGroup className="gap-5">
        <Field>
          <FieldLabel>{t("avatar.label")}</FieldLabel>
          <ImagePicker
            imageURL={avatar.pending?.previewURL}
            name={displayName}
            fallback="person"
            label={t("avatar.choose")}
            className="rounded-full"
            avatarClassName="rounded-full text-2xl"
            disabled={form.formState.isSubmitting}
            loading={avatar.pending?.status === "uploading"}
            onSelect={avatar.select}
          />
        </Field>
        <FormInputField
          name="displayName"
          control={form.control}
          label={t("members.form.name")}
          autoFocus
        />
        <FormInputField
          name="email"
          control={form.control}
          label={t("members.form.email")}
          type="email"
        />
        <FormInputField
          name="password"
          control={form.control}
          label={t("members.form.password")}
          type="password"
        />
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
      <FormActions saving={form.formState.isSubmitting} onCancel={onCancel} />
    </form>
  )
}
