/** 外部知识库检索测试侧栏。 */
import { useEffect, useMemo, useRef, useState, type RefObject } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { z } from "zod"

import {
  isApiError,
  retrieveKnowledgeBase,
  type KnowledgeRetrievalResultData,
} from "@/api"
import { SelectableText } from "@/components/selectable-text"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const retrievalQueryMaxLength = 250

/** 显示检索输入和按返回顺序排列的命中分段。 */
export function KnowledgeRetrievalSheet({
  open,
  onOpenChange,
  knowledgeBaseId,
  triggerRef,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  knowledgeBaseId: string
  triggerRef: RefObject<HTMLButtonElement | null>
}) {
  const { t, i18n } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const mounted = useRef(true)
  const requestSequence = useRef(0)
  const [result, setResult] = useState<KnowledgeRetrievalResultData | null>(
    null,
  )
  const [requestError, setRequestError] = useState<unknown>(null)
  const [loading, setLoading] = useState(false)
  const schema = useMemo(
    () =>
      z.object({
        query: z
          .string()
          .trim()
          .min(1, t("retrieval.validation.required"))
          .max(
            retrievalQueryMaxLength,
            t("retrieval.validation.tooLong", {
              count: retrievalQueryMaxLength,
            }),
          ),
      }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      query: "",
    },
  })
  const scoreFormatter = useMemo(
    () =>
      new Intl.NumberFormat(i18n.resolvedLanguage, {
        maximumFractionDigits: 3,
      }),
    [i18n.resolvedLanguage],
  )

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
      requestSequence.current += 1
    }
  }, [])

  /** 发起检索，并且只采纳最后一次请求的结果。 */
  async function retrieve(values: z.infer<typeof schema>) {
    const sequence = ++requestSequence.current
    setLoading(true)
    setRequestError(null)
    setResult(null)
    try {
      const nextResult = await retrieveKnowledgeBase(knowledgeBaseId, values)
      if (!mounted.current || sequence !== requestSequence.current) return
      setResult(nextResult)
    } catch (error) {
      if (!mounted.current || sequence !== requestSequence.current) return
      if (recoverSession(error, navigate)) return
      setRequestError(error)
    } finally {
      if (mounted.current && sequence === requestSequence.current) {
        setLoading(false)
      }
    }
  }

  const records = result?.records ?? []

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        id="knowledge-retrieval-sheet"
        className="w-full gap-0 p-0 sm:max-w-xl"
        onOpenAutoFocus={(event) => {
          event.preventDefault()
          form.setFocus("query")
        }}
        onCloseAutoFocus={(event) => {
          event.preventDefault()
          triggerRef.current?.focus()
        }}
      >
        <SheetHeader className="px-6 pt-6 pr-12 pb-0">
          <SheetTitle>{t("retrieval.title")}</SheetTitle>
          <SheetDescription>{t("retrieval.description")}</SheetDescription>
        </SheetHeader>
        <form
          className="shrink-0 space-y-9 px-6 pt-4 pb-6"
          onSubmit={form.handleSubmit(retrieve)}
          noValidate
        >
          <FieldGroup className="gap-5">
            <Controller
              name="query"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor={field.name} required>
                    {t("retrieval.query")}
                  </FieldLabel>
                  <Input
                    {...field}
                    id={field.name}
                    autoComplete="off"
                    maxLength={retrievalQueryMaxLength}
                    required
                    aria-invalid={fieldState.invalid}
                  />
                </Field>
              )}
            />
          </FieldGroup>
          <Button type="submit">{t("retrieval.submit")}</Button>
        </form>
        <div
          className="min-h-0 flex-1 overflow-y-auto border-t px-6 py-5"
          aria-live="polite"
          aria-busy={loading}
        >
          {loading ? (
            <p className="py-12 text-center text-sm text-muted-foreground">
              {t("retrieval.loading")}
            </p>
          ) : requestError ? (
            <p className="py-12 text-center text-sm text-destructive">
              {isApiError(requestError)
                ? requestError.reason ||
                  apiErrorMessage(requestError, ["query"])
                : t("retrieval.error")}
            </p>
          ) : !result ? (
            <p className="py-12 text-center text-sm text-muted-foreground">
              {t("retrieval.initial")}
            </p>
          ) : records.length === 0 ? (
            <p className="py-12 text-center text-sm text-muted-foreground">
              {t("retrieval.empty")}
            </p>
          ) : (
            <div>
              <p className="mb-1 text-sm text-muted-foreground">
                {t("retrieval.resultCount", { count: records.length })}
              </p>
              <ol className="divide-y">
                {records.map((record, index) => (
                  <li key={`${record.segmentId}-${index}`} className="py-5">
                    <div className="flex items-start gap-3">
                      <span className="w-5 shrink-0 text-right text-sm font-medium tabular-nums">
                        {index + 1}
                      </span>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                          <SelectableText className="break-all text-sm font-medium">
                            {record.documentName || record.documentId}
                          </SelectableText>
                          <span className="text-xs text-muted-foreground">
                            {t("retrieval.position", {
                              position: record.position,
                            })}
                          </span>
                        </div>
                        <SelectableText className="mt-3 block whitespace-pre-wrap break-words text-sm leading-6">
                          {record.content || "—"}
                        </SelectableText>
                        {record.answer?.trim() ? (
                          <div className="mt-4 border-l-2 pl-3">
                            <p className="mb-1 text-xs font-medium text-muted-foreground">
                              {t("retrieval.answer")}
                            </p>
                            <SelectableText className="block whitespace-pre-wrap break-words text-sm leading-6">
                              {record.answer}
                            </SelectableText>
                          </div>
                        ) : null}
                      </div>
                      <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                        {record.score == null
                          ? t("retrieval.scoreUnavailable")
                          : t("retrieval.score", {
                              score: scoreFormatter.format(record.score),
                            })}
                      </span>
                    </div>
                  </li>
                ))}
              </ol>
            </div>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
