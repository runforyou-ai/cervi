/** 移动端群头像、名称和描述的独立编辑页。 */
import { useEffect, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import {
  Navigate,
  useNavigate,
  useOutletContext,
  useParams,
} from "react-router"
import { toast } from "sonner"
import { z } from "zod"
import {
  ConversationStatus,
  FilePurpose,
  uploadFile,
  updateGroupConversation,
} from "@/api"
import type { MobileGroupDetailsContext } from "@/apps/mobile/mobile-group-context"
import { useMobileBack } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { Button } from "@/components/ui/button"
import { FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { GroupImagePicker } from "@/features/inbox/group-avatar"
import { useImmediateSave } from "@/hooks/use-immediate-save"
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
            context.group.status ===
              ConversationStatus.ConversationStatusArchived
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
  const upload = useImmediateSave()
  const [imageFileID, setImageFileID] = useState<string | null>(null)
  const [preview, setPreview] = useState("")
  const previewRef = useRef("")
  const alive = useRef(false)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
      URL.revokeObjectURL(previewRef.current)
    }
  }, [])
  const label = t(
    field === "image"
      ? "groupImageLabel"
      : field === "title"
        ? "groupTitleLabel"
        : "groupDescriptionLabel",
  )
  const schema = z.object({
    value:
      field === "title"
        ? z
            .string()
            .trim()
            .min(1, t("groupTitleRequired"))
            .max(100, t("groupTitleTooLong"))
        : z.string().trim().max(500),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { value: field === "image" ? "" : group[field] },
  })

  /** 选图立即上传，保存时再关联临时文件。 */
  async function selectImage(file: File) {
    const request = upload.begin()
    if (request === null) return
    try {
      const result = await uploadFile(file, FilePurpose.FilePurposeGroupImage)
      if (!upload.isCurrent(request)) return
      URL.revokeObjectURL(previewRef.current)
      previewRef.current = URL.createObjectURL(file)
      setPreview(previewRef.current)
      setImageFileID(result.id)
    } catch (error) {
      if (!upload.isCurrent(request) || recoverSession(error, navigate)) return
      console.warn("移动端上传群图片失败", error)
      toast.error(t("groupImageUploadError"))
    } finally {
      upload.finish(request)
    }
  }

  /** 保存当前字段，离开编辑页后忽略迟到的导航结果。 */
  async function save(values: z.infer<typeof schema>) {
    if (upload.isSaving()) return
    const success = await onSave(() =>
      updateGroupConversation(group.id, {
        title: field === "title" ? values.value : group.title,
        description: field === "description" ? values.value : group.description,
        imageFileId: field === "image" ? imageFileID : null,
      }),
    )
    if (success && alive.current) close()
  }

  const disabled = busy || upload.saving
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
                imageURL={preview || group.imageUrl}
                className="size-24"
                disabled={disabled}
                loading={upload.saving}
                onSelect={(file) => void selectImage(file)}
              />
              <p className="text-xs text-muted-foreground">
                {tm("group.changeImage")}
              </p>
            </div>
          ) : (
            <>
              <FieldLabel
                htmlFor="mobile-edit-group-value"
                required={field === "title"}
              >
                {label}
              </FieldLabel>
              {field === "title" ? (
                <Input
                  {...form.register("value")}
                  id="mobile-edit-group-value"
                  required
                  maxLength={100}
                  disabled={disabled}
                  className="min-h-11"
                />
              ) : (
                <Textarea
                  {...form.register("value")}
                  id="mobile-edit-group-value"
                  maxLength={500}
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
            disabled={disabled || (field === "image" && !imageFileID)}
          >
            {tm("group.complete")}
          </Button>
        </div>
      </form>
    </section>
  )
}
