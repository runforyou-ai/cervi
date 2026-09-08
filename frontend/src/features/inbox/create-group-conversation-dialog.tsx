/** 企业内部群聊创建表单。 */
import { useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useController, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createGroupConversation,
  FilePurpose,
  isApiError,
  isGroupInboxConversation,
  type GroupInboxConversationData,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { GroupMemberPicker } from "@/features/inbox/group-member-picker"
import { GroupImagePicker } from "@/features/inbox/group-avatar"
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { usePendingImageUpload } from "@/hooks/use-pending-image-upload"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const groupTitleMaxLength = 100
const groupDescriptionMaxLength = 500
const groupAdditionalMemberMaxCount = 99

/** 创建群聊表单校验规则。 */
function createGroupConversationSchema(messages: {
  titleRequired: string
  titleTooLong: string
  descriptionTooLong: string
  membersRequired: string
  membersTooMany: string
}) {
  return z.object({
    title: z
      .string()
      .trim()
      .min(1, messages.titleRequired)
      .max(groupTitleMaxLength, messages.titleTooLong),
    description: z
      .string()
      .trim()
      .max(groupDescriptionMaxLength, messages.descriptionTooLong),
    memberIdentityIds: z
      .array(z.string())
      .min(1, messages.membersRequired)
      .max(groupAdditionalMemberMaxCount, messages.membersTooMany),
  })
}

type GroupConversationValues = z.infer<
  ReturnType<typeof createGroupConversationSchema>
>

/** 选择初始成员并创建企业内部群聊。 */
export function CreateGroupConversationDialog({
  open,
  currentIdentityID,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  currentIdentityID: string
  onOpenChange: (open: boolean) => void
  onCreated: (conversation: GroupInboxConversationData) => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const navigate = useNavigate()
  const [query, setQuery] = useState("")
  const image = usePendingImageUpload({
    purpose: FilePurpose.FilePurposeGroupImage,
    onError: (error) => {
      console.warn("上传群聊图片失败", error)
      if (!recoverSession(error, navigate)) toast.error(t("groupImageUploadError"))
    },
  })
  const pendingImage = image.pending
  const schema = useMemo(
    () =>
      createGroupConversationSchema({
        titleRequired: t("groupTitleRequired"),
        titleTooLong: t("groupTitleTooLong"),
        descriptionTooLong: t("groupDescriptionTooLong"),
        membersRequired: t("groupMembersRequired"),
        membersTooMany: t("groupMembersTooMany"),
      }),
    [t],
  )
  const form = useForm<GroupConversationValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { title: "", description: "", memberIdentityIds: [] },
  })

  const { field: memberIdentityIDsField } = useController({
    control: form.control,
    name: "memberIdentityIds",
  })
  const selectedIdentityIDs = memberIdentityIDsField.value
  const { data, loading, error, refresh } = useResource(
    resourceKeys.memberOptions(),
    listAllMemberOptions,
    { enabled: open, staleTime: 0 },
  )
  const candidates = (data ?? []).filter((member) => member.id !== currentIdentityID)

  /** 创建群聊并关闭表单。 */
  async function create(values: GroupConversationValues) {
    let uploadingImage = false
    try {
      uploadingImage = Boolean(pendingImage && !pendingImage.fileID)
      const imageFileId = await image.ensureUploaded()
      uploadingImage = false
      const conversation = await createGroupConversation({
        title: values.title.trim(),
        description: values.description.trim(),
        imageFileId,
        memberIdentityIds: values.memberIdentityIds,
      })
      if (!isGroupInboxConversation(conversation)) {
        throw new Error("企业群聊响应结构无效")
      }
      onCreated(conversation)
      changeOpen(false)
    } catch (createError) {
      // 上传失败已由共享上传回调提示，创建只处理群聊提交错误。
      if (uploadingImage) return
      if (recoverSession(createError, navigate)) return
      console.warn("创建企业内部群聊失败", { error: createError })
      toast.error(
        isApiError(createError)
          ? apiErrorMessage(createError, [
              "title",
              "description",
              "imageFileId",
              "memberIdentityIds",
            ])
          : t("groupCreateError"),
      )
    }
  }

  /** 关闭时清空尚未提交的群聊表单。 */
  function changeOpen(nextOpen: boolean) {
    if (!nextOpen) {
      form.reset()
      setQuery("")
      image.clear()
    }
    onOpenChange(nextOpen)
  }

  const { isSubmitting } = form.formState

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent className="grid-rows-[auto_minmax(0,1fr)] overflow-hidden sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("groupCreateTitle")}</DialogTitle>
          <DialogDescription>{t("groupCreateDescription")}</DialogDescription>
        </DialogHeader>
        <form
          className="grid min-h-0 grid-rows-[minmax(0,1fr)_auto] gap-9 overflow-hidden"
          onSubmit={form.handleSubmit(create)}
          noValidate
        >
          <div className="grid min-h-0 gap-5 overflow-y-auto pr-1">
            <div className="space-y-1.5">
              <span className="block text-sm font-medium">
                {t("groupImageLabel")}
              </span>
              <div className="flex items-center gap-3">
                <GroupImagePicker
                  imageURL={pendingImage?.previewURL}
                  disabled={form.formState.isSubmitting}
                  loading={pendingImage?.status === "uploading"}
                  onSelect={image.select}
                />
                {pendingImage ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={form.formState.isSubmitting}
                    onClick={image.clear}
                  >
                    {t("groupImageDiscard")}
                  </Button>
                ) : null}
              </div>
            </div>
            <div className="space-y-1.5">
              <FieldLabel htmlFor="group-title" required>
                {t("groupTitleLabel")}
              </FieldLabel>
              <Input
                {...form.register("title")}
                id="group-title"
                autoComplete="off"
                maxLength={groupTitleMaxLength}
                required
              />
            </div>
            <div className="space-y-1.5">
              <FieldLabel htmlFor="group-description">
                {t("groupDescriptionLabel")}
              </FieldLabel>
              <Textarea
                {...form.register("description")}
                id="group-description"
                rows={3}
                maxLength={groupDescriptionMaxLength}
                className="min-h-20 resize-y"
              />
            </div>
            <GroupMemberPicker
              label={t("groupMembersLabel")}
              emptyMessage={t("groupMembersEmpty")}
              members={candidates}
              selected={selectedIdentityIDs}
              onChange={memberIdentityIDsField.onChange}
              query={query}
              onQueryChange={setQuery}
              selectionLimit={groupAdditionalMemberMaxCount}
              disabled={isSubmitting}
              required
              showCount
              inputRef={memberIdentityIDsField.ref}
              name={memberIdentityIDsField.name}
              onBlur={memberIdentityIDsField.onBlur}
              loading={loading}
              error={Boolean(error)}
              onRetry={() => void refresh()}
            />
          </div>
          <div className="flex shrink-0 justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={isSubmitting}
              onClick={() => changeOpen(false)}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {t("common:actions.create")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
