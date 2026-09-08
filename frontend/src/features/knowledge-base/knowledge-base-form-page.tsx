/** 企业知识库新建和编辑页。 */
import { useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"
import { toast } from "sonner"

import {
  KnowledgeBaseCategory,
  type KnowledgeBaseCategoryId,
  type KnowledgeBaseData,
  createKnowledgeBase,
  getKnowledgeBase,
  isApiError,
  updateKnowledgeBase,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import {
  createKnowledgeBaseSchema,
  knowledgeBaseDescriptionMaxLength,
  knowledgeBaseNameMaxLength,
  type KnowledgeBaseFormValues,
} from "@/features/knowledge-base/knowledge-base-schema"
import { useKnowledgeBaseContext } from "@/features/knowledge-base/knowledge-base-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 创建或编辑知识库。 */
export function KnowledgeBaseFormPage({
  mode,
}: {
  mode: "create" | "edit"
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const requestedCategory =
    searchParams.get("category") ===
    KnowledgeBaseCategory.KnowledgeBaseCategoryQA
      ? KnowledgeBaseCategory.KnowledgeBaseCategoryQA
      : KnowledgeBaseCategory.KnowledgeBaseCategoryStandard
  const { upsertKnowledgeBase } = useKnowledgeBaseContext()
  const { knowledgeBaseId = "" } = useParams()
  const invalidateResource = useResourceInvalidator()
  const [category, setCategory] =
    useState<KnowledgeBaseCategoryId>(requestedCategory)
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createKnowledgeBaseSchema({
        nameRequired: t("validation.nameRequired"),
        nameTooLong: t("validation.nameTooLong"),
        descriptionTooLong: t("validation.descriptionTooLong"),
      }),
    [t],
  )
  const form = useForm<KnowledgeBaseFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: "",
      description: "",
    },
  })
  useEffect(() => {
    if (mode !== "create") return
    setCategory(requestedCategory)
    form.reset({
      name: "",
      description: "",
    })
  }, [form, mode, requestedCategory])

  const {
    data: loadedKnowledgeBase,
    loading: detailLoading,
    refreshing: detailRefreshing,
    error: detailError,
    refresh: refreshKnowledgeBase,
  } = useResource(
    resourceKeys.knowledgeBase(knowledgeBaseId),
    () => getKnowledgeBase(knowledgeBaseId),
    { enabled: mode === "edit" },
  )
  const loading =
    mode === "edit" &&
    (detailLoading || (Boolean(detailError) && detailRefreshing))
  const loadError = mode === "edit" && Boolean(detailError) && !loading
  /** 详情就绪后回填知识库表单和派生状态。 */
  useEffect(() => {
    if (!loadedKnowledgeBase) return
    form.reset({
      name: loadedKnowledgeBase.name,
      description: loadedKnowledgeBase.description,
    })
    setCategory(loadedKnowledgeBase.category)
  }, [form, loadedKnowledgeBase])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 保存知识库。 */
  async function save(values: KnowledgeBaseFormValues) {
    try {
      let knowledgeBase: KnowledgeBaseData
      const input = { ...values, category }
      if (mode === "create") {
        knowledgeBase = await createKnowledgeBase(input)
      } else {
        knowledgeBase = await updateKnowledgeBase(knowledgeBaseId, input)
      }
      if (mode === "edit") {
        void invalidateResource(resourceKeys.knowledgeBase(knowledgeBaseId))
      }
      if (!mounted.current) return
      form.reset(values)
      upsertKnowledgeBase(knowledgeBase)
      toast.success(
        mode === "create"
          ? t("form.createSuccess")
          : t("form.updateSuccess"),
      )
      navigate(
        mode === "create"
          ? "/knowledge-bases"
          : `/knowledge-bases/${knowledgeBase.id}`,
      )
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) return
      console.warn("知识库保存失败", {
        knowledge_base_id: knowledgeBaseId,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, [
              "name",
              "category",
              "description",
            ])
          : t("form.saveError"),
      )
    }
  }

  const title =
    mode === "create" ? t("form.createTitle") : t("form.editTitle")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        {loading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("form.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refreshKnowledgeBase()}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : (
          <form
            className="w-full max-w-3xl space-y-9"
            onSubmit={form.handleSubmit(save)}
            noValidate
          >
            <FieldGroup>
              <Field>
                <FieldLabel>{t("form.category")}</FieldLabel>
                <p className="text-sm">
                  {category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
                    ? t("category.qa")
                    : t("category.standard")}
                </p>
              </Field>
              <FormInputField
                name="name"
                control={form.control}
                label={t("form.name")}
                autoFocus
                maxLength={knowledgeBaseNameMaxLength}
              />
              <Controller
                name="description"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel
                      htmlFor="knowledge-base-description"
                      required={false}
                    >
                      {t("form.description")}
                    </FieldLabel>
                    <Textarea
                      {...field}
                      id="knowledge-base-description"
                      rows={6}
                      maxLength={knowledgeBaseDescriptionMaxLength}
                      aria-invalid={fieldState.invalid}
                    />
                  </Field>
                )}
              />
            </FieldGroup>
            <div className="flex items-center gap-3">
              <Button
                type="submit"
                disabled={form.formState.isSubmitting}
              >
                {form.formState.isSubmitting
                  ? t("common:actions.saving")
                  : mode === "create"
                    ? t("common:actions.create")
                    : t("common:actions.save")}
              </Button>
              {mode === "create" ? (
                <Button type="button" variant="outline" asChild>
                  <Link to="/knowledge-bases">{t("common:actions.cancel")}</Link>
                </Button>
              ) : null}
            </div>
          </form>
        )}
      </PageContent>
    </div>
  )
}
