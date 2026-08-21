/** 个人资料设置表单。 */
import { useEffect, useMemo, useRef, useState, type ChangeEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  FilePurpose,
  isApiError,
  selectProfileImage,
  updateProfile,
  uploadFile,
  type User,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  createProfileSettingsSchema,
  type ProfileSettingsFormValues,
} from "@/features/settings/profile-settings-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { UserAvatar } from "@/features/users/user-avatar"
import { resolveAppPlatform } from "@/platform/app-platform"

const avatarContentTypes = new Set(["image/jpeg", "image/png", "image/webp"])
const maxAvatarByteSize = 5 * 1024 * 1024
const avatarFileAccept = ".jpg,.jpeg,.png,.webp,image/jpeg,image/png,image/webp"

type PendingAvatar = {
  requestID: number
  file: File
  previewURL: string
  status: "uploading" | "uploaded" | "failed"
  fileID: string
  upload?: Promise<string>
}

/** 修改当前用户的头像、姓名和邮箱。 */
export function ProfileSettingsForm({
  user,
  onUpdated,
}: {
  user: User
  onUpdated: (user: User) => void
}) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const avatarRequestID = useRef(0)
  const mounted = useRef(true)
  const [pendingAvatar, setPendingAvatar] = useState<PendingAvatar | null>(null)
  const [selectingAvatar, setSelectingAvatar] = useState(false)
  const schema = useMemo(() => createProfileSettingsSchema(t), [t])
  const form = useForm<ProfileSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      displayName: user.displayName,
      email: user.email,
    },
  })

  useEffect(() => {
    const previewURL = pendingAvatar?.previewURL
    return () => {
      if (previewURL) {
        URL.revokeObjectURL(previewURL)
      }
    }
  }, [pendingAvatar?.previewURL])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 跟踪当前候选头像的上传结果。 */
  function monitorAvatarUpload(candidate: PendingAvatar) {
    if (!candidate.upload) {
      return
    }
    void candidate.upload.then(
      (fileID) => {
        if (
          !mounted.current ||
          avatarRequestID.current !== candidate.requestID
        ) {
          return
        }
        setPendingAvatar((current) =>
          current?.requestID === candidate.requestID
            ? { ...current, status: "uploaded", fileID, upload: undefined }
            : current,
        )
      },
      (error) => {
        if (
          !mounted.current ||
          avatarRequestID.current !== candidate.requestID
        ) {
          return
        }
        setPendingAvatar((current) =>
          current?.requestID === candidate.requestID
            ? { ...current, status: "failed", upload: undefined }
            : current,
        )
        console.warn("上传用户头像失败", error)
        if (!recoverSession(error, navigate)) {
          toast.error(t("profile.avatarUploadError"))
        }
      },
    )
  }

  /** 立即上传候选头像并保留业务关联所需的文件编号。 */
  function startAvatarUpload(candidate: PendingAvatar) {
    const upload = uploadFile(
      candidate.file,
      FilePurpose.FilePurposeUserAvatar,
    ).then((file) => file.id)
    const uploading = { ...candidate, status: "uploading" as const, upload }
    setPendingAvatar(uploading)
    monitorAvatarUpload(uploading)
    return upload
  }

  /** 校验新的用户头像并在保存前预览。 */
  function previewAvatar(selected: File) {
    if (!avatarContentTypes.has(selected.type)) {
      toast.error(t("profile.avatarTypeError"))
      return false
    }
    if (selected.size <= 0 || selected.size > maxAvatarByteSize) {
      toast.error(t("profile.avatarSizeError"))
      return false
    }
    const candidate: PendingAvatar = {
      requestID: avatarRequestID.current + 1,
      file: selected,
      previewURL: URL.createObjectURL(selected),
      status: "uploading",
      fileID: "",
    }
    avatarRequestID.current = candidate.requestID
    startAvatarUpload(candidate)
    return true
  }

  /** 处理 Web 文件选择器返回的头像图片。 */
  function selectBrowserAvatar(event: ChangeEvent<HTMLInputElement>) {
    const selected = event.target.files?.[0]
    event.target.value = ""
    if (selected) {
      previewAvatar(selected)
    }
  }

  /** 按当前平台打开头像图片选择器。 */
  async function chooseAvatar() {
    if (resolveAppPlatform() !== "desktop") {
      fileInputRef.current?.click()
      return
    }
    setSelectingAvatar(true)
    try {
      const selected = await selectProfileImage()
      if (!selected.name) {
        return
      }
      const binary = window.atob(selected.dataBase64)
      const content = new Uint8Array(binary.length)
      for (let index = 0; index < binary.length; index += 1) {
        content[index] = binary.charCodeAt(index)
      }
      previewAvatar(
        new File([content], selected.name, { type: selected.contentType }),
      )
    } catch (error) {
      console.warn("选择用户头像失败", error)
      toast.error(t("profile.avatarChooseError"))
    } finally {
      setSelectingAvatar(false)
    }
  }

  /** 保存个人资料并同步工作台中的当前用户。 */
  async function save(values: ProfileSettingsFormValues) {
    let uploadingAvatar = false
    try {
      let avatarFileId = ""
      if (pendingAvatar) {
        avatarFileId = pendingAvatar.fileID
        if (!avatarFileId) {
          uploadingAvatar = true
          avatarFileId = await (
            pendingAvatar.upload ?? startAvatarUpload(pendingAvatar)
          )
          uploadingAvatar = false
        }
      }
      const updated = await updateProfile({ ...values, avatarFileId })
      form.reset({
        displayName: updated.displayName,
        email: updated.email,
      })
      avatarRequestID.current += 1
      setPendingAvatar(null)
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
      toast.error(
        t(uploadingAvatar ? "profile.avatarUploadError" : "profile.saveError"),
      )
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
        <Field>
          <FieldLabel>{t("profile.avatar")}</FieldLabel>
          <div>
            <input
              ref={fileInputRef}
              className="sr-only"
              type="file"
              accept={avatarFileAccept}
              aria-label={t("profile.avatarChoose")}
              onChange={selectBrowserAvatar}
            />
            <button
              className="group relative size-20 overflow-hidden rounded-full outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
              type="button"
              aria-label={t("profile.avatarChoose")}
              disabled={selectingAvatar || isSubmitting}
              onClick={() => void chooseAvatar()}
            >
              <UserAvatar
                user={
                  pendingAvatar
                    ? { ...user, avatarUrl: pendingAvatar.previewURL }
                    : user
                }
                className="size-full rounded-full text-2xl"
              />
              <span className="absolute inset-0 flex items-center justify-center bg-black/55 text-xs font-medium text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100">
                {selectingAvatar || pendingAvatar?.status === "uploading" ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : (
                  t("profile.avatarChange")
                )}
              </span>
            </button>
          </div>
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
          <Button
            type="submit"
            disabled={(!isDirty && !pendingAvatar) || isSubmitting}
          >
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
