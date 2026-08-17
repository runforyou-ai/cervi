import {
  type MouseEvent,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  getS3Setting,
  saveS3Setting,
  type StorageProviderId,
  testS3Setting,
} from "@/api/settings"
import { ApiError } from "@/api/client"
import { FormInputField } from "@/components/form/form-input-field"
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
  FieldError,
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
  provider: "generic",
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

function isConfigured(setting: StorageSettingsFormValues) {
  return Boolean(
    setting.endpoint.trim() &&
      setting.region.trim() &&
      setting.bucket.trim() &&
      setting.accessKeyId.trim() &&
      setting.secretAccessKey.trim()
  )
}

function maskAccessKey(value: string) {
  if (value.length <= 8) {
    return "••••••••"
  }
  return `${value.slice(0, 4)}••••${value.slice(-4)}`
}

export function StorageSettingsForm() {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState("")
  const [savedSetting, setSavedSetting] =
    useState<StorageSettingsFormValues>(emptySetting)
  const [editing, setEditing] = useState(false)
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
  const displayedEnabled = configured ? savedSetting.enabled : editing

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

  const loadSetting = useCallback(async () => {
    setLoading(true)
    setLoadError("")
    try {
      const setting = await getS3Setting()
      setSavedSetting(setting)
      setEditing(false)
      form.reset(setting)
    } catch (error) {
      if (error instanceof ApiError && error.code === "AUTH_REQUIRED") {
        navigate("/login", { replace: true })
        return
      }
      setLoadError(t("storage.loadError"))
    } finally {
      setLoading(false)
    }
  }, [form, navigate, t])

  useEffect(() => {
    void loadSetting()
  }, [loadSetting])

  function handleRequestError(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.code === "AUTH_REQUIRED") {
        navigate("/login", { replace: true })
        return
      }
      toast.error(apiErrorMessage(error, settingFieldNames))
      return
    }
    toast.error(fallback)
  }

  async function save(values: StorageSettingsFormValues) {
    setPendingAction("save")
    try {
      const saved = await saveS3Setting(values)
      setSavedSetting(saved)
      setEditing(false)
      form.reset(saved)
      toast.success(t("storage.saveSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.saveError"))
    } finally {
      setPendingAction(null)
    }
  }

  async function test(values: StorageSettingsFormValues) {
    setPendingAction("test")
    try {
      await testS3Setting(values)
      toast.success(t("storage.testSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.testError"))
    } finally {
      setPendingAction(null)
    }
  }

  function beginEditing() {
    form.reset(savedSetting)
    setEditing(true)
  }

  function cancelEditing() {
    form.reset(savedSetting)
    setEditing(false)
  }

  function changeEnabled(checked: boolean) {
    if (!configured) {
      if (checked) {
        form.reset({ ...savedSetting, enabled: true })
        setEditing(true)
      } else {
        cancelEditing()
      }
      return
    }

    if (checked) {
      void enableSavedSetting()
      return
    }
    setDisableDialogOpen(true)
  }

  async function enableSavedSetting() {
    const nextSetting = { ...savedSetting, enabled: true }
    setPendingAction("enable")
    try {
      await testS3Setting(nextSetting)
      const enabledSetting = await saveS3Setting(nextSetting)
      setSavedSetting(enabledSetting)
      form.reset(enabledSetting)
      toast.success(t("storage.enableSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.enableError"))
    } finally {
      setPendingAction(null)
    }
  }

  async function disableSavedSetting() {
    const nextSetting = { ...savedSetting, enabled: false }
    setDisableDialogOpen(false)
    setPendingAction("disable")
    try {
      const disabledSetting = await saveS3Setting(nextSetting)
      setSavedSetting(disabledSetting)
      setEditing(false)
      form.reset(disabledSetting)
      toast.success(t("storage.disableSuccess"))
    } catch (error) {
      handleRequestError(error, t("storage.disableError"))
    } finally {
      setPendingAction(null)
    }
  }

  async function openProviderHelp(event: MouseEvent<HTMLAnchorElement>) {
    event.preventDefault()
    try {
      await openExternalURL(activeProvider.helpUrl)
    } catch {
      toast.error(t("storage.openDocumentationError"))
    }
  }

  if (loading) {
    return (
      <div className="mt-8 flex items-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("storage.loading")}
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="mt-8">
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
    <div className="mt-6 w-full">
      <Field className="gap-2">
        <FieldLabel htmlFor="storage-enabled">
          {t("storage.form.enabled")}
        </FieldLabel>
        <div className="flex items-center gap-3">
          <Switch
            id="storage-enabled"
            checked={displayedEnabled}
            disabled={submitting || (editing && configured)}
            onCheckedChange={changeEnabled}
          />
          <FieldDescription>
            {!configured
              ? editing
                ? t("storage.state.configuringDescription")
                : t("storage.state.unconfiguredDescription")
              : savedSetting.enabled
                ? t("storage.state.enabledDescription")
                : t("storage.state.disabledDescription")}
          </FieldDescription>
        </div>
      </Field>

      {editing ? (
        <form
          className="mt-8 w-full"
          onSubmit={form.handleSubmit(save)}
          noValidate
        >
          <h3 className="mb-6 text-base font-medium">
            {configured
              ? t("storage.form.editTitle")
              : t("storage.form.configureTitle")}
          </h3>
          <FieldGroup className="gap-6">
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
                  <FieldError errors={[fieldState.error]} />
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
                    <FieldError errors={[fieldState.error]} />
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
                  <FieldDescription>
                    {t("storage.form.endpointDescription")}
                  </FieldDescription>
                  <FieldError errors={[fieldState.error]} />
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

            <div className="flex flex-wrap items-center gap-4">
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
                variant="ghost"
                disabled={submitting}
                onClick={cancelEditing}
              >
                {t("storage.form.cancel")}
              </Button>
            </div>
          </FieldGroup>
        </form>
      ) : configured ? (
        <section className="mt-8" aria-labelledby="storage-detail-title">
          <h3 id="storage-detail-title" className="text-base font-medium">
            {t("storage.detail.title")}
          </h3>
          <dl className="mt-4 border-y text-sm">
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.detail.status")}
              </dt>
              <dd>
                {savedSetting.enabled
                  ? t("storage.detail.enabled")
                  : t("storage.detail.disabled")}
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
              <dd>{maskAccessKey(savedSetting.accessKeyId)}</dd>
            </div>
            <div className="grid gap-1 border-b py-3 sm:grid-cols-[12rem_minmax(0,1fr)]">
              <dt className="text-muted-foreground">
                {t("storage.form.secretAccessKey")}
              </dt>
              <dd>{t("storage.detail.credentialConfigured")}</dd>
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

          <div className="mt-6 flex flex-wrap items-center gap-4">
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
      ) : null}

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
