/** 企业内部群聊创建表单。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon, SearchIcon } from "lucide-react"
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
  OrganizationIdentityType,
  type GroupInboxConversationData,
  uploadFile,
} from "@/api"
import { ProfileAvatar } from "@/components/profile-avatar"
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
import { ScrollArea } from "@/components/ui/scroll-area"
import { Textarea } from "@/components/ui/textarea"
import { GroupImagePicker } from "@/features/inbox/group-avatar"
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const groupTitleMaxLength = 100
const groupDescriptionMaxLength = 500
const groupAdditionalMemberMaxCount = 99

type PendingGroupImage = {
  requestID: number
  file: File
  previewURL: string
  status: "uploading" | "uploaded" | "failed"
  fileID: string
  upload?: Promise<string>
}

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
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const [query, setQuery] = useState("")
  const imageRequestID = useRef(0)
  const [pendingImage, setPendingImage] =
    useState<PendingGroupImage | null>(null)
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

  useEffect(() => {
    const previewURL = pendingImage?.previewURL
    return () => {
      if (previewURL) URL.revokeObjectURL(previewURL)
    }
  }, [pendingImage?.previewURL])

  useEffect(() => {
    return () => {
      imageRequestID.current += 1
    }
  }, [])
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
  const candidates = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    return (data ?? []).filter(
      (member) =>
        member.id !== currentIdentityID &&
        member.type ===
          OrganizationIdentityType.OrganizationIdentityTypeUser &&
        (!normalizedQuery ||
          member.displayName.toLocaleLowerCase().includes(normalizedQuery)),
    )
  }, [currentIdentityID, data, query])

  /** 跟踪当前候选群图片的上传结果。 */
  function monitorImageUpload(
    candidate: PendingGroupImage,
    upload: Promise<string>,
  ) {
    void upload.then(
      (fileID) => {
        if (imageRequestID.current !== candidate.requestID) return
        setPendingImage((current) =>
          current?.requestID === candidate.requestID
            ? { ...current, status: "uploaded", fileID, upload: undefined }
            : current,
        )
      },
      (error) => {
        if (imageRequestID.current !== candidate.requestID) return
        setPendingImage((current) =>
          current?.requestID === candidate.requestID
            ? { ...current, status: "failed", upload: undefined }
            : current,
        )
        console.warn("上传群聊图片失败", error)
        if (!recoverSession(error, navigate)) {
          toast.error(t("groupImageUploadError"))
        }
      },
    )
  }

  /** 立即上传候选群图片并保留创建群聊所需的文件编号。 */
  function startImageUpload(candidate: PendingGroupImage) {
    const upload = uploadFile(
      candidate.file,
      FilePurpose.FilePurposeGroupImage,
    ).then((file) => file.id)
    const uploading = { ...candidate, status: "uploading" as const, upload }
    setPendingImage(uploading)
    monitorImageUpload(uploading, upload)
    return upload
  }

  /** 预览并上传新选择的群图片。 */
  function prepareImage(file: File) {
    const candidate: PendingGroupImage = {
      requestID: imageRequestID.current + 1,
      file,
      previewURL: URL.createObjectURL(file),
      status: "uploading",
      fileID: "",
    }
    imageRequestID.current = candidate.requestID
    startImageUpload(candidate)
  }

  /** 移除尚未提交的候选群图片。 */
  function discardImage() {
    imageRequestID.current += 1
    setPendingImage(null)
  }

  /** 创建群聊并关闭表单。 */
  async function create(values: GroupConversationValues) {
    let uploadingImage = false
    try {
      let imageFileId = pendingImage?.fileID ?? ""
      if (pendingImage && !imageFileId) {
        uploadingImage = true
        imageFileId = await (
          pendingImage.upload ?? startImageUpload(pendingImage)
        )
        uploadingImage = false
      }
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
      if (recoverSession(createError, navigate)) return
      console.warn("创建企业内部群聊失败", { error: createError })
      if (uploadingImage) return
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
      imageRequestID.current += 1
      setPendingImage(null)
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
                  onSelect={prepareImage}
                />
                {pendingImage ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={form.formState.isSubmitting}
                    onClick={discardImage}
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
            <div className="grid min-h-0 gap-2">
              <div className="flex items-center justify-between gap-3">
                <FieldLabel htmlFor="group-member-search" required>
                  {t("groupMembersLabel")}
                </FieldLabel>
                <span className="text-xs text-muted-foreground">
                  {t("groupMembersSelected", {
                    count: selectedIdentityIDs.length,
                  })}
                </span>
              </div>
              <div className="relative">
                <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="group-member-search"
                  ref={memberIdentityIDsField.ref}
                  value={query}
                  autoComplete="off"
                  className="pl-9"
                  onChange={(event) => setQuery(event.target.value)}
                />
              </div>
              <ScrollArea className="h-64 rounded-md border">
                {loading ? (
                  <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
                    {t("groupMembersLoading")}
                  </div>
                ) : error ? (
                  <div className="flex h-64 flex-col items-center justify-center p-6 text-center">
                    <p className="text-sm text-muted-foreground">
                      {t("groupMembersLoadError")}
                    </p>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="mt-3"
                      onClick={() => void refresh()}
                    >
                      {t("messagesRetry")}
                    </Button>
                  </div>
                ) : candidates.length === 0 ? (
                  <p className="px-6 py-12 text-center text-sm text-muted-foreground">
                    {t("groupMembersEmpty")}
                  </p>
                ) : (
                  <div className="grid p-1.5">
                    {candidates.map((member) => {
                      const selected = selectedIdentityIDs.includes(member.id)
                      const selectionFull =
                        selectedIdentityIDs.length >=
                        groupAdditionalMemberMaxCount
                      return (
                        <label
                          key={member.id}
                          className="flex items-center gap-3 rounded-md px-3 py-2.5 transition-colors hover:bg-muted"
                        >
                          <input
                            type="checkbox"
                            name={memberIdentityIDsField.name}
                            checked={selected}
                            disabled={!selected && selectionFull}
                            className="size-4 rounded border-input accent-primary"
                            onBlur={memberIdentityIDsField.onBlur}
                            onChange={(event) => {
                              memberIdentityIDsField.onChange(
                                event.target.checked
                                  ? [...selectedIdentityIDs, member.id]
                                  : selectedIdentityIDs.filter(
                                      (identityID) => identityID !== member.id,
                                    ),
                              )
                            }}
                          />
                          <ProfileAvatar imageURL={member.avatarUrl} name={member.displayName} className="size-9" />
                          <span className="min-w-0 flex-1 truncate text-sm">
                            {member.displayName}
                          </span>
                        </label>
                      )
                    })}
                  </div>
                )}
              </ScrollArea>
            </div>
          </div>
          <div className="flex shrink-0 justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={isSubmitting}
              onClick={() => changeOpen(false)}
            >
              {t("groupCreateCancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {t("groupCreateSubmit")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
