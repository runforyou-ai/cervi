/** 移动端群头像、名称和描述的独立编辑页。 */
import { useEffect, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Navigate, useNavigate, useOutletContext, useParams } from "react-router"
import { toast } from "sonner"
import { z } from "zod"
import { ConversationStatus, FilePurpose, updateGroupConversation } from "@/api"
import type { MobileGroupDetailsContext } from "@/apps/mobile/mobile-group-context"
import { useMobileBack } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { Button } from "@/components/ui/button"
import { FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { GroupImagePicker } from "@/features/inbox/group-avatar"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import {
  createGroupProfileSchema,
  groupTitleMaxLength,
  groupDescriptionMaxLength,
} from "@/features/inbox/group-conversation-schema"
import { recoverSession } from "@/lib/session-navigation"

/** 按字段隔离表单，拒绝无效字段和失去群主资格的编辑入口。 */
export function MobileGroupProfileEditor() {
  const { t } = useTranslation("mobile")
  const context = useOutletContext<MobileGroupDetailsContext>()
  const { field } = useParams()
  if (field !== "image" && field !== "title" && field !== "description")
    return <Navigate to={`/inbox/group/${context.group.id}/details`} replace />
  if (!context.canManage)
    return (
      <section className="flex h-full min-h-0 flex-col bg-background">
        <MobilePageHeader
          title={t("group.profile")}
          backTo={`/inbox/group/${context.group.id}/details`}
        />
        <p className="p-4 text-sm text-muted-foreground" role="status">
          {t(
            context.group.status === ConversationStatus.ConversationStatusArchived
              ? "group.editArchived"
              : "group.editOwnerOnly",
          )}
        </p>
      </section>
    )
  return <MobileGroupFieldEditor key={field} field={field} {...context} />
}

/** 只编辑选中的资料项，其他字段采用当前服务端资料。 */
function MobileGroupFieldEditor({
  group,
  field,
  busy,
  onSave,
}: MobileGroupDetailsContext & { field: "image" | "title" | "description" }) {
  const { t } = useTranslation("inbox")
  const { t: tm } = useTranslation("mobile")
  const navigate = useNavigate()
  const close = useMobileBack(`/inbox/group/${group.id}/details`)
  const image = usePendingImageUpload({
    purpose: FilePurpose.FilePurposeGroupImage,
    onError: (error) => {
      if (recoverSession(error, navigate)) return
      console.warn("移动端上传群图片失败", error)
      toast.error(t("groupImageUploadError"))
    },
  })
  const alive = useRef(false)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  const label = t(
    field === "image"
      ? "groupImageLabel"
      : field === "title"
        ? "groupTitleLabel"
        : "groupDescriptionLabel",
  )
  const profileSchema = createGroupProfileSchema(t)
  const schema = z.object({
    value:
      field === "title" ? profileSchema.shape.title : profileSchema.shape.description,
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { value: field === "image" ? "" : group[field] },
  })

  /** 保存当前字段，离开编辑页后忽略迟到的导航结果。 */
  async function save(values: z.infer<typeof schema>) {
    // 上传失败由图片 Hook 提示，保留候选供再次保存时重试。
    const imageFileID =
      field === "image" ? await image.ensureUploaded().catch(() => null) : null
    if (!alive.current || (field === "image" && !imageFileID)) return
    const success = await onSave(() =>
      updateGroupConversation(group.id, {
        title: field === "title" ? values.value : group.title,
        description: field === "description" ? values.value : group.description,
        imageFileId: field === "image" ? imageFileID : null,
      }),
    )
    if (success && alive.current) close()
  }

  const uploading = image.pending?.status === "uploading"
  const disabled = busy || form.formState.isSubmitting || uploading
  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobilePageHeader
        title={label}
        backTo={`/inbox/group/${group.id}/details`}
        backDisabled={busy}
      />
      <form
        className="min-h-0 flex-1 space-y-9 overflow-y-auto p-4"
        noValidate
        onSubmit={form.handleSubmit(save)}
      >
        <div className="space-y-2">
          {field === "image" ? (
            <div className="flex flex-col items-center gap-2 text-center">
              <GroupImagePicker
                imageURL={image.pending?.previewURL || group.imageUrl}
                className="size-24"
                disabled={disabled}
                loading={uploading}
                onSelect={image.select}
              />
              <p className="text-xs text-muted-foreground">{tm("group.changeImage")}</p>
            </div>
          ) : (
            <>
              <FieldLabel htmlFor="mobile-edit-group-value" required={field === "title"}>
                {label}
              </FieldLabel>
              {field === "title" ? (
                <Input
                  {...form.register("value")}
                  id="mobile-edit-group-value"
                  required
                  maxLength={groupTitleMaxLength}
                  disabled={disabled}
                  className="min-h-11"
                />
              ) : (
                <Textarea
                  {...form.register("value")}
                  id="mobile-edit-group-value"
                  maxLength={groupDescriptionMaxLength}
                  rows={6}
                  disabled={disabled}
                />
              )}
            </>
          )}
        </div>
        <div className="w-full">
          <Button
            type="submit"
            className="min-h-11 w-full"
            disabled={disabled || (field === "image" && !image.pending)}
          >
            {tm("group.complete")}
          </Button>
        </div>
      </form>
    </section>
  )
}
