/** 修改密码表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { changePassword, isApiError } from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createChangePasswordSchema,
  type ChangePasswordFormValues,
} from "@/features/settings/change-password-schema"
import { apiErrorMessage } from "@/lib/form-errors"

/** 修改当前用户的登录密码。 */
export function ChangePasswordForm() {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const schema = useMemo(() => createChangePasswordSchema(t), [t])
  const form = useForm<ChangePasswordFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      currentPassword: "",
      newPassword: "",
      confirmPassword: "",
    },
  })

  /** 提交密码修改。 */
  async function save(values: ChangePasswordFormValues) {
    try {
      await changePassword({
        currentPassword: values.currentPassword,
        newPassword: values.newPassword,
      })
      form.reset()
      console.info("密码修改成功")
      toast.success(t("password.saveSuccess"))
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("修改密码失败", error)
      if (isApiError(error)) {
        let fieldError = false
        for (const name of ["currentPassword", "newPassword"] as const) {
          const message = error.fields[name]
          if (!message) {
            continue
          }
          const shouldFocus = !fieldError
          fieldError = true
          form.setError(name, { message }, { shouldFocus })
        }
        if (fieldError) {
          return
        }
        toast.error(apiErrorMessage(error))
        return
      }
      toast.error(t("password.saveError"))
    }
  }

  const { isDirty, isSubmitting } = form.formState

  return (
    <form
      className="mt-6 w-full max-w-xl"
      aria-label={t("password.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup className="gap-6">
        <Controller
          name="currentPassword"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("password.currentPassword")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                type="password"
                autoComplete="current-password"
                aria-invalid={fieldState.invalid}
                required
                autoFocus
              />
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />
        <Controller
          name="newPassword"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("password.newPassword")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                type="password"
                autoComplete="new-password"
                aria-invalid={fieldState.invalid}
                required
              />
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />
        <Controller
          name="confirmPassword"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("password.confirmPassword")}
              </FieldLabel>
              <Input
                {...field}
                id={field.name}
                type="password"
                autoComplete="new-password"
                aria-invalid={fieldState.invalid}
                required
              />
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />
        <div>
          <Button type="submit" disabled={!isDirty || isSubmitting}>
            {isSubmitting ? (
              <LoaderCircleIcon className="animate-spin" />
            ) : null}
            {isSubmitting ? t("password.saving") : t("password.save")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
