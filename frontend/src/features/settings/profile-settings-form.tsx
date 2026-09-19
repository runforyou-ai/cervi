/** 个人资料设置表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  FilePurpose,
  isApiError,
  updateProfile,
  type CurrentUser,
} from "@/api"
import { resourceKeys } from "@/hooks/resource-keys"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { recoverSession } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createProfileSettingsSchema,
  type ProfileSettingsFormValues,
} from "@/features/settings/profile-settings-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { ImagePicker } from "@/components/image-picker"
import { resolveAppPlatform } from "@/platform/app-platform"

/** 修改当前用户的头像、姓名和邮箱，移动端使用触屏尺寸的整行保存按钮。 */
export function ProfileSettingsForm({ user }: { user: CurrentUser }) {
  const { t } = useTranslation(["settings", "common"])
  const mobile = resolveAppPlatform() === "mobile"
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const avatar = usePendingImageUpload({
    purpose: FilePurpose.FilePurposeUserAvatar,
    onError: (error) => {
      console.warn("上传用户头像失败", error)
      if (!recoverSession(error, navigate)) toast.error(t("profile.avatarUploadError"))
    },
  })
  const pendingAvatar = avatar.pending
  const schema = useMemo(() => createProfileSettingsSchema(t), [t])
  const form = useForm<ProfileSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: user.displayName,
      email: user.email,
    },
  })
  /** 保存个人资料并刷新当前身份。 */
  async function save(values: ProfileSettingsFormValues) {
    let uploadingAvatar = false
    try {
      uploadingAvatar = Boolean(pendingAvatar && !pendingAvatar.fileID)
      const avatarFileId = await avatar.ensureUploaded()
      uploadingAvatar = false
      const updated = await updateProfile({ ...values, avatarFileId })
      form.reset({
        displayName: updated.displayName,
        email: updated.email,
      })
      avatar.clear()
      void invalidate(resourceKeys.identity())
      toast.success(t("profile.saveSuccess"))
    } catch (error) {
      // 上传失败已由共享上传回调提示，保存只处理资料提交错误。
      if (uploadingAvatar) return
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

  const { isSubmitting } = form.formState

  return (
    <form
      className="w-full max-w-xl space-y-9"
      aria-label={t("profile.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        <Field>
          <FieldLabel>{t("profile.avatar")}</FieldLabel>
          <ImagePicker
            imageURL={pendingAvatar?.previewURL || user.avatarUrl}
            name={user.displayName}
            fallback="person"
            label={t("profile.avatarChoose")}
            className="rounded-full"
            avatarClassName="rounded-full text-2xl"
            disabled={isSubmitting}
            loading={pendingAvatar?.status === "uploading"}
            onSelect={avatar.select}
          />
        </Field>
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
                autoFocus={!mobile}
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
      </FieldGroup>
      <div>
        <Button
          type="submit"
          className={mobile ? "min-h-11 w-full" : undefined}
          disabled={isSubmitting}
        >
          {isSubmitting ? (
            <LoaderCircleIcon className="animate-spin" />
          ) : null}
          {isSubmitting ? t("common:actions.saving") : t("common:actions.save")}
        </Button>
      </div>
    </form>
  )
}
