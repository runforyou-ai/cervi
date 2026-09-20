/** 在线文档的独立新增和编辑页面。 */
import { useEffect, useId, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createKnowledgeTextDocument,
  getKnowledgeBase,
  getKnowledgeDocumentContent,
  isApiError,
  KnowledgeBaseCategory,
  KnowledgeDocumentSourceKind,
  updateKnowledgeDocumentContent,
  type KnowledgeDocumentContentData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { useAutoSave } from "@/hooks/use-auto-save"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { KnowledgeDocumentEditor } from "./knowledge-document-editor"
import { KnowledgeQAFeedback } from "./knowledge-qa-feedback"

export const knowledgeDocumentTitleMaxLength = 120

/** 生成在线文档表单的必填与长度校验。 */
function createDocumentSchema(messages: {
  title: string
  titleTooLong: string
  content: string
}) {
  return z.object({
    title: z
      .string()
      .trim()
      .min(1, messages.title)
      .max(knowledgeDocumentTitleMaxLength, messages.titleTooLong),
    content: z.string().trim().min(1, messages.content),
  })
}

type DocumentFormValues = z.infer<ReturnType<typeof createDocumentSchema>>

/** 读取知识库和文档正文后展示编辑表单。 */
export function KnowledgeDocumentFormPage({
  mode,
}: {
  mode: "create" | "edit"
}) {
  const { t } = useTranslation("knowledgeBase")
  const {
    knowledgeBaseId = "",
    groupId = "",
    documentId = "",
  } = useParams()
  const base = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    (signal) => getKnowledgeBase(knowledgeBaseId, signal),
  )
  const content = useResource(
    resourceKeys.knowledgeDocumentContent(knowledgeBaseId, documentId),
    (signal) => getKnowledgeDocumentContent(knowledgeBaseId, documentId, signal),
    { enabled: mode === "edit", staleTime: 0 },
  )
  const error = base.error ?? content.error
  const ready = base.data && (mode === "create" || content.data)
  const supported =
    base.data?.category ===
      KnowledgeBaseCategory.KnowledgeBaseCategoryStandard &&
    (mode === "create" ||
      content.data?.document.sourceKind ===
        KnowledgeDocumentSourceKind.KnowledgeDocumentSourceText)
  return (
    <>
      <PageHeader
        title={t(
          mode === "create" ? "documents.createTitle" : "documents.editTitle",
        )}
      />
      <PageContent>
        {error || !ready ? (
          <KnowledgeQAFeedback
            error={error}
            retry={() => void (base.error ? base.refresh() : content.refresh())}
          />
        ) : !supported ? (
          <p className="text-sm text-muted-foreground">
            {t("documents.sourceUnsupported")}
          </p>
        ) : (
          <KnowledgeDocumentForm
            key={`${knowledgeBaseId}/${groupId}/${documentId}/${mode}`}
            baseId={knowledgeBaseId}
            groupId={groupId}
            stored={mode === "edit" ? content.data : undefined}
          />
        )}
      </PageContent>
    </>
  )
}

/** 保存在线文档并返回原分组的筛选和页码。 */
function KnowledgeDocumentForm({
  baseId,
  groupId,
  stored,
}: {
  baseId: string
  groupId: string
  stored?: KnowledgeDocumentContentData
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const navigate = useNavigate()
  const location = useLocation()
  const invalidate = useResourceInvalidator()
  const id = useId()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createDocumentSchema({
        title: t("documents.titleRequired"),
        titleTooLong: t("documents.titleTooLong"),
        content: t("documents.contentRequired"),
      }),
    [t],
  )
  const form = useForm<DocumentFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onBlur",
    defaultValues: {
      title: stored?.document.name ?? "",
      content: stored?.content ?? "",
    },
  })
  const returnPath = `/knowledge-bases/${baseId}/groups/${groupId}/documents${location.search}`
  // 表单未编辑时跟随最新读取到的名称与正文，编辑器按内容版本重建。
  const [contentVersion, setContentVersion] = useState(0)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  useEffect(() => {
    if (!stored || form.formState.isDirty) return
    const values = form.getValues()
    if (values.title === stored.document.name && values.content === stored.content)
      return
    form.reset({ title: stored.document.name, content: stored.content })
    setContentVersion((version) => version + 1)
  }, [stored, form])

  /** 提交表单并失效该文档的列表、详情和正文缓存。 */
  // 编辑已有文档时边改边存，新建仍由底部按钮提交并跳回列表。
  const markSaved = useAutoSave({
    form,
    schema,
    enabled: Boolean(stored),
    save: (values) => save(values, true),
  })

  async function save(values: DocumentFormValues, autoSaved = false) {
    try {
      const saved = stored
        ? await updateKnowledgeDocumentContent(baseId, stored.document.id, values)
        : await createKnowledgeTextDocument(baseId, { ...values, groupId })
      await Promise.all([
        invalidate(resourceKeys.knowledgeDocuments(baseId)),
        invalidate(resourceKeys.knowledgeDocument(baseId, saved.id)),
        invalidate(resourceKeys.knowledgeDocumentContent(baseId, saved.id)),
      ])
      if (!mounted.current) return
      if (autoSaved) {
        markSaved(values)
        return
      }
      toast.success(t("documents.saveSuccess"))
      navigate(returnPath, { replace: true })
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("documents.saveError"),
      )
    }
  }

  const disabled = form.formState.isSubmitting
  return (
    <form className="max-w-3xl space-y-9" onSubmit={form.handleSubmit((values) => save(values))}>
      <FieldGroup>
        <FormInputField
          control={form.control}
          name="title"
          id={`${id}-title`}
          label={t("documents.name")}
          required
          disabled={disabled}
        />
        <Controller
          control={form.control}
          name="content"
          render={({ field }) => (
            <Field>
              <FieldLabel htmlFor={`${id}-content`} required>
                {t("documents.content")}
              </FieldLabel>
              <KnowledgeDocumentEditor
                key={contentVersion}
                id={`${id}-content`}
                value={field.value}
                disabled={disabled}
                onChange={field.onChange}
                fieldRef={field.ref}
              />
            </Field>
          )}
        />
      </FieldGroup>
      {stored ? null : (
        <div className="flex gap-3">
          <Button type="submit" disabled={disabled}>
            {t(disabled ? "common:actions.saving" : "common:actions.save")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            onClick={() => navigate(returnPath, { replace: true })}
          >
            {t("common:actions.cancel")}
          </Button>
        </div>
      )}
    </form>
  )
}
