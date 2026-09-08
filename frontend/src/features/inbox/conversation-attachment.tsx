/** 在时间线中展示无气泡附件、图片和原地上传状态。 */
import { CheckIcon, ClockIcon, RotateCcwIcon, XIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import {
  AttachmentUploadStatus,
  getAttachmentDownload,
  type MessageAttachment,
} from "@/api"
import { useResource } from "@/hooks/use-resource"
import { resourceKeys } from "@/hooks/resource-keys"
import { formatFileSize } from "@/lib/file-size"
import { cn } from "@/lib/utils"
import { recoverSession } from "@/lib/session-navigation"
import { resolveAppPlatform } from "@/platform/app-platform"
import { openExternalURL } from "@/platform/external-navigation"
import { AttachmentContent } from "./attachment-content"
import { useAttachmentQueue } from "./attachment-queue-context"

/** 用圆环表示上传进度，发送者可在原位置取消或重试。 */
export function ConversationAttachment({
  attachment,
  conversationID,
  messageID,
  originatedAt,
  timeLabel,
  timeTitle,
  incoming,
}: {
  attachment: MessageAttachment
  conversationID: string
  messageID: string
  originatedAt: string
  timeLabel: string
  timeTitle: string
  incoming: boolean
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const { queue, jobs } = useAttachmentQueue()
  const job = jobs.find(
    (item) =>
      item.message.saved?.id === messageID ||
      item.message.attachment?.id === attachment.id,
  )
  const ready =
    attachment.uploadStatus === AttachmentUploadStatus.AttachmentReady ||
    job?.stage === "ready"
  const failed =
    !ready &&
    (attachment.uploadStatus === AttachmentUploadStatus.AttachmentFailed ||
      job?.stage === "failed")
  const image = attachment.imageWidth > 0 && attachment.imageHeight > 0
  const preview = useResource(
    resourceKeys.attachmentDownload(conversationID, messageID),
    () => getAttachmentDownload(conversationID, messageID),
    { enabled: ready && image && Boolean(conversationID) },
  )
  const progress = attachment.byteSize
    ? Math.min((job?.bytes ?? 0) / attachment.byteSize, 1)
    : 0
  const controlLabel = failed ? t("messageRetry") : t("attachmentCancel")

  /** 点击文件图标或名称后，通过浏览器下载已完成的附件。 */
  async function download() {
    try {
      const request = await getAttachmentDownload(conversationID, messageID)
      if (resolveAppPlatform() === "web") {
        const anchor = document.createElement("a")
        anchor.href = request.url
        anchor.download = attachment.name
        document.body.append(anchor)
        anchor.click()
        anchor.remove()
      } else {
        await openExternalURL(request.url)
      }
    } catch (error) {
      if (!recoverSession(error, navigate))
        toast.error(t("attachmentDownloadFailed"))
    }
  }

  const control = ready ? null : (
    <div
      className={`relative flex size-12 items-center justify-center rounded-full ${image ? "bg-black/45 text-white" : "text-primary-foreground"}`}
    >
      <svg
        viewBox="0 0 48 48"
        className={`absolute inset-0 size-full -rotate-90 ${!job && !failed ? "animate-spin" : ""}`}
        aria-hidden="true"
      >
        <circle
          cx="24"
          cy="24"
          r="21"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          opacity="0.25"
        />
        {!failed ? (
          <circle
            cx="24"
            cy="24"
            r="21"
            fill="none"
            stroke="currentColor"
            strokeWidth="2.5"
            strokeLinecap="round"
            strokeDasharray={`${Math.max(progress, 0.06) * 132} 132`}
          />
        ) : null}
      </svg>
      {!incoming && job && queue ? (
        <button
          type="button"
          className="relative flex size-full items-center justify-center rounded-full"
          aria-label={controlLabel}
          onClick={() => {
            if (failed) queue.retry(job.id)
            else
              void queue
                .cancel(job.id)
                .catch(() => toast.error(t("attachmentCancelFailed")))
          }}
        >
          {failed ? (
            <RotateCcwIcon className="size-5" />
          ) : (
            <XIcon className="size-6" />
          )}
        </button>
      ) : (
        <ClockIcon
          className="size-5"
          aria-label={t(
            failed ? "attachmentUploadFailed" : "attachmentReceiving",
          )}
        />
      )}
    </div>
  )
  const footer = (
    <span className="inline-flex items-center gap-1 whitespace-nowrap text-[10px]">
      <time dateTime={originatedAt} title={timeTitle}>
        {timeLabel}
      </time>
      {!incoming ? (
        ready ? (
          <CheckIcon className="size-3.5" aria-label={t("attachmentSent")} />
        ) : (
          <ClockIcon className="size-3.5" />
        )
      ) : null}
    </span>
  )
  const detail = ready
    ? undefined
    : failed
      ? t("attachmentUploadFailed")
      : incoming
        ? t("attachmentReceiving")
        : `${formatFileSize(job?.bytes ?? 0)} / ${formatFileSize(attachment.byteSize)}`
  return (
    <div
      className={cn(
        "min-w-0 max-w-full text-foreground",
        !image && "w-80 rounded-xl border border-border bg-muted px-3 py-2",
      )}
      data-attachment-status={ready ? "ready" : failed ? "failed" : "uploading"}
    >
      <AttachmentContent
        name={attachment.name}
        byteSize={attachment.byteSize}
        imageWidth={attachment.imageWidth}
        imageHeight={attachment.imageHeight}
        previewURL={preview.data?.previewUrl || job?.previewURL}
        action={control}
        detail={detail}
        footer={footer}
        imageFooterClassName={
          ready
            ? "opacity-0 group-hover/message-row:opacity-100 group-focus-within/message-row:opacity-100"
            : undefined
        }
        onOpen={ready ? () => void download() : undefined}
        onImageLoad={() => {
          if (preview.data?.previewUrl && job) queue?.releasePreview(job.id)
        }}
      />
      {image && failed ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {t("attachmentUploadFailed")}
        </p>
      ) : null}
      {image && preview.error ? (
        <button
          type="button"
          className="mt-1 text-xs text-muted-foreground"
          onClick={() => void preview.refresh()}
        >
          {t("attachmentPreviewRetry")}
        </button>
      ) : null}
    </div>
  )
}
