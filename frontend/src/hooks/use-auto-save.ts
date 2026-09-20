/** 表单值变化后的自动保存。 */
import { useEffect, useRef } from "react"
import type { FieldValues, UseFormReturn } from "react-hook-form"
import type { ZodType } from "zod"

/**
 * 启用时，值变化并通过校验后按延迟提交，与上次保存结果相同时跳过；
 * 保存期间产生的改动在本次完成后补发一次。校验不通过时静默等待，
 * 由字段自身的失焦校验提示用户。
 */
export function useAutoSave<T extends FieldValues>({
  form,
  schema,
  save,
  enabled = true,
  delay = 600,
}: {
  form: UseFormReturn<T>
  schema: ZodType<T>
  save: (values: T) => Promise<void>
  enabled?: boolean
  delay?: number
}) {
  const latest = useRef({ schema, save })
  latest.current = { schema, save }
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  const saving = useRef(false)
  const queued = useRef(false)
  const saved = useRef(JSON.stringify(form.getValues()))

  useEffect(() => {
    if (!enabled) return

    async function flush() {
      if (saving.current) {
        queued.current = true
        return
      }
      const parsed = latest.current.schema.safeParse(form.getValues())
      if (!parsed.success) return
      const serialized = JSON.stringify(parsed.data)
      if (serialized === saved.current) return

      saving.current = true
      try {
        await latest.current.save(parsed.data)
        saved.current = serialized
      } finally {
        saving.current = false
        if (queued.current) {
          queued.current = false
          void flush()
        }
      }
    }

    const subscription = form.watch(() => {
      clearTimeout(timer.current)
      timer.current = setTimeout(() => void flush(), delay)
    })
    return () => {
      clearTimeout(timer.current)
      subscription.unsubscribe()
    }
  }, [form, enabled, delay])

  /** 让外部在服务端返回新值后同步基准，避免回填触发再次保存。 */
  return function markSaved(values: T) {
    saved.current = JSON.stringify(values)
  }
}
