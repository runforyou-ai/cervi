/** 保留原生文本编辑能力，并为聊天输入框绘制固定周期的光标。 */
import { useImperativeHandle, useLayoutEffect, useRef } from "react"

import { Textarea } from "@/components/ui/textarea"
import { cn } from "@/lib/utils"

const mirroredTextProperties = [
  "font-family", "font-size", "font-weight", "font-style", "font-stretch",
  "font-variant", "font-feature-settings", "font-variation-settings",
  "line-height", "letter-spacing", "word-spacing", "text-align",
  "text-indent", "text-transform", "text-rendering", "tab-size",
  "direction", "white-space", "overflow-wrap", "word-break",
  "padding-top", "padding-right", "padding-bottom", "padding-left",
]

/** 根据输入框的排版和滚动位置同步镜像中的插入点。 */
function updateComposerCaret(
  input: HTMLTextAreaElement,
  mirror: HTMLDivElement,
  caret: HTMLSpanElement,
) {
  caret.hidden = document.activeElement !== input || !document.hasFocus() ||
    input.disabled || input.readOnly || input.selectionStart !== input.selectionEnd
  if (caret.hidden) return

  const style = window.getComputedStyle(input)
  for (const property of mirroredTextProperties) {
    mirror.style.setProperty(property, style.getPropertyValue(property))
  }
  mirror.style.width = `${input.clientWidth}px`
  // 末尾零宽字符让空输入框和末尾换行也具有可测量的插入点。
  mirror.textContent = `${input.value}\u200b`
  const range = document.createRange()
  range.setStart(mirror.firstChild!, input.selectionStart)
  range.collapse(true)
  const position = range.getBoundingClientRect()
  const origin = mirror.getBoundingClientRect()
  caret.style.left = `${position.left - origin.left + input.clientLeft - input.scrollLeft}px`
  caret.style.top = `${position.top - origin.top + input.clientTop - input.scrollTop}px`
  caret.style.height = `${position.height}px`
}

/** 展示每秒完成一次明暗切换的聊天输入框。 */
export function ConversationComposerInput({
  ref,
  className,
  ...props
}: React.ComponentProps<typeof Textarea>) {
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const mirrorRef = useRef<HTMLDivElement>(null)
  const caretRef = useRef<HTMLSpanElement>(null)
  const updateRef = useRef<(() => void) | null>(null)
  useImperativeHandle(ref, () => inputRef.current!, [])

  useLayoutEffect(() => {
    const input = inputRef.current!
    const mirror = mirrorRef.current!
    const caret = caretRef.current!
    let frame: number | null = null

    /** 在浏览器完成输入和选区更新后合并刷新光标位置。 */
    function scheduleUpdate() {
      if (frame !== null) return
      frame = window.requestAnimationFrame(() => {
        frame = null
        updateComposerCaret(input, mirror, caret)
      })
    }

    const inputEvents = [
      "focus", "blur", "input", "select", "scroll",
      "compositionupdate", "compositionend",
    ]
    for (const event of inputEvents) input.addEventListener(event, scheduleUpdate)
    document.addEventListener("selectionchange", scheduleUpdate)
    document.addEventListener("visibilitychange", scheduleUpdate)
    document.fonts.addEventListener("loadingdone", scheduleUpdate)
    window.addEventListener("focus", scheduleUpdate)
    window.addEventListener("blur", scheduleUpdate)
    const observer = new ResizeObserver(scheduleUpdate)
    observer.observe(input)
    updateRef.current = scheduleUpdate
    scheduleUpdate()

    return () => {
      updateRef.current = null
      if (frame !== null) window.cancelAnimationFrame(frame)
      observer.disconnect()
      for (const event of inputEvents) input.removeEventListener(event, scheduleUpdate)
      document.removeEventListener("selectionchange", scheduleUpdate)
      document.removeEventListener("visibilitychange", scheduleUpdate)
      document.fonts.removeEventListener("loadingdone", scheduleUpdate)
      window.removeEventListener("focus", scheduleUpdate)
      window.removeEventListener("blur", scheduleUpdate)
    }
  }, [])

  // 同步表单重置、失败重试和插入 @ 提醒等由程序更新的正文。
  useLayoutEffect(() => {
    updateRef.current?.()
  })

  return (
    <div className="relative">
      <Textarea {...props} ref={inputRef} className={cn(className, "caret-transparent")} />
      <div aria-hidden="true" className="pointer-events-none absolute inset-0 overflow-hidden">
        <div ref={mirrorRef} className="invisible absolute top-0 left-0 box-border" />
        <span ref={caretRef} hidden className="conversation-composer-caret absolute w-0.5 bg-primary" />
      </div>
    </div>
  )
}
