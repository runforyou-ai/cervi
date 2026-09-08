/** 选择、上传并发送当前内部会话的单个附件。 */
import { useEffect, useRef, useState } from "react"
import { PaperclipIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  sendAttachmentMessage,
  type ConversationMessageData,
  type InboxConversation,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { useAttachmentUpload } from "./use-attachment-upload"
import { formatFileSize } from "@/lib/file-size"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 展示附件选择按钮和保留当前聊天上下文的上传对话框。 */
export function GroupAttachmentUpload({
  conversationID,
  targetIdentityID = "",
  disabled,
  onSent,
}: {
  conversationID: string
  targetIdentityID?: string
  disabled: boolean
  onSent: (
    clientMessageID: string,
    message: ConversationMessageData,
    conversation: InboxConversation | null,
  ) => void
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const upload = useAttachmentUpload()
  const inputRef = useRef<HTMLInputElement>(null)
  const clientMessageID = useRef("")
  const aliveRef = useRef(true)
  const sendingRef = useRef(false)
  const [sending, setSending] = useState(false)

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
    }
  }, [])

  /** 展示上传或发送错误，并恢复失效会话入口。 */
  function showError(error: unknown, fallback: string) {
    if (!aliveRef.current || recoverSession(error, navigate)) return
    toast.error(isApiError(error) ? apiErrorMessage(error, []) : fallback)
  }

  /** 将已上传附件保存为独立消息，失败后保留同一发送编号。 */
  async function send() {
    if (!upload.uploaded || sendingRef.current) return
    sendingRef.current = true
    setSending(true)
    upload.markSending(true)
    try {
      const result = await sendAttachmentMessage({
        conversationId: targetIdentityID ? "" : conversationID,
        targetIdentityId: targetIdentityID,
        clientMessageId: clientMessageID.current,
        fileId: upload.uploaded.id,
      })
      upload.clear(true)
      if (aliveRef.current)
        onSent(clientMessageID.current, result.message, result.conversation)
    } catch (error) {
      showError(error, t("messageSendError"))
      if (!aliveRef.current) upload.clear()
    } finally {
      sendingRef.current = false
      upload.markSending(false)
      if (aliveRef.current) setSending(false)
    }
  }

  return (
    <>
      <input
        ref={inputRef}
        type="file"
        className="hidden"
        aria-label={t("attachmentAdd")}
        onChange={(event) => {
          const selected = event.currentTarget.files?.[0]
          event.currentTarget.value = ""
          if (!selected) return
          clientMessageID.current = window.crypto.randomUUID()
          void upload
            .select(selected)
            .catch((error) => showError(error, t("attachmentUploadFailed")))
        }}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        disabled={disabled}
        aria-label={t("attachmentAdd")}
        title={t("attachmentAdd")}
        onClick={() => inputRef.current?.click()}
      >
        <PaperclipIcon />
      </Button>
      <Dialog
        open={upload.file !== null}
        onOpenChange={(open) => {
          if (!open && !sendingRef.current) upload.clear()
        }}
      >
        <DialogContent
          className="sm:max-w-md"
          onInteractOutside={(event) => event.preventDefault()}
        >
          <DialogHeader>
            <DialogTitle>{t("attachmentSend")}</DialogTitle>
            <DialogDescription className="break-all">
              {upload.file?.name}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-9">
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">
                {formatFileSize(upload.file?.size ?? 0)}
              </p>
              <progress
                className="block h-2 w-full accent-primary"
                value={upload.progress}
                max={1}
                aria-label={t("attachmentUploadProgress")}
              />
              <p className="min-h-5 text-sm" role="status">
                {t(
                  upload.status === "failed"
                    ? "attachmentUploadFailed"
                    : upload.status === "uploaded"
                      ? "attachmentUploadReady"
                      : "attachmentUploading",
                  { percent: Math.floor(upload.progress * 100) },
                )}
              </p>
            </div>
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                disabled={sending}
                onClick={() => upload.clear()}
              >
                {t("attachmentCancel")}
              </Button>
              {upload.status === "failed" ? (
                <Button
                  type="button"
                  onClick={() =>
                    void upload
                      .retry()
                      .catch((error) =>
                        showError(error, t("attachmentUploadFailed")),
                      )
                  }
                >
                  {t("messageRetry")}
                </Button>
              ) : (
                <Button
                  type="button"
                  disabled={upload.status !== "uploaded" || sending}
                  onClick={() => void send()}
                >
                  {t(sending ? "messageSending" : "messageSend")}
                </Button>
              )}
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
