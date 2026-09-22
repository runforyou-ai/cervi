/** 本地问答的独立新增和编辑页面。 */
import { useEffect, useId, useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import {
  Controller,
  useFieldArray,
  useForm,
  type Control,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate, useParams } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createKnowledgeQAEntry,
  getKnowledgeBase,
  getKnowledgeQAEntry,
  KnowledgeBaseCategory,
  updateKnowledgeQAEntry,
  type KnowledgeBaseData,
  type KnowledgeQAEntryData,
} from "@/api"
import { FormActions } from "@/components/form/form-actions"
import { FormInputField } from "@/components/form/form-input-field"
import { PageContent } from "@/components/page-content"
import { PageBackButton } from "@/components/page-back-button"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import { resourceKeys } from "@/hooks/resource-keys"
import { useFormSave } from "@/hooks/use-form-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

/** 生成问答表单的必填校验。 */
function createQASchema(messages: {
  question: string
  answer: string
}) {
  return z.object({
    question: z.string().trim().min(1, messages.question),
    similarQuestions: z.array(
      z.object({ id: z.string(), content: z.string() }),
    ),
    answer: z.string().trim().min(1, messages.answer),
  })
}

type QAFormValues = z.infer<ReturnType<typeof createQASchema>>

/** 读取知识库和问答详情后展示编辑表单。 */
export function KnowledgeQAFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("knowledgeBase")
  const { knowledgeBaseId = "", entryId = "" } = useParams()
  const location = useLocation()
  const base = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    (signal) => getKnowledgeBase(knowledgeBaseId, signal),
  )
  const entry = useResource(
    resourceKeys.knowledgeQAEntry(knowledgeBaseId, entryId),
    (signal) => getKnowledgeQAEntry(knowledgeBaseId, entryId, signal),
    { enabled: mode === "edit", staleTime: 0 },
  )
  const supported =
    base.data?.category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  return (
    <>
      <PageHeader
        title={t(mode === "create" ? "qa.createTitle" : "qa.editTitle")}
        description={t(
          mode === "create" ? "qa.createDescription" : "qa.editDescription",
        )}
      >
        {mode === "edit" ? (
          <PageBackButton
            to={`/knowledge-bases/${knowledgeBaseId}/qa${location.search}`}
          />
        ) : null}
      </PageHeader>
      <PageContent variant="form">
        <ResourceContent
          resources={mode === "edit" ? [base, entry] : [base]}
          errorMessage={t("qa.loadError")}
        >
          {!supported ? (
            <p className="text-sm text-muted-foreground">{t("qa.unsupported")}</p>
          ) : (
            <KnowledgeQAForm
              key={`${knowledgeBaseId}/${entryId}/${mode}`}
              knowledgeBase={base.data!}
              entry={mode === "edit" ? entry.data : undefined}
            />
          )}
        </ResourceContent>
      </PageContent>
    </>
  )
}

/** 保存完整问答并返回原列表的筛选和页码。 */
function KnowledgeQAForm({
  knowledgeBase,
  entry,
}: {
  knowledgeBase: KnowledgeBaseData
  entry?: KnowledgeQAEntryData
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const navigate = useNavigate()
  const location = useLocation()
  const invalidate = useResourceInvalidator()
  const schema = useMemo(
    () =>
      createQASchema({
        question: t("qa.questionRequired"),
        answer: t("qa.answerRequired"),
      }),
    [t],
  )
  const form = useForm<QAFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    // 编辑时离开字段即校验以便自动保存；新建时等提交再校验，避免原生校验把焦点锁在必填项上。
    mode: entry ? "onBlur" : "onSubmit",
    defaultValues: {
      question: entry?.question ?? "",
      answer: entry?.answer ?? "",
      similarQuestions: entry?.similarQuestions ?? [],
    },
  })
  const returnPath = `/knowledge-bases/${knowledgeBase.id}/qa${location.search}`
  useEffect(() => {
    if (!entry || form.formState.isDirty) return
    form.reset({
      question: entry.question,
      answer: entry.answer,
      similarQuestions: entry.similarQuestions,
    })
  }, [entry, form])

  /** 提交表单并失效该知识库下的问答缓存。 */
  // 编辑已有问答时边改边存，新建仍由底部按钮提交并跳回列表。
  const { submit } = useFormSave({
    form,
    schema,
    autoSave: Boolean(entry),
    save: async (values) => {
      const saved = entry
        ? await updateKnowledgeQAEntry(knowledgeBase.id, entry.id, values)
        : await createKnowledgeQAEntry(knowledgeBase.id, values)
      await Promise.all([
        invalidate(resourceKeys.knowledgeQAEntries(knowledgeBase.id)),
        invalidate(resourceKeys.knowledgeQAEntry(knowledgeBase.id, saved.id)),
      ])
      return saved
    },
    onSubmitted: () => {
      toast.success(t("qa.saveSuccess"))
      navigate(returnPath, { replace: true })
    },
    errorMessage: t("qa.saveError"),
    logLabel: "保存问答",
  })

  return (
    <form
      className="w-full space-y-9"
      onSubmit={form.handleSubmit(submit)}
      noValidate
    >
      <QAFormFields
        control={form.control}
        disabled={form.formState.isSubmitting}
      />
      {entry ? null : (
        <FormActions
          saving={form.formState.isSubmitting}
          onCancel={() => navigate(returnPath, { replace: true })}
        />
      )}
    </form>
  )
}

/** 管理带稳定业务编号的多条相似问题输入。 */
function SimilarQuestionFields({
  control,
  disabled,
}: {
  control: Control<QAFormValues>
  disabled: boolean
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const id = useId()
  const { fields, append, remove } = useFieldArray({
    control,
    name: "similarQuestions",
    keyName: "fieldKey",
  })
  return (
    <div className="space-y-4" role="group" aria-labelledby={`${id}-label`}>
      <div id={`${id}-label`} className="text-sm font-medium">
        {t("qa.similarQuestions")}
      </div>
      {fields.map((item, index) => (
        <div className="flex items-end gap-3" key={item.fieldKey}>
          <div className="min-w-0 flex-1">
            <FormInputField
              control={control}
              name={`similarQuestions.${index}.content`}
              required={false}
              id={`${id}-${item.fieldKey}`}
              label={t("qa.similarQuestion", { number: index + 1 })}
              disabled={disabled}
            />
          </div>
          <Button
            type="button"
            variant="outline"
            // 与表单页加高后的输入框同高，并排时底边对齐。
            className="h-11 rounded-lg"
            disabled={disabled}
            aria-label={t("qa.removeSimilarQuestion", { number: index + 1 })}
            onClick={() => remove(index)}
          >
            {t("common:actions.remove")}
          </Button>
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => append({ id: "", content: "" })}
      >
        {t("qa.addSimilarQuestion")}
      </Button>
    </div>
  )
}

/** 展示标准问题、相似问题和答案输入。 */
function QAFormFields({
  control,
  disabled,
}: {
  control: Control<QAFormValues>
  disabled: boolean
}) {
  const { t } = useTranslation("knowledgeBase")
  const id = useId()
  return (
    <FieldGroup>
      <FormInputField
        control={control}
        name="question"
        id={`${id}-question`}
        label={t("qa.question")}
        required
        disabled={disabled}
      />
      <SimilarQuestionFields control={control} disabled={disabled} />
      <Controller
        control={control}
        name="answer"
        render={({ field, fieldState }) => (
          <Field>
            <FieldLabel htmlFor={`${id}-answer`} required>
              {t("qa.answer")}
            </FieldLabel>
            <Textarea
              {...field}
              id={`${id}-answer`}
              required
              rows={12}
              aria-invalid={fieldState.invalid}
              disabled={disabled}
            />
          </Field>
        )}
      />
    </FieldGroup>
  )
}
