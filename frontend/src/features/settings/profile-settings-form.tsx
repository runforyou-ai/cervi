/** 个人资料设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { isApiError, recoverSession, updateProfile, type User } from "@/api"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createProfileSettingsSchema,
  type ProfileSettingsFormValues,
} from "@/features/settings/profile-settings-schema"
import { apiErrorMessage } from "@/lib/form-errors"

/** 修改当前用户的姓名和邮箱。 */
export function ProfileSettingsForm({
  user,
  onUpdated,
}: {
  user: User
  onUpdated: (user: User) => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const schema = useMemo(() => createProfileSettingsSchema(t), [t])
  const form = useForm<ProfileSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: user.displayName,
      email: user.email,
    },
  })

  /** 保存个人资料并同步工作台中的当前用户。 */
  async function save(values: ProfileSettingsFormValues) {
    try {
      const updated = await updateProfile(values)
      form.reset({
        displayName: updated.displayName,
        email: updated.email,
      })
      onUpdated(updated)
      console.info("个人资料已保存", { user_id: updated.id })
      toast.success(t("profile.saveSuccess"))
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("保存个人资料失败", error)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["displayName", "email"]))
        return
      }
      toast.error(t("profile.saveError"))
    }
  }

  const { isDirty, isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-xl"
      aria-label={t("profile.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        <Controller
          name="displayName"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("profile.displayName")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                autoComplete="name"
                aria-invalid={fieldState.invalid}
                required
                autoFocus
              />
            </Field>
          )}
        />
        <Controller
          name="email"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("profile.email")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                type="email"
                autoComplete="email"
                aria-invalid={fieldState.invalid}
                required
              />
            </Field>
          )}
        />
        <div>
          <Button type="submit" disabled={!isDirty || isSubmitting}>
            {isSubmitting ? (
              <LoaderCircleIcon className="animate-spin" />
            ) : null}
            {isSubmitting ? t("profile.saving") : t("profile.save")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
