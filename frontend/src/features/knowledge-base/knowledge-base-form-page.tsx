/** 企业知识库新建和编辑页。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
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
import { PageBack } from "@/components/page-back"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { Textarea } from "@/components/ui/textarea"
import {
  createKnowledgeBaseSchema,
  knowledgeBaseDescriptionMaxLength,
  knowledgeBaseNameMaxLength,
  type KnowledgeBaseFormValues,
} from "@/features/knowledge-base/knowledge-base-schema"
import { useKnowledgeBaseContext } from "@/features/knowledge-base/knowledge-base-context"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type DifyDocForm = "text_model" | "hierarchical_model" | "qa_model"

const difyDocFormCategories: Record<
  DifyDocForm,
  KnowledgeBaseCategoryId
> = {
  text_model: KnowledgeBaseCategory.KnowledgeBaseCategoryStandard,
  hierarchical_model: KnowledgeBaseCategory.KnowledgeBaseCategoryStandard,
  qa_model: KnowledgeBaseCategory.KnowledgeBaseCategoryQA,
}

/** 固定的 Dify 演示连接和知识库选项。 */
const demoDifyIntegrationConnectionId =
  "019c91a2-7b4e-7e52-a1c9-6f0d8b3a2e14"
const demoDifyKnowledgeBases = [
  {
    id: "019c91a2-8d63-7c21-b5e7-4a9f1d6c3b20",
    docForm: "text_model",
    label: "form.demoDifyDocumentKnowledgeBase",
  },
  {
    id: "019c91a2-9f74-77a3-86d2-71b5c4e8903f",
    docForm: "qa_model",
    label: "form.demoDifyQAKnowledgeBase",
  },
] as const
const defaultDemoDifyKnowledgeBase = demoDifyKnowledgeBases[0]

/** 创建或编辑知识库。 */
export function KnowledgeBaseFormPage({
  mode,
}: {
  mode: "create" | "edit"
}) {
  const { t } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const requestedExternal =
    mode === "create" && searchParams.get("source") === "dify"
  const requestedCategory = requestedExternal
    ? difyDocFormCategories[defaultDemoDifyKnowledgeBase.docForm]
    : searchParams.get("category") ===
        KnowledgeBaseCategory.KnowledgeBaseCategoryQA
      ? KnowledgeBaseCategory.KnowledgeBaseCategoryQA
      : KnowledgeBaseCategory.KnowledgeBaseCategoryStandard
  const { upsertKnowledgeBase } = useKnowledgeBaseContext()
  const { knowledgeBaseId = "" } = useParams()
  const [loading, setLoading] = useState(mode === "edit")
  const [loadError, setLoadError] = useState(false)
  const [external, setExternal] = useState(requestedExternal)
  const [category, setCategory] =
    useState<KnowledgeBaseCategoryId>(requestedCategory)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createKnowledgeBaseSchema(
        {
          nameRequired: t("validation.nameRequired"),
          nameTooLong: t("validation.nameTooLong"),
          descriptionTooLong: t("validation.descriptionTooLong"),
          integrationRequired: t("validation.integrationRequired"),
          externalResourceRequired: t("validation.externalResourceRequired"),
          externalResourceTooLong: t("validation.externalResourceTooLong"),
        },
        external,
      ),
    [external, t],
  )
  const form = useForm<KnowledgeBaseFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      name: "",
      description: "",
      integrationConnectionId: requestedExternal
        ? demoDifyIntegrationConnectionId
        : "",
      externalResourceId: requestedExternal
        ? defaultDemoDifyKnowledgeBase.id
        : "",
    },
  })
  useEffect(() => {
    if (mode !== "create") return
    setExternal(requestedExternal)
    setCategory(requestedCategory)
    form.reset({
      name: "",
      description: "",
      integrationConnectionId: requestedExternal
        ? demoDifyIntegrationConnectionId
        : "",
      externalResourceId: requestedExternal
        ? defaultDemoDifyKnowledgeBase.id
        : "",
    })
  }, [form, mode, requestedCategory, requestedExternal])

  /** 读取待编辑知识库。 */
  const load = useCallback(async () => {
    if (mode !== "edit") return
    const version = ++loadVersion.current
    setLoading(true)
    setLoadError(false)
    try {
      const knowledgeBase = await getKnowledgeBase(knowledgeBaseId)
      if (version !== loadVersion.current) return
      form.reset({
        name: knowledgeBase.name,
        description: knowledgeBase.description,
        integrationConnectionId: knowledgeBase.integrationConnectionId,
        externalResourceId: knowledgeBase.externalResourceId,
      })
      setCategory(knowledgeBase.category)
      setExternal(knowledgeBase.integrationConnectionId !== "")
    } catch (error) {
      if (version !== loadVersion.current) return
      if (recoverSession(error, navigate)) return
      console.warn("知识库详情加载失败", {
        knowledge_base_id: knowledgeBaseId,
        error,
      })
      setLoadError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [form, knowledgeBaseId, mode, navigate])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

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
              "integrationConnectionId",
              "externalResourceId",
            ])
          : t("form.saveError"),
      )
    }
  }

  const title =
    mode === "create" ? t("form.createTitle") : t("form.editTitle")

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title}>
        <PageBack to="/knowledge-bases" />
      </PageHeader>
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("loading")}
          </div>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("form.loadError")}
            </p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void load()}
            >
              {t("retry")}
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
              <Field>
                <FieldLabel>{t("form.source")}</FieldLabel>
                <p className="text-sm">
                  {external ? t("source.dify") : t("source.internal")}
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
              {external ? (
                <>
                  <Controller
                    name="integrationConnectionId"
                    control={form.control}
                    render={({ field, fieldState }) => (
                      <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor={field.name} required>
                          {t("form.integration")}
                        </FieldLabel>
                        <NativeSelect
                          {...field}
                          id={field.name}
                          required
                          aria-invalid={fieldState.invalid}
                        >
                          <option value={demoDifyIntegrationConnectionId}>
                            {t("form.demoDifyIntegration")}
                          </option>
                        </NativeSelect>
                      </Field>
                    )}
                  />
                  <Controller
                    name="externalResourceId"
                    control={form.control}
                    render={({ field, fieldState }) => (
                      <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor={field.name} required>
                          {t("form.difyKnowledgeBase")}
                        </FieldLabel>
                        <NativeSelect
                          {...field}
                          id={field.name}
                          required
                          aria-invalid={fieldState.invalid}
                          onChange={(event) => {
                            const selectedKnowledgeBase =
                              demoDifyKnowledgeBases[
                                event.currentTarget.selectedIndex
                              ]
                            field.onChange(event)
                            setCategory(
                              difyDocFormCategories[
                                selectedKnowledgeBase.docForm
                              ],
                            )
                          }}
                        >
                          {demoDifyKnowledgeBases.map((knowledgeBase) => (
                            <option
                              key={knowledgeBase.id}
                              value={knowledgeBase.id}
                            >
                              {t(knowledgeBase.label)}
                            </option>
                          ))}
                        </NativeSelect>
                      </Field>
                    )}
                  />
                </>
              ) : null}
            </FieldGroup>
            <div className="flex items-center gap-3">
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting
                  ? t("form.saving")
                  : mode === "create"
                    ? t("form.create")
                    : t("form.save")}
              </Button>
              <Button type="button" variant="outline" asChild>
                <Link to="/knowledge-bases">{t("form.cancel")}</Link>
              </Button>
            </div>
          </form>
        )}
      </PageContent>
    </div>
  )
}
