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
  listAIProviders,
  isApiError,
  updateKnowledgeBase,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { Textarea } from "@/components/ui/textarea"
import {
  createKnowledgeBaseSchema,
  knowledgeBaseDescriptionMaxLength,
  knowledgeBaseNameMaxLength,
  type KnowledgeBaseFormValues,
} from "@/features/knowledge-base/knowledge-base-schema"
import { KnowledgeBaseSettingsFields } from "./knowledge-base-settings-fields"
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
  const saveButton = useRef<HTMLButtonElement>(null)
  const [confirmModelChange, setConfirmModelChange] = useState(false)
  const isQA = category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  const schema = useMemo(
    () =>
      createKnowledgeBaseSchema({
        nameRequired: t("validation.nameRequired"),
        nameTooLong: t("validation.nameTooLong"),
        descriptionTooLong: t("validation.descriptionTooLong"),
        embeddingModelRequired: t("validation.embeddingModelRequired"),
        embeddingDimensionInvalid: t("validation.embeddingDimensionInvalid"),
        chunkLengthInvalid: t("validation.chunkLengthInvalid"),
        chunkOverlapInvalid: t("validation.chunkOverlapInvalid"),
        retrievalCountInvalid: t("validation.retrievalCountInvalid"),
      }, isQA),
    [t, isQA],
  )
  const form = useForm<KnowledgeBaseFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: "",
      description: "",
      embeddingModel: "",
      embeddingDimension: "",
      chunkLength: "512",
      chunkOverlap: "50",
      retrievalCount: "3",
      rerankModel: "",
    },
  })
  useEffect(() => {
    if (mode !== "create") return
    setCategory(requestedCategory)
    form.reset({
      name: "",
      description: "",
      embeddingModel: "",
      embeddingDimension: "",
      chunkLength: "512",
      chunkOverlap: "50",
      retrievalCount: "3",
      rerankModel: "",
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
  const providers = useResource(resourceKeys.aiProviders(), () => listAIProviders(), { staleTime: 0 })
  const loading = providers.loading || (Boolean(providers.error) && providers.refreshing) || (
    mode === "edit" &&
    (detailLoading || (Boolean(detailError) && detailRefreshing))
  )
  const loadError = !loading && (Boolean(providers.error) || (mode === "edit" && Boolean(detailError)))
  /** 详情就绪后回填知识库表单和派生状态。 */
  useEffect(() => {
    if (!loadedKnowledgeBase) return
    form.reset({
      name: loadedKnowledgeBase.name,
      description: loadedKnowledgeBase.description,
      embeddingModel: loadedKnowledgeBase.embeddingProviderId ? JSON.stringify([loadedKnowledgeBase.embeddingProviderId, loadedKnowledgeBase.embeddingModelIdentifier]) : "",
      embeddingDimension: String(loadedKnowledgeBase.embeddingDimension),
      chunkLength: loadedKnowledgeBase.chunkLength === null ? "" : String(loadedKnowledgeBase.chunkLength),
      chunkOverlap: loadedKnowledgeBase.chunkOverlap === null ? "" : String(loadedKnowledgeBase.chunkOverlap),
      retrievalCount: String(loadedKnowledgeBase.retrievalCount),
      rerankModel: loadedKnowledgeBase.rerankProviderId ? JSON.stringify([loadedKnowledgeBase.rerankProviderId, loadedKnowledgeBase.rerankModelIdentifier]) : "",
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
      // 保存供应商与模型标识，并按知识库类型提交分段配置。
      const [embeddingProviderId, embeddingModelIdentifier] = JSON.parse(values.embeddingModel) as [string, string]
      const [rerankProviderId, rerankModelIdentifier] = values.rerankModel ? JSON.parse(values.rerankModel) as [string, string] : ["", ""]
      const input = {
        name: values.name,
        description: values.description,
        category,
        embeddingProviderId,
        embeddingModelIdentifier,
        embeddingDimension: Number(values.embeddingDimension),
        chunkLength: isQA ? null : Number(values.chunkLength),
        chunkOverlap: isQA ? null : Number(values.chunkOverlap),
        retrievalCount: Number(values.retrievalCount),
        rerankProviderId,
        rerankModelIdentifier,
      }
      if (mode === "create") {
        knowledgeBase = await createKnowledgeBase(input)
      } else {
        knowledgeBase = await updateKnowledgeBase(knowledgeBaseId, input)
      }
      if (mode === "edit") {
        void invalidateResource(resourceKeys.knowledgeBase(knowledgeBaseId))
      }
      if (!mounted.current) return
      setConfirmModelChange(false)
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
              "embeddingModelIdentifier",
              "embeddingDimension",
              "chunkLength",
              "chunkOverlap",
              "retrievalCount",
              "rerankModelIdentifier",
            ])
          : t("form.saveError"),
      )
    }
  }

  /** 保存前确认编辑页中的向量模型切换。 */
  async function submit(values: KnowledgeBaseFormValues) {
    if (mode === "edit" && loadedKnowledgeBase && values.embeddingModel !== JSON.stringify([loadedKnowledgeBase.embeddingProviderId, loadedKnowledgeBase.embeddingModelIdentifier])) {
      setConfirmModelChange(true)
      return
    }
    await save(values)
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
              onClick={() => {
                // 一次重试本页所有读取失败的数据。
                if (providers.error) void providers.refresh()
                if (mode === "edit" && detailError) void refreshKnowledgeBase()
              }}
            >
              {t("common:actions.retry")}
            </Button>
          </div>
        ) : (
          <form
            className="w-full max-w-3xl space-y-9"
            onSubmit={form.handleSubmit(submit)}
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
              <KnowledgeBaseSettingsFields control={form.control} isQA={isQA} providers={providers.data?.providers ?? []} />
            </FieldGroup>
            <div className="flex items-center gap-3">
              <Button
                ref={saveButton}
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
      <AlertDialog open={confirmModelChange} onOpenChange={(open) => !form.formState.isSubmitting && setConfirmModelChange(open)}>
        <AlertDialogContent onCloseAutoFocus={(event) => {
          // 确认框关闭后恢复保存按钮的键盘焦点。
          event.preventDefault()
          saveButton.current?.focus()
        }}>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("form.changeEmbeddingTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("form.changeEmbeddingDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={form.formState.isSubmitting}>{t("common:actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={form.formState.isSubmitting}
              onClick={(event) => {
                event.preventDefault()
                void form.handleSubmit(save)()
              }}
            >
              {form.formState.isSubmitting ? t("common:actions.saving") : t("form.confirmSave")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
