/** 展示会话消息输入区、工具栏和提及候选。 */
import { ArrowUpIcon, LoaderCircleIcon, MicIcon, StickyNoteIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { ConversationType, MessageVisibility } from "@/api"
import { IconTooltip } from "@/components/icon-tooltip"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { CustomerReplyAssistant } from "./customer-reply-assistant"
import { composerToolClass } from "./composer-tool"
import { resizeComposerInput } from "./composer-input"
import { ComposerAttachmentTool, ComposerEmojiPicker, ComposerMentionOverlay, ComposerReplyPreview } from "./conversation-composer-parts"
import { reconcileMentionAllToken } from "@/lib/mention-token"
import { cn } from "@/lib/utils"
import { useConversationComposer } from "./use-conversation-composer"
import type { ConversationComposerProps } from "./conversation-composer-types"

/** 展示并提交成员会话文本编辑区。 */
export function ConversationComposer(props: ConversationComposerProps) {
  const { t } = useTranslation("inbox")
  const { conversationID, conversationType, currentIdentityID = "", replyTo = null, onReplyToChange, onVisibilityChange,
    attachmentTargetIdentityID, attachmentAgentDraft, onBeforeSend, onAttachmentConversationCreated,
  } = props
  const {
    form, inputRef, inputID, bodyValue, isBodyEmpty, isSubmitting, internalNote, disabledReason,
    mobile, groupConversation, customerAttachmentSupported, customerAttachmentByteLimit, customerAttachmentCaptionLimit,
    mentionCandidates, activeMentionIndex, mentionQuery, noteMentionHint, selectMention, switchVisibility,
    setMentionAllToken, typingReport, reconcileMentions, updateMentionQuery, setMentionQuery,
    insertEmoji, applyReplySuggestion, submitFromKeyboard, showSubmitting, send,
  } = useConversationComposer(props)
  const bodyField = form.register("body")
  // 客户会话在输入区工具栏提供 AI 写回复入口，不可对客发送时保留显示并禁用。
  const replyAssistant =
    conversationType === ConversationType.ConversationTypeCustomer &&
    conversationID &&
    !internalNote ? (
      <CustomerReplyAssistant
        mobile={mobile}
        conversationID={conversationID}
        currentIdentityID={currentIdentityID}
        draft={bodyValue}
        replyToMessageID={replyTo && !replyTo.deleted ? replyTo.id : ""}
        disabled={isSubmitting || Boolean(disabledReason)}
        onApply={applyReplySuggestion}
      />
    ) : null

  // 输入内容后语音入口换成发送按钮，并保持到本次发送结束。
  const showSend = !isBodyEmpty || isSubmitting
  const bodyInput = (
    <Textarea
      {...bodyField}
      ref={(input) => {
        bodyField.ref(input)
        inputRef.current = input
      }}
      id={inputID}
      disabled={isSubmitting}
      readOnly={Boolean(disabledReason)}
      rows={1}
      aria-label={t(internalNote ? "internalNoteLabel" : "replyLabel")}
      aria-describedby={disabledReason ? `${inputID}-reason` : undefined}
      aria-invalid={form.formState.errors.body ? true : undefined}
      className={cn(
        // 行高贴近字体自然行高，避免换行前后光标高度跳变；上下内边距之和保持 16px，下伸部留空由上多下少补偿。
        "max-h-[200px] min-h-10 min-w-0 flex-1 resize-none rounded-none border-0 bg-transparent px-0.5 pt-[11px] pb-[9px] leading-5 shadow-none focus-visible:ring-0 dark:bg-transparent",
        // 正文各端统一 16px，移动端聚焦时不缩放，与访客端一致。
        "md:text-[1rem]",
      )}
      onInput={(event) => {
        resizeComposerInput(event.currentTarget)
      }}
      onChange={(event) => {
        const previousBody = form.getValues("body")
        const { value, selectionStart } = event.currentTarget
        setMentionAllToken((current) =>
          reconcileMentionAllToken(
            current, previousBody, value, selectionStart,
          ),
        )
        bodyField.onChange(event)
        typingReport.input(event.currentTarget.value)
        reconcileMentions(event.currentTarget.value)
        updateMentionQuery(
          event.currentTarget.value,
          event.currentTarget.selectionStart,
        )
      }}
      onClick={(event) =>
        updateMentionQuery(
          event.currentTarget.value,
          event.currentTarget.selectionStart,
        )
      }
      onBlur={(event) => {
        bodyField.onBlur(event)
        setMentionQuery(null)
      }}
      onKeyDown={submitFromKeyboard}
    />
  )

  return (
    <form
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      className="shrink-0 bg-background"
      onSubmit={form.handleSubmit(send)}
      noValidate
    >
      <div className="relative">
        <ComposerMentionOverlay
          candidates={mentionCandidates}
          activeIndex={activeMentionIndex}
          groupConversation={groupConversation}
          showCandidates={!disabledReason && Boolean(mentionQuery) && mentionCandidates.length > 0}
          showNoteHint={!disabledReason && noteMentionHint}
          onSelect={selectMention}
          onSwitchToNote={() =>
            switchVisibility(MessageVisibility.MessageVisibilityInternalOnly)
          }
        />
        <div
          className={cn(
            internalNote
              ? "bg-note/60"
              : "bg-background",
          )}
        >
          {replyTo ? (
            <ComposerReplyPreview
              replyTo={replyTo}
              disabled={Boolean(disabledReason)}
              onCancel={() => onReplyToChange?.(null)}
            />
          ) : null}
          {/* 输入区整体铺底色，正文单独用白底并与上下边缘留出间距。 */}
          <div className="flex items-end gap-2 bg-foreground/[0.03] px-2 py-1">
            <div className="mb-1.5 flex items-end">
              {onVisibilityChange ? (
                <IconTooltip label={t("composerModeNote")}>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-sm"
                      aria-pressed={internalNote}
                      className={cn(
                        composerToolClass,
                        internalNote &&
                          "bg-note-active text-note-active-foreground hover:bg-note-active hover:text-note-active-foreground dark:hover:bg-note-active",
                      )}
                      aria-label={t("composerModeNote")}
                      onClick={() =>
                        switchVisibility(
                          internalNote
                            ? MessageVisibility.MessageVisibilityCustomerVisible
                            : MessageVisibility.MessageVisibilityInternalOnly,
                        )
                      }
                    >
                      <StickyNoteIcon />
                    </Button>
                </IconTooltip>
              ) : null}
              <ComposerAttachmentTool
                conversationID={conversationID}
                conversationType={conversationType}
                customerEnabled={customerAttachmentSupported && !internalNote}
                targetIdentityID={attachmentTargetIdentityID}
                agentDraft={attachmentAgentDraft}
                byteLimit={customerAttachmentByteLimit}
                captionLimit={customerAttachmentCaptionLimit}
                replyTo={replyTo}
                disabled={isSubmitting || Boolean(disabledReason)}
                onSent={() => onReplyToChange?.(null)}
                onBeforeSend={onBeforeSend}
                onCreated={onAttachmentConversationCreated}
              />
            </div>
            <div className="flex min-w-0 flex-1 items-end rounded-md bg-background px-2">
              {disabledReason ? (
                <p
                  id={`${inputID}-reason`}
                  className="min-w-0 flex-1 truncate py-[10px] text-xs leading-5 text-muted-foreground"
                >
                  {disabledReason}
                </p>
              ) : (
                bodyInput
              )}
            </div>
            <div className="mb-1.5 flex items-end">
              <ComposerEmojiPicker
                disabled={isSubmitting || Boolean(disabledReason)}
                inputRef={inputRef}
                onInsert={insertEmoji}
              />
              {replyAssistant}
              {showSend ? (
                <IconTooltip label={t(internalNote ? "internalNoteSave" : "messageSend")}>
                  <Button
                    type="submit"
                    variant="ghost"
                    size="icon-sm"
                    className={cn(composerToolClass, "group")}
                    disabled={isSubmitting || Boolean(disabledReason) || isBodyEmpty || replyTo?.deleted}
                    aria-label={t(internalNote ? "internalNoteSave" : "messageSend")}
                  >
                    {/* 28px 按钮内绘制 24px 主色实心圆。 */}
                    <span className="flex size-6 items-center justify-center rounded-full bg-primary text-primary-foreground transition-colors group-hover:bg-primary/90">
                      {showSubmitting ? <LoaderCircleIcon className="size-3.5 animate-spin" /> : <ArrowUpIcon className="size-3.5" />}
                    </span>
                  </Button>
                </IconTooltip>
              ) : (
                <IconTooltip label={t("voiceMessage")}>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className={composerToolClass}
                    disabled
                    aria-label={t("voiceMessage")}
                  >
                    <MicIcon />
                  </Button>
                </IconTooltip>
              )}
            </div>
          </div>
        </div>
      </div>
    </form>
  )
}
