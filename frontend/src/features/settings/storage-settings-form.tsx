/** 对象存储设置表单。 */
import {
  type MouseEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { EyeIcon, EyeOffIcon, LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  StorageProvider,
  getS3Setting,
  isApiError,
  saveS3Setting,
  testS3Setting,
  type StorageProviderId,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { FormInputField } from "@/components/form/form-input-field"
import { StatusBadge } from "@/components/status-badge"
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
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { Switch } from "@/components/ui/switch"
import {
  getStorageProvider,
  getStorageRegion,
  storageProviders,
} from "@/features/settings/storage-providers"
import {
  createStorageSettingsSchema,
  type StorageSettingsFormValues,
} from "@/features/settings/storage-settings-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { openExternalURL } from "@/platform/open-external-url"

const customRegionOption = "__custom__"

const emptySetting: StorageSettingsFormValues = {
  enabled: false,
  provider: StorageProvider.StorageProviderGeneric,
  endpoint: "https://s3.us-east-1.amazonaws.com",
  region: "us-east-1",
  bucket: "",
  accessKeyId: "",
  secretAccessKey: "",
  forcePathStyle: false,
}

const settingFieldNames = [
  "provider",
  "endpoint",
  "region",
  "bucket",
  "accessKeyId",
  "secretAccessKey",
] as const

/** 判断对象存储设置是否已填写完整。 */
function isConfigured(setting: StorageSettingsFormValues) {
  return Boolean(
    setting.endpoint.trim() &&
      setting.region.trim() &&
      setting.bucket.trim() &&
      setting.accessKeyId.trim() &&
      setting.secretAccessKey.trim()
  )
}

/** 读取、保存和测试对象存储设置。 */
export function StorageSettingsForm() {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState("")
  const [savedSetting, setSavedSetting] =
    useState<StorageSettingsFormValues>(emptySetting)
  const [editing, setEditing] = useState(false)
  const [secretAccessKeyVisible, setSecretAccessKeyVisible] = useState(false)
  const [disableDialogOpen, setDisableDialogOpen] = useState(false)
  const [pendingAction, setPendingAction] = useState<
    "save" | "test" | "enable" | "disable" | null
  >(null)
  const schema = useMemo(
    () =>
      createStorageSettingsSchema({
        providerInvalid: t("storage.validation.providerInvalid"),
        endpointRequired: t("storage.validation.endpointRequired"),
        endpointInvalid: t("storage.validation.endpointInvalid"),
        regionRequired: t("storage.validation.regionRequired"),
        bucketRequired: t("storage.validation.bucketRequired"),
        accessKeyIdRequired: t("storage.validation.accessKeyIdRequired"),
        secretAccessKeyRequired: t(
          "storage.validation.secretAccessKeyRequired"
        ),
      }),
    [t]
  )
  const form = useForm<StorageSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: emptySetting,
  })
  const activeProvider = getStorageProvider(form.watch("provider"))
  const activeRegion = getStorageRegion(activeProvider, form.watch("region"))
  const configured = isConfigured(savedSetting)

  /** 切换对象存储提供商并填充默认区域。 */
  function selectProvider(providerId: StorageProviderId) {
    const provider = getStorageProvider(providerId)
    const region = provider.regions[0]

    form.setValue("provider", providerId, {
      shouldDirty: true,
      shouldValidate: true,
    })
    form.setValue("region", region.id, {
      shouldDirty: true,
      shouldValidate: true,
    })
    form.setValue("endpoint", region.endpoint, {
      shouldDirty: true,
      shouldValidate: true,
    })
    form.setValue("forcePathStyle", provider.forcePathStyle, {
      shouldDirty: true,
    })
  }

  /** 选择预设区域或改为自定义区域。 */
  function selectRegion(regionId: string) {
    if (regionId === customRegionOption) {
      form.setValue("region", "", {
        shouldDirty: true,
        shouldValidate: true,
      })
      return
    }

    const region = getStorageRegion(activeProvider, regionId)!

    form.setValue("region", regionId, {
      shouldDirty: true,
      shouldValidate: true,
    })
    form.setValue("endpoint", region.endpoint, {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  /** 读取已保存的对象存储设置。 */
  const loadSetting = useCallback(async () => {
    setLoading(true)
    setLoadError("")
    try {
      const setting = await getS3Setting()
      setSavedSetting(setting)
      setEditing(false)
      form.reset(setting)
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      console.warn("对象存储设置加载失败", error)
      setLoadError(t("storage.loadError"))
    } finally {
      setLoading(false)
    }
  }, [form, navigate, t])

  useEffect(() => {
    void loadSetting()
  }, [loadSetting])

  /** 处理对象存储请求错误。 */
  function handleRequestError(error: unknown, message: string) {
    if (recoverSession(error, navigate)) {
      return
    }
    if (isApiError(error)) {
      console.warn("对象存储请求失败", error)
      toast.error(apiErrorMessage(error, settingFieldNames))
      return
    }
    console.warn("对象存储请求失败", error)
    toast.error(message)
  }

  /** 保存对象存储设置。 */
  async function save(values: StorageSettingsFormValues) {
    setPendingAction("save")
    try {
      const saved = await saveS3Setting(values)
      setSavedSetting(saved)
      setEditing(false)
      form.reset(saved)
      console.info("对象存储设置已保存")
      toast.success(t("storage.saveSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.saveError"))
    } finally {
      setPendingAction(null)
    }
  }

  /** 测试对象存储连接。 */
  async function test(values: StorageSettingsFormValues) {
    setPendingAction("test")
    try {
      await testS3Setting(values)
      console.info("对象存储连接测试成功")
      toast.success(t("storage.testSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.testError"))
    } finally {
      setPendingAction(null)
    }
  }

  /** 进入编辑模式。 */
  function beginEditing() {
    form.reset(savedSetting)
    setSecretAccessKeyVisible(false)
    setEditing(true)
  }

  /** 取消编辑并恢复已保存的设置。 */
  function cancelEditing() {
    form.reset(savedSetting)
    setEditing(false)
  }

  /** 启用已保存的对象存储设置。 */
  async function enableSavedSetting() {
    const nextSetting = { ...savedSetting, enabled: true }
    setPendingAction("enable")
    try {
      await testS3Setting(nextSetting)
      const enabledSetting = await saveS3Setting(nextSetting)
      setSavedSetting(enabledSetting)
      form.reset(enabledSetting)
      console.info("对象存储已启用")
      toast.success(t("storage.enableSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.enableError"))
    } finally {
      setPendingAction(null)
    }
  }

  /** 关闭对象存储。 */
  async function disableSavedSetting() {
    const nextSetting = { ...savedSetting, enabled: false }
    setDisableDialogOpen(false)
    setPendingAction("disable")
    try {
      const disabledSetting = await saveS3Setting(nextSetting)
      setSavedSetting(disabledSetting)
      setEditing(false)
      form.reset(disabledSetting)
      console.info("对象存储已停用")
      toast.success(t("storage.disableSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.disableError"))
    } finally {
      setPendingAction(null)
    }
  }

  /** 打开当前提供商的帮助文档。 */
  async function openProviderHelp(event: MouseEvent<HTMLAnchorElement>) {
    event.preventDefault()
    try {
      await openExternalURL(activeProvider.helpUrl)
    } catch (error) {
      console.warn("打开对象存储帮助文档失败", error)
      toast.error(t("storage.openDocumentationError"))
    }
  }

  if (loading) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("storage.loading")}
      </div>
    )
  }

  if (loadError) {
    return (
      <div>
        <p className="text-sm text-muted-foreground">{loadError}</p>
        <Button
          className="mt-4"
          variant="outline"
          onClick={() => void loadSetting()}
        >
          {t("storage.retry")}
        </Button>
      </div>
    )
  }

  const submitting = pendingAction !== null
  const detailProvider = getStorageProvider(savedSetting.provider)
  const detailRegion = getStorageRegion(detailProvider, savedSetting.region)

  return (
    <div className="w-full max-w-3xl">
      {editing ? (
        <form
          className="w-full"
          onSubmit={form.handleSubmit(save)}
          noValidate
        >
          <h3 className="mb-4 text-base font-medium">
            {configured
              ? t("storage.form.editTitle")
              : t("storage.form.configureTitle")}
          </h3>
          <FieldGroup>
            <Controller
              name="provider"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor={field.name} required>
                    {t("storage.form.provider")}
                  </FieldLabel>
                  <NativeSelect
                    {...field}
                    id={field.name}
                    aria-invalid={fieldState.invalid}
                    autoFocus
                    required
                    onChange={(event) =>
                      selectProvider(event.target.value as StorageProviderId)
                    }
                  >
                    {storageProviders.map((provider) => (
                      <option key={provider.id} value={provider.id}>
                        {t(provider.nameKey)}
                      </option>
                    ))}
                  </NativeSelect>
                  <FieldDescription>
                    <a
                      href={activeProvider.helpUrl}
                      target="_blank"
                      rel="noreferrer"
                      onClick={openProviderHelp}
                    >
                      {t("storage.form.providerHelp", {
                        provider: t(activeProvider.nameKey),
                      })}
                    </a>
                  </FieldDescription>
                </Field>
              )}
            />

            <Field>
              <FieldLabel htmlFor="storage-region" required>
                {t("storage.form.region")}
              </FieldLabel>
              <NativeSelect
                id="storage-region"
                value={activeRegion?.id ?? customRegionOption}
                onChange={(event) => selectRegion(event.target.value)}
              >
                {activeProvider.regions.map((region) => (
                  <option key={region.id} value={region.id}>
                    {t(region.nameKey)} ({region.id})
                  </option>
                ))}
                <option value={customRegionOption}>
                  {t("storage.form.customRegionOption")}
                </option>
              </NativeSelect>
            </Field>

            {!activeRegion ? (
              <Controller
                name="region"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="storage-custom-region" required>
                      {t("storage.form.customRegion")}
                    </FieldLabel>
                    <Input
                      {...field}
                      id="storage-custom-region"
                      required
                      aria-invalid={fieldState.invalid}
                    />
                  </Field>
                )}
              />
            ) : null}

            <Controller
              name="endpoint"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor={field.name} required>
                    {t("storage.form.endpoint")}
                  </FieldLabel>
                  <Input
                    {...field}
                    id={field.name}
                    type="url"
                    required
                    aria-invalid={fieldState.invalid}
                  />
                </Field>
              )}
            />

            <FormInputField
              name="bucket"
              control={form.control}
              label={t("storage.form.bucket")}
            />

            <FormInputField
              name="accessKeyId"
              control={form.control}
              label={t("storage.form.accessKeyId")}
              autoComplete="off"
            />

            <FormInputField
              name="secretAccessKey"
              control={form.control}
              label={t("storage.form.secretAccessKey")}
              type="password"
              autoComplete="new-password"
              passwordVisibilityLabels={{
                show: t("storage.form.showSecretAccessKey"),
                hide: t("storage.form.hideSecretAccessKey"),
              }}
            />

            <Controller
              name="forcePathStyle"
              control={form.control}
              render={({ field }) => (
                <Field className="gap-2">
                  <FieldLabel htmlFor={field.name}>
                    {t("storage.form.forcePathStyle")}
                  </FieldLabel>
                  <div className="flex items-center gap-3">
                    <Switch
                      id={field.name}
                      name={field.name}
                      checked={field.value}
                      onBlur={field.onBlur}
                      onCheckedChange={field.onChange}
                      ref={field.ref}
                    />
                    <FieldDescription>
                      {t("storage.form.forcePathStyleDescription")}
                    </FieldDescription>
                  </div>
                </Field>
              )}
            />

            <div className="flex flex-wrap items-center gap-2">
              <Button type="submit" disabled={submitting}>
                {pendingAction === "save"
                  ? t("storage.form.saving")
                  : t("storage.form.save")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={submitting}
                onClick={form.handleSubmit(test)}
              >
                {pendingAction === "test"
                  ? t("storage.form.testing")
                  : t("storage.form.test")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={submitting}
                onClick={cancelEditing}
              >
                {t("storage.form.cancel")}
              </Button>
            </div>
          </FieldGroup>
        </form>
      ) : configured ? (
        <section aria-labelledby="storage-detail-title">
          <h3 id="storage-detail-title" className="text-base font-medium">
            {t("storage.detail.title")}
          </h3>
          <dl className="mt-4 border-y text-sm">
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.detail.status")}
              </dt>
              <dd>
                <StatusBadge
                  variant={savedSetting.enabled ? "success" : "muted"}
                >
                  {savedSetting.enabled
                    ? t("storage.detail.enabled")
                    : t("storage.detail.disabled")}
                </StatusBadge>
              </dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.provider")}
              </dt>
              <dd>{t(detailProvider.nameKey)}</dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.region")}
              </dt>
              <dd>
                {detailRegion
                  ? `${t(detailRegion.nameKey)} · ${savedSetting.region}`
                  : savedSetting.region}
              </dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.endpoint")}
              </dt>
              <dd className="break-all">{savedSetting.endpoint}</dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.bucket")}
              </dt>
              <dd>{savedSetting.bucket}</dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.accessKeyId")}
              </dt>
              <dd>{savedSetting.accessKeyId}</dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.secretAccessKey")}
              </dt>
              <dd className="flex items-center gap-1">
                <span>
                  {secretAccessKeyVisible
                    ? savedSetting.secretAccessKey
                    : "••••••••"}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={
                    secretAccessKeyVisible
                      ? t("storage.form.hideSecretAccessKey")
                      : t("storage.form.showSecretAccessKey")
                  }
                  aria-pressed={secretAccessKeyVisible}
                  onClick={() =>
                    setSecretAccessKeyVisible((visible) => !visible)
                  }
                >
                  {secretAccessKeyVisible ? <EyeOffIcon /> : <EyeIcon />}
                </Button>
              </dd>
            </div>
            <div className="grid gap-1 py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.forcePathStyle")}
              </dt>
              <dd>
                {savedSetting.forcePathStyle
                  ? t("storage.detail.yes")
                  : t("storage.detail.no")}
              </dd>
            </div>
          </dl>

          <div className="mt-6 flex flex-wrap items-center gap-2">
            {!savedSetting.enabled ? (
              <Button
                type="button"
                disabled={submitting}
                onClick={() => void enableSavedSetting()}
              >
                {pendingAction === "enable"
                  ? t("storage.actions.enabling")
                  : t("storage.actions.enable")}
              </Button>
            ) : null}
            <Button
              type="button"
              variant={savedSetting.enabled ? "default" : "outline"}
              disabled={submitting}
              onClick={beginEditing}
            >
              {t("storage.actions.edit")}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={submitting}
              onClick={() => void test(savedSetting)}
            >
              {pendingAction === "test"
                ? t("storage.form.testing")
                : t("storage.form.test")}
            </Button>
            {savedSetting.enabled ? (
              <Button
                type="button"
                variant="destructive"
                disabled={submitting}
                onClick={() => setDisableDialogOpen(true)}
              >
                {pendingAction === "disable"
                  ? t("storage.actions.disabling")
                  : t("storage.actions.disable")}
              </Button>
            ) : null}
          </div>
        </section>
      ) : (
        <div className="grid gap-4">
          <p className="text-sm text-muted-foreground">
            {t("storage.state.unconfiguredDescription")}
          </p>
          <div>
            <Button type="button" onClick={beginEditing}>
              {t("storage.actions.configure")}
            </Button>
          </div>
        </div>
      )}

      <AlertDialog
        open={disableDialogOpen}
        onOpenChange={setDisableDialogOpen}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("storage.disableDialog.title")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("storage.disableDialog.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("storage.disableDialog.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction onClick={() => void disableSavedSetting()}>
              {t("storage.disableDialog.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
