/** 在单聊附件模态框中选择文件和说明，发送后交给时间线上传。 */
import { useEffect, useRef, useState } from "react"
import { PaperclipIcon, XIcon } from "lucide-react"
import { ScrollArea } from "radix-ui"
import { useTranslation } from "react-i18next"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { toast } from "sonner"
import type { InboxConversation } from "@/api"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { FieldLabel } from "@/components/ui/field"
import { ScrollBar } from "@/components/ui/scroll-area"
import { Textarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"
import { AttachmentContent } from "./attachment-content"
import { useAttachmentQueue } from "./attachment-queue-context"
import type { SelectedAttachment } from "./attachment-queue"

/** 选择最多一百个文件并在发送时建立有序消息。 */
export function ConversationAttachmentUpload({
  conversationID,
  targetIdentityID = "",
  disabled,
  onCreated,
}: {
  conversationID: string
  targetIdentityID?: string
  disabled: boolean
  onCreated: (conversation: InboxConversation) => void
}) {
  const { t } = useTranslation("inbox")
  const { t: tCommon } = useTranslation("common")
  const { queue } = useAttachmentQueue()
  const [selected, setSelected] = useState<SelectedAttachment[]>([])
  const selectedRef = useRef<SelectedAttachment[]>([])
  const inputRef = useRef<HTMLInputElement>(null)
  const dialogRef = useRef<HTMLDivElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const previousCountRef = useRef(0)
  const aliveRef = useRef(true)
  const selectingRef = useRef(false)
  const selectionRevision = useRef(0)
  const [selecting, setSelecting] = useState(false)
  const form = useForm({
    defaultValues: { description: "" },
    resolver: zodResolver(
      z.object({
        description: z.string().max(4000, t("attachmentDescriptionTooLong")),
      }),
    ),
    shouldUseNativeValidation: true,
  })
  const description = form.register("description")

  // 追加附件后滚动到列表底部，移除附件时保留浏览位置。
  useEffect(() => {
    if (selected.length > previousCountRef.current && listRef.current) {
      listRef.current.scrollTop = listRef.current.scrollHeight
    }
    previousCountRef.current = selected.length
  }, [selected.length])

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
      for (const item of selectedRef.current)
        if (item.previewURL) URL.revokeObjectURL(item.previewURL)
    }
  }, [])

  /** 释放移除的预览，保持剩余文件的选择顺序。 */
  function replace(items: SelectedAttachment[]) {
    if (!items.length) selectionRevision.current++
    for (const item of selectedRef.current)
      if (!items.includes(item) && item.previewURL)
        URL.revokeObjectURL(item.previewURL)
    selectedRef.current = items
    setSelected(items)
  }

  /** 只读取本地图片尺寸和预览，不创建上传请求。 */
  async function add(files: File[]) {
    if (selectingRef.current) return
    if (selectedRef.current.length + files.length > 100) {
      toast.error(t("attachmentLimit"))
      return
    }
    selectingRef.current = true
    setSelecting(true)
    const revision = selectionRevision.current
    const added: SelectedAttachment[] = []
    try {
      for (const file of files) {
        const item: SelectedAttachment = {
          id: crypto.randomUUID(),
          file,
          previewURL: "",
          imageWidth: 0,
          imageHeight: 0,
        }
        if (file.type.startsWith("image/")) {
          const url = URL.createObjectURL(file)
          const image = new Image()
          image.src = url
          try {
            await image.decode()
            item.previewURL = url
            item.imageWidth = image.naturalWidth
            item.imageHeight = image.naturalHeight
          } catch {
            URL.revokeObjectURL(url)
          }
        }
        added.push(item)
      }
      if (!aliveRef.current || revision !== selectionRevision.current) {
        for (const item of added)
          if (item.previewURL) URL.revokeObjectURL(item.previewURL)
        return
      }
      selectedRef.current = [...selectedRef.current, ...added]
      setSelected(selectedRef.current)
    } finally {
      selectingRef.current = false
      if (aliveRef.current) setSelecting(false)
    }
  }

  /** 把文件所有权移交工作台队列，立即关闭选择框。 */
  function send(values: { description: string }) {
    if (!queue || selectedRef.current.length === 0) return
    // 说明只随最后一个附件发送，其余附件保持独立消息。
    queue.enqueue(
      selectedRef.current.map((item, index) => ({
        ...item,
        body: index === selectedRef.current.length - 1 ? values.description : "",
      })),
      conversationID,
      targetIdentityID,
      (conversation) => {
        if (aliveRef.current && conversation) onCreated(conversation)
      },
    )
    selectedRef.current = []
    setSelected([])
    form.reset()
  }

  return (
    <>
      <input
        ref={inputRef}
        type="file"
        multiple
        className="hidden"
        aria-label={t("attachmentAdd")}
        onChange={(event) => {
          const files = Array.from(event.currentTarget.files ?? [])
          event.currentTarget.value = ""
          void add(files)
        }}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        disabled={disabled}
        aria-label={t("attachmentAdd")}
        onClick={() => inputRef.current?.click()}
      >
        <PaperclipIcon />
      </Button>
      <Dialog
        open={selected.length > 0}
        onOpenChange={(open) => {
          if (!open) {
            replace([])
            form.reset()
          }
        }}
      >
        <DialogContent
          ref={dialogRef}
          className="max-h-[85dvh] sm:max-w-lg"
          aria-describedby={undefined}
          onInteractOutside={(event) => event.preventDefault()}
          onOpenAutoFocus={(event) => {
            // 聚焦弹窗，避免首个附件的删除按钮把列表带回顶部。
            event.preventDefault()
            dialogRef.current?.focus({ preventScroll: true })
            if (listRef.current) {
              listRef.current.scrollTop = listRef.current.scrollHeight
            }
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("attachmentSend")}</DialogTitle>
          </DialogHeader>
          <form
            className="min-h-0 min-w-0 space-y-9"
            onSubmit={(event) => {
              event.stopPropagation()
              void form.handleSubmit(send)(event)
            }}
          >
            <div className="space-y-5">
              <ScrollArea.Root type="auto" className="relative -mx-6 min-w-0">
                <ScrollArea.Viewport
                  ref={listRef}
                  className="max-h-[45dvh] w-full overscroll-contain [&>div]:!block"
                >
                  <div className="space-y-4 py-1 pl-6 pr-8">
                    {selected.map((item) => (
                      <div
                        key={item.id}
                        className="flex min-w-0 items-center gap-3"
                      >
                        <div
                          className={cn(
                            "min-w-0 flex-1",
                            item.imageWidth > 0 &&
                              item.imageHeight > 0 &&
                              "flex justify-center rounded-xl bg-muted p-3",
                          )}
                        >
                          <AttachmentContent
                            name={item.file.name}
                            byteSize={item.file.size}
                            previewURL={item.previewURL}
                            imageWidth={item.imageWidth}
                            imageHeight={item.imageHeight}
                          />
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          aria-label={t("attachmentRemove", {
                            name: item.file.name,
                          })}
                          onClick={() =>
                            replace(
                              selectedRef.current.filter(
                                (value) => value.id !== item.id,
                              ),
                            )
                          }
                        >
                          <XIcon />
                        </Button>
                      </div>
                    ))}
                  </div>
                </ScrollArea.Viewport>
                <ScrollBar />
              </ScrollArea.Root>
              <div className="space-y-2">
                <FieldLabel htmlFor="attachment-description">
                  {t("attachmentDescription")}
                </FieldLabel>
                <Textarea
                  {...description}
                  id="attachment-description"
                  rows={1}
                  className="min-h-0 max-h-[184px] resize-none leading-6"
                  onChange={(event) => {
                    void description.onChange(event)
                    event.currentTarget.style.height = "auto"
                    event.currentTarget.style.height = `${Math.min(event.currentTarget.scrollHeight, 184)}px`
                  }}
                />
              </div>
            </div>
            <div className="flex items-center justify-between">
              <Button
                type="button"
                variant="outline"
                disabled={selected.length >= 100 || selecting}
                onClick={() => inputRef.current?.click()}
              >
                {t("attachmentAppend")}
              </Button>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => {
                    replace([])
                    form.reset()
                  }}
                >
                  {tCommon("actions.cancel")}
                </Button>
                <Button type="submit" disabled={selecting}>
                  {t("messageSend")}
                </Button>
              </div>
            </div>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
