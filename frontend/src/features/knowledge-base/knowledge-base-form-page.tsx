/** 企业知识库新建和编辑页。 */
import { useEffect, useMemo, useRef, useState } from "react"
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
  IntegrationConnectionType,
  KnowledgeBaseCategory,
  type KnowledgeBaseCategoryId,
  type KnowledgeBaseData,
  createKnowledgeBase,
  getKnowledgeBase,
  isApiError,
  listExternalKnowledgeBaseOptions,
  listIntegrationConnections,
  updateKnowledgeBase,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
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
  const { t } = useTranslation("knowledgeBase")
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const requestedExternal =
    mode === "create" && searchParams.get("source") === "dify"
  const requestedCategory =
    searchParams.get("category") ===
    KnowledgeBaseCategory.KnowledgeBaseCategoryQA
      ? KnowledgeBaseCategory.KnowledgeBaseCategoryQA
      : KnowledgeBaseCategory.KnowledgeBaseCategoryStandard
  const { upsertKnowledgeBase } = useKnowledgeBaseContext()
  const { knowledgeBaseId = "" } = useParams()
  const invalidateResource = useResourceInvalidator()
  const [external, setExternal] = useState(requestedExternal)
  const [category, setCategory] =
    useState<KnowledgeBaseCategoryId>(requestedCategory)
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
      integrationConnectionId: "",
      externalResourceId: "",
    },
  })
  const selectedConnectionId = form.watch("integrationConnectionId")
  const selectedExternalResourceId = form.watch("externalResourceId")
  useEffect(() => {
    if (mode !== "create") return
    setExternal(requestedExternal)
    setCategory(requestedCategory)
    form.reset({
      name: "",
      description: "",
      integrationConnectionId: "",
      externalResourceId: "",
    })
  }, [form, mode, requestedCategory, requestedExternal])

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
  const {
    data: connectionList,
    loading: connectionLoading,
    refreshing: connectionRefreshing,
    error: connectionError,
    refresh: refreshConnections,
  } = useResource(
    resourceKeys.connectors(),
    () => listIntegrationConnections(),
    { enabled: external, staleTime: 0 },
  )
  const difyConnections = (connectionList?.connections ?? []).filter(
    (connection) =>
      connection.type ===
      IntegrationConnectionType.IntegrationConnectionTypeDify,
  )
  const showConnectionLoading =
    external &&
    (connectionLoading || (Boolean(connectionError) && connectionRefreshing))
  const {
    data: externalOptionList,
    loading: externalOptionLoading,
    refreshing: externalOptionRefreshing,
    error: externalOptionError,
    refresh: refreshExternalOptions,
  } = useResource(
    resourceKeys.externalKnowledgeBaseOptions(selectedConnectionId),
    () => listExternalKnowledgeBaseOptions(selectedConnectionId),
    { enabled: external && selectedConnectionId !== "", staleTime: 0 },
  )
  const externalOptions = useMemo(
    () => externalOptionList?.knowledgeBases ?? [],
    [externalOptionList],
  )

  /** 远端列表刷新后，把失效选择同步到当前首个可用知识库。 */
  useEffect(() => {
    if (!externalOptionList || selectedConnectionId === "") return
    if (
      externalOptions.some(
        (knowledgeBase) => knowledgeBase.id === selectedExternalResourceId,
      )
    ) {
      return
    }
    const nextExternalResourceId = externalOptions[0]?.id ?? ""
    if (nextExternalResourceId === selectedExternalResourceId) return
    form.setValue("externalResourceId", nextExternalResourceId, {
      shouldDirty: selectedExternalResourceId !== "",
    })
  }, [
    externalOptionList,
    externalOptions,
    form,
    selectedConnectionId,
    selectedExternalResourceId,
  ])

  const selectedExternalOption = externalOptions.find(
    (knowledgeBase) => knowledgeBase.id === selectedExternalResourceId,
  )
  const effectiveCategory = selectedExternalOption?.category ?? category
  const isQACategory =
    effectiveCategory === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  const showExternalOptionLoading =
    external &&
    selectedConnectionId !== "" &&
    (externalOptionLoading ||
      (Boolean(externalOptionError) && externalOptionRefreshing))
  const externalConfigurationReady =
    !external ||
    (!showConnectionLoading &&
      !connectionError &&
      !showExternalOptionLoading &&
      !externalOptionError &&
      Boolean(selectedExternalOption))

  /** 详情就绪后回填知识库表单和派生状态。 */
  useEffect(() => {
    if (!loadedKnowledgeBase) return
    form.reset({
      name: loadedKnowledgeBase.name,
      description: loadedKnowledgeBase.description,
      integrationConnectionId: loadedKnowledgeBase.integrationConnectionId,
      externalResourceId: loadedKnowledgeBase.externalResourceId,
    })
    setCategory(loadedKnowledgeBase.category)
    setExternal(loadedKnowledgeBase.integrationConnectionId !== "")
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
      const input = { ...values, category: effectiveCategory }
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
      <PageHeader title={title} />
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
              onClick={() => void refreshKnowledgeBase()}
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
                  {isQACategory ? t("category.qa") : t("category.standard")}
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
                        {showConnectionLoading ? (
                          <NativeSelect id={field.name} disabled>
                            <option>{t("form.connectionsLoading")}</option>
                          </NativeSelect>
                        ) : connectionError ? (
                          <div className="flex items-center gap-3">
                            <p className="text-sm text-muted-foreground">
                              {t("form.connectionsLoadError")}
                            </p>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={() => void refreshConnections()}
                            >
                              {t("retry")}
                            </Button>
                          </div>
                        ) : difyConnections.length === 0 ? (
                          <div className="flex items-center gap-3">
                            <p className="text-sm text-muted-foreground">
                              {t("form.noDifyConnections")}
                            </p>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              asChild
                            >
                              <Link to="/integrations/connectors/new">
                                {t("form.addDifyConnection")}
                              </Link>
                            </Button>
                          </div>
                        ) : (
                          <NativeSelect
                            {...field}
                            id={field.name}
                            required
                            aria-invalid={fieldState.invalid}
                            onChange={(event) => {
                              field.onChange(event)
                              form.setValue("externalResourceId", "", {
                                shouldDirty: true,
                              })
                              setCategory(
                                KnowledgeBaseCategory.KnowledgeBaseCategoryStandard,
                              )
                            }}
                          >
                            <option value="" disabled>
                              {t("form.selectIntegration")}
                            </option>
                            {difyConnections.map((connection) => (
                              <option key={connection.id} value={connection.id}>
                                {connection.name}
                              </option>
                            ))}
                          </NativeSelect>
                        )}
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
                        {selectedConnectionId === "" ? (
                          <NativeSelect id={field.name} disabled>
                            <option>{t("form.selectIntegrationFirst")}</option>
                          </NativeSelect>
                        ) : showExternalOptionLoading ? (
                          <NativeSelect id={field.name} disabled>
                            <option>{t("form.knowledgeBasesLoading")}</option>
                          </NativeSelect>
                        ) : externalOptionError ? (
                          <div className="flex items-center gap-3">
                            <p className="text-sm text-muted-foreground">
                              {t("form.knowledgeBasesLoadError")}
                            </p>
                            <Button
                              type="button"
                              variant="outline"
                              size="sm"
                              onClick={() => void refreshExternalOptions()}
                            >
                              {t("retry")}
                            </Button>
                          </div>
                        ) : externalOptions.length === 0 ? (
                          <p className="text-sm text-muted-foreground">
                            {t("form.noDifyKnowledgeBases")}
                          </p>
                        ) : (
                          <NativeSelect
                            {...field}
                            id={field.name}
                            required
                            aria-invalid={fieldState.invalid}
                          >
                            <option value="" disabled>
                              {t("form.selectDifyKnowledgeBase")}
                            </option>
                            {externalOptions.map((knowledgeBase) => (
                              <option
                                key={knowledgeBase.id}
                                value={knowledgeBase.id}
                              >
                                {knowledgeBase.name}
                              </option>
                            ))}
                          </NativeSelect>
                        )}
                      </Field>
                    )}
                  />
                </>
              ) : null}
            </FieldGroup>
            <div className="flex items-center gap-3">
              <Button
                type="submit"
                disabled={
                  form.formState.isSubmitting || !externalConfigurationReady
                }
              >
                {form.formState.isSubmitting
                  ? t("form.saving")
                  : mode === "create"
                    ? t("form.create")
                    : t("form.save")}
              </Button>
              {mode === "create" ? (
                <Button type="button" variant="outline" asChild>
                  <Link to="/knowledge-bases">{t("form.cancel")}</Link>
                </Button>
              ) : null}
            </div>
          </form>
        )}
      </PageContent>
    </div>
  )
}
