/** 本地问答的独立新增和编辑页面。 */
import { useEffect, useId, useMemo, useRef } from "react"
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
  isApiError,
  KnowledgeBaseCategory,
  updateKnowledgeQAEntry,
  type KnowledgeBaseData,
  type KnowledgeQAEntryData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { KnowledgeQAFeedback } from "@/features/knowledge-base/knowledge-qa-feedback"

/** 生成问答表单的必填校验。 */
function createQASchema(messages: {
  question: string
  answer: string
  group: string
}) {
  return z.object({
    question: z.string().trim().min(1, messages.question),
    similarQuestions: z.array(
      z.object({ id: z.string(), content: z.string() }),
    ),
    answer: z.string().trim().min(1, messages.answer),
    groupId: z.string().min(1, messages.group),
  })
}

type QAFormValues = z.infer<ReturnType<typeof createQASchema>>

/** 读取知识库和问答详情后展示编辑表单。 */
export function KnowledgeQAFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("knowledgeBase")
  const { knowledgeBaseId = "", groupId = "", entryId = "" } = useParams()
  const base = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    (signal) => getKnowledgeBase(knowledgeBaseId, signal),
  )
  const entry = useResource(
    resourceKeys.knowledgeQAEntry(knowledgeBaseId, entryId),
    (signal) => getKnowledgeQAEntry(knowledgeBaseId, entryId, signal),
    { enabled: mode === "edit", staleTime: 0 },
  )
  const error = base.error ?? entry.error
  const ready = base.data && (mode === "create" || entry.data)
  const supported =
    base.data?.category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA &&
    base.data.integrationConnectionId === ""
  return (
    <>
      <PageHeader
        title={t(mode === "create" ? "qa.createTitle" : "qa.editTitle")}
      />
      <PageContent>
        {error || !ready ? (
          <KnowledgeQAFeedback
            error={error}
            retry={() => void (base.error ? base.refresh() : entry.refresh())}
          />
        ) : !supported ? (
          <p className="text-sm text-muted-foreground">{t("qa.unsupported")}</p>
        ) : (
          <KnowledgeQAForm
            key={`${knowledgeBaseId}/${groupId}/${entryId}/${mode}`}
            knowledgeBase={base.data!}
            groupId={groupId}
            entry={mode === "edit" ? entry.data : undefined}
          />
        )}
      </PageContent>
    </>
  )
}

/** 保存完整问答并返回原分组的筛选和页码。 */
function KnowledgeQAForm({
  knowledgeBase,
  groupId,
  entry,
}: {
  knowledgeBase: KnowledgeBaseData
  groupId: string
  entry?: KnowledgeQAEntryData
}) {
  const { t } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const location = useLocation()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createQASchema({
        question: t("qa.questionRequired"),
        answer: t("qa.answerRequired"),
        group: t("qa.groupRequired"),
      }),
    [t],
  )
  const form = useForm<QAFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      question: entry?.question ?? "",
      answer: entry?.answer ?? "",
      similarQuestions: entry?.similarQuestions ?? [],
      groupId: entry?.groupId ?? groupId,
    },
  })
  const returnPath = `/knowledge-bases/${knowledgeBase.id}/groups/${groupId}/qa${location.search}`
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  useEffect(() => {
    if (!entry || form.formState.isDirty) return
    form.reset({
      question: entry.question,
      answer: entry.answer,
      similarQuestions: entry.similarQuestions,
      groupId: entry.groupId,
    })
  }, [entry, form])

  /** 提交表单并失效该知识库下的问答缓存。 */
  async function save(values: QAFormValues) {
    try {
      const saved = entry
        ? await updateKnowledgeQAEntry(knowledgeBase.id, entry.id, values)
        : await createKnowledgeQAEntry(knowledgeBase.id, values)
      await Promise.all([
        invalidate(resourceKeys.knowledgeQAEntries(knowledgeBase.id)),
        invalidate(resourceKeys.knowledgeQAEntry(knowledgeBase.id, saved.id)),
      ])
      if (!mounted.current) return
      const savedGroup = knowledgeBase.groups
        .flatMap((group) => [group, ...group.children])
        .find((group) => group.id === saved.groupId)
      toast.success(
        saved.groupId !== groupId
          ? t("qa.movedSuccess", {
              group: savedGroup?.isDefault
                ? t("group.default")
                : savedGroup?.name,
            })
          : t("qa.saveSuccess"),
      )
      navigate(returnPath, { replace: true })
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("qa.saveError"),
      )
    }
  }

  return (
    <form className="max-w-3xl space-y-9" onSubmit={form.handleSubmit(save)}>
      <QAFormFields
        control={form.control}
        disabled={form.formState.isSubmitting}
        knowledgeBase={knowledgeBase}
      />
      <div className="flex gap-3">
        <Button type="submit" disabled={form.formState.isSubmitting}>
          {t(form.formState.isSubmitting ? "qa.saving" : "qa.save")}
        </Button>
        <Button
          type="button"
          variant="outline"
          disabled={form.formState.isSubmitting}
          onClick={() => navigate(returnPath, { replace: true })}
        >
          {t("qa.cancel")}
        </Button>
      </div>
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
  const { t } = useTranslation("knowledgeBase")
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
            size="sm"
            disabled={disabled}
            aria-label={t("qa.removeSimilarQuestion", { number: index + 1 })}
            onClick={() => remove(index)}
          >
            {t("qa.remove")}
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

/** 展示问答正文和同一知识库中的分组选项。 */
function QAFormFields({
  control,
  disabled,
  knowledgeBase,
}: {
  control: Control<QAFormValues>
  disabled: boolean
  knowledgeBase: KnowledgeBaseData
}) {
  const { t } = useTranslation("knowledgeBase")
  const id = useId()
  const groups = knowledgeBase.groups.flatMap((group) => [
    group,
    ...group.children,
  ])
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
      <Controller
        control={control}
        name="groupId"
        render={({ field, fieldState }) => (
          <Field>
            <FieldLabel htmlFor={`${id}-group`} required>
              {t("qa.group")}
            </FieldLabel>
            <NativeSelect
              {...field}
              id={`${id}-group`}
              required
              aria-invalid={fieldState.invalid}
              disabled={disabled}
            >
              <option value="">{t("qa.selectGroup")}</option>
              {groups.map((group) => (
                <option key={group.id} value={group.id}>
                  {group.isDefault
                    ? t("group.default")
                    : group.parentId
                      ? `${groups.find((parent) => parent.id === group.parentId)?.name} / ${group.name}`
                      : group.name}
                </option>
              ))}
            </NativeSelect>
          </Field>
        )}
      />
    </FieldGroup>
  )
}
