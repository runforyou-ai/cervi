/** 显示知识文档上传批次及每个原件的进度。 */
import { useRef } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from "@/components/ui/dialog"
import { formatFileSize } from "@/lib/file-size"
import { useKnowledgeDocumentUpload, knowledgeDocumentFormats } from "./use-knowledge-document-upload"

/** 在文档列表中显示批次进度并保留失败重试入口。 */
export function KnowledgeDocumentUpload({ baseId, groupId }: { baseId: string; groupId: string }) {
  const { t } = useTranslation("knowledgeBase")
  const picker = useRef<HTMLInputElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const { open, busy, items, run, select, close } = useKnowledgeDocumentUpload({ baseId, groupId })
  return (
    <>
      <Button ref={trigger} size="sm" disabled={busy || open} onClick={() => picker.current?.click()}>
        {t("documents.upload.action")}
      </Button>
      <input
        ref={picker}
        type="file"
        multiple
        accept={knowledgeDocumentFormats.join(",")}
        className="hidden"
        aria-label={t("documents.upload.action")}
        onChange={(event) => {
          select(Array.from(event.target.files ?? []))
          event.target.value = ""
        }}
      />
      <Dialog
        open={open}
        onOpenChange={(value) => {
          if (!value) close()
        }}
      >
        <DialogContent
          className="sm:max-w-xl"
          closeDisabled={busy}
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            trigger.current?.focus()
          }}
        >
          <DialogHeader>
            <DialogTitle>{t("documents.upload.action")}</DialogTitle>
            <DialogDescription>{t("documents.upload.description")}</DialogDescription>
          </DialogHeader>
          <div className="max-h-96 space-y-4 overflow-auto">
            {items.map((item, index) => (
              <div key={index} className="space-y-2">
                <div className="flex items-center justify-between gap-4 text-sm">
                  <span className="truncate" title={item.file.name}>
                    {item.file.name}
                  </span>
                  <span className="shrink-0 text-muted-foreground">{t(`documents.upload.${item.stage}`)}</span>
                </div>
                <progress
                  className="h-1.5 w-full accent-primary"
                  max={Math.max(1, item.file.size)}
                  value={item.stage === "saved" ? Math.max(1, item.file.size) : item.bytes}
                  aria-label={item.file.name}
                />
                <p className="text-xs text-muted-foreground">{formatFileSize(item.file.size)}</p>
              </div>
            ))}
          </div>
          <div className="flex justify-end gap-3 pt-5">
            {!busy && items.some((item) => item.stage === "failed") && (
              <Button onClick={() => void run()}>{t("documents.upload.retry")}</Button>
            )}
            <Button variant="outline" disabled={busy} onClick={close}>
              {t("documents.close")}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
