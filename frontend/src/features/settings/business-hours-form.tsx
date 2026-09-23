/** 企业客服工作时间设置表单。 */
import { useEffect, useId, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { PlusIcon, XIcon } from "lucide-react"
import {
  Controller,
  useFieldArray,
  useForm,
  useFormState,
  useWatch,
  type Control,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  getBusinessHours,
  isApiError,
  updateBusinessHours,
  type BusinessHoursData,
} from "@/api"
import { ResourceContent } from "@/components/resource-content"
import { FormValidationMessage } from "@/components/form/form-validation-message"
import { SwitchCardField } from "@/components/form/switch-card-field"
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
  clockValue,
  createBusinessHoursSchema,
  periodEndInput,
  periodEndValue,
  type BusinessHoursFormValues,
} from "@/features/settings/business-hours-schema"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { supportedTimeZones } from "@/lib/time-zones"

/** 周一到周日的文案键。 */
const weekdays = [
  "monday",
  "tuesday",
  "wednesday",
  "thursday",
  "friday",
  "saturday",
  "sunday",
] as const

/** 开启某天上班时默认填入的时段。 */
const defaultPeriod = { start: "09:00", end: "18:00" }

/** 返回嵌套字段错误中的第一条文案。 */
function firstErrorMessage(error: unknown): string | undefined {
  if (!error || typeof error !== "object") return undefined
  if ("message" in error && typeof error.message === "string" && error.message) {
    return error.message
  }
  for (const [key, value] of Object.entries(error)) {
    // ref 指向输入元素，不参与查找。
    if (key === "ref") continue
    const message = firstErrorMessage(value)
    if (message) return message
  }
  return undefined
}

/** 按服务端取值换算一组时段的表单值或提交值。 */
function mapPeriods(
  periods: { start: string; end: string }[],
  end: (value: string) => string,
) {
  return periods.map((period) => ({ start: clockValue(period.start), end: end(period.end) }))
}

/** 读取客服工作时间并显示设置表单。 */
export function BusinessHoursSettings() {
  const { t } = useTranslation("settings")
  const hours = useResource(resourceKeys.businessHours(), () => getBusinessHours())
  return (
    <ResourceContent resources={hours} errorMessage={t("customerService.loadError")}>
      {hours.data ? <BusinessHoursForm hours={hours.data} /> : null}
    </ResourceContent>
  )
}

/** 维护启用开关、时区、每周时段和特殊日期，修改后自动保存。 */
function BusinessHoursForm({ hours }: { hours: BusinessHoursData }) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createBusinessHoursSchema({
        timeRequired: t("customerService.validation.timeRequired"),
        periodOrder: t("customerService.validation.periodOrder"),
        periodOverlap: t("customerService.validation.periodOverlap"),
        dateRequired: t("customerService.validation.dateRequired"),
        dateDuplicate: t("customerService.validation.dateDuplicate"),
      }),
    [t],
  )
  const form = useForm<BusinessHoursFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onChange",
    defaultValues: {
      enabled: hours.enabled,
      timeZone: hours.timeZone,
      weekly: hours.weekly.map((periods) => ({ periods: mapPeriods(periods, periodEndInput) })),
      overrides: hours.overrides.map((override) => ({ date: override.date, periods: mapPeriods(override.periods, periodEndInput) })),
    },
  })
  const timeZones = useMemo(
    () => supportedTimeZones(hours.timeZone),
    [hours.timeZone],
  )
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const { markSaved, saveNow } = useAutoSave({ form, schema, save })
  // 时段与日期的校验跨字段关联，组内任一值或行数变化后重新校验整组。
  useEffect(() => {
    const subscription = form.watch((_, { name }) => {
      const group = name?.split(".")[0]
      if (group === "weekly" || group === "overrides") void form.trigger(group)
    })
    return () => subscription.unsubscribe()
  }, [form])

  /** 保存客服工作时间，保留表单中的行顺序。 */
  async function save(values: BusinessHoursFormValues) {
    try {
      await updateBusinessHours({
        enabled: values.enabled,
        timeZone: values.timeZone,
        weekly: values.weekly.map((day) => mapPeriods(day.periods, periodEndValue)),
        overrides: values.overrides.map((override) => ({ date: override.date, periods: mapPeriods(override.periods, periodEndValue) })),
      })
      void invalidate(resourceKeys.businessHours())
      if (!mounted.current) return true
      markSaved(values)
      return true
    } catch (error) {
      // 离开页面后提交的改动失败时同样提示。
      if (recoverSession(error, navigate)) return false
      console.warn("保存客服工作时间失败", error)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, ["timeZone", "weekly", "overrides"]))
        return false
      }
      toast.error(t("customerService.saveError"))
      return false
    }
  }

  return (
    <form
      className="w-full"
      aria-label={t("customerService.businessHours.formLabel")}
      onSubmit={form.handleSubmit(() => saveNow())}
      noValidate
    >
      <FieldGroup>
        <Controller
          name="enabled"
          control={form.control}
          render={({ field }) => (
            <SwitchCardField
              id={field.name}
              name={field.name}
              label={t("customerService.businessHours.enabled")}
              description={t("customerService.businessHours.enabledDescription")}
              checked={field.value}
              onBlur={field.onBlur}
              onCheckedChange={field.onChange}
              ref={field.ref}
            />
          )}
        />
        <Controller
          name="timeZone"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("customerService.businessHours.timeZone")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
                required
              >
                {timeZones.map((timeZone) => (
                  <option key={timeZone} value={timeZone}>
                    {timeZone}
                  </option>
                ))}
              </NativeSelect>
            </Field>
          )}
        />
        <WeeklyHours control={form.control} />
        <OverrideDates control={form.control} />
      </FieldGroup>
    </form>
  )
}

/** 按周一到周日逐行维护上班开关与时段。 */
function WeeklyHours({ control }: { control: Control<BusinessHoursFormValues> }) {
  const { t } = useTranslation("settings")
  const id = useId()
  const { errors } = useFormState({ control, name: "weekly" })
  return (
    <div className="space-y-3" role="group" aria-labelledby={`${id}-label`}>
      <div>
        <div id={`${id}-label`} className="text-sm font-medium">
          {t("customerService.businessHours.weekly")}
        </div>
        <FieldDescription className="mt-1">
          {t("customerService.businessHours.weeklyDescription")}
        </FieldDescription>
      </div>
      <div className="divide-y rounded-lg border">
        {weekdays.map((weekday, index) => (
          <div className="flex items-start gap-3 px-4 py-3" key={weekday}>
            <div className="w-16 shrink-0 pt-1.5 text-sm">
              {t(`customerService.businessHours.weekdays.${weekday}`)}
            </div>
            <DayPeriods
              control={control}
              name={`weekly.${index}.periods`}
              dayLabel={t(`customerService.businessHours.weekdays.${weekday}`)}
            />
          </div>
        ))}
      </div>
      <FormValidationMessage message={firstErrorMessage(errors.weekly)} />
    </div>
  )
}

/** 维护按日期覆盖的时段，用于节假日休息与调休上班。 */
function OverrideDates({ control }: { control: Control<BusinessHoursFormValues> }) {
  const { t } = useTranslation("settings")
  const id = useId()
  const { fields, append, remove } = useFieldArray({
    control,
    name: "overrides",
    keyName: "fieldKey",
  })
  const { errors } = useFormState({ control, name: "overrides" })
  return (
    <div className="space-y-3" role="group" aria-labelledby={`${id}-label`}>
      <div>
        <div id={`${id}-label`} className="text-sm font-medium">
          {t("customerService.businessHours.overrides")}
        </div>
        <FieldDescription className="mt-1">
          {t("customerService.businessHours.overridesDescription")}
        </FieldDescription>
      </div>
      {fields.length > 0 ? (
        <div className="divide-y rounded-lg border">
          {fields.map((item, index) => (
            <div className="flex items-start gap-3 px-4 py-3" key={item.fieldKey}>
              <Controller
                name={`overrides.${index}.date`}
                control={control}
                render={({ field, fieldState }) => (
                  <Input
                    {...field}
                    type="date"
                    className="w-40 shrink-0"
                    aria-label={t("customerService.businessHours.overrideDate", { number: index + 1 })}
                    aria-invalid={fieldState.invalid}
                    required
                  />
                )}
              />
              <DayPeriods
                control={control}
                name={`overrides.${index}.periods`}
                dayLabel={t("customerService.businessHours.overrideDate", { number: index + 1 })}
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={t("customerService.businessHours.removeOverride", { number: index + 1 })}
                title={t("customerService.businessHours.removeOverride", { number: index + 1 })}
                onClick={() => remove(index)}
              >
                <XIcon />
              </Button>
            </div>
          ))}
        </div>
      ) : null}
      <FormValidationMessage message={firstErrorMessage(errors.overrides)} />
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => append({ date: "", periods: [] })}
      >
        {t("customerService.businessHours.addOverride")}
      </Button>
    </div>
  )
}

/** 一天的上班开关与时段列表；关闭上班时清空时段，开启时填入默认时段。 */
function DayPeriods({
  control,
  name,
  dayLabel,
}: {
  control: Control<BusinessHoursFormValues>
  name: `weekly.${number}.periods` | `overrides.${number}.periods`
  dayLabel: string
}) {
  const { t } = useTranslation("settings")
  const { fields, append, remove, replace } = useFieldArray({
    control,
    name,
    keyName: "fieldKey",
  })
  const periods = useWatch({ control, name })
  const working = fields.length > 0
  // 新时段从上一段结束时开始、到午夜结束；上一段已到午夜时留空由用户填写。
  const lastEnd = periods?.[periods.length - 1]?.end ?? ""
  const nextPeriod = lastEnd && lastEnd !== "00:00" ? { start: lastEnd, end: "00:00" } : { start: "", end: "" }
  return (
    <div className="flex min-w-0 flex-1 items-start gap-3">
      <div className="flex h-8 shrink-0 items-center gap-2">
        <Switch
          checked={working}
          aria-label={t("customerService.businessHours.working", { day: dayLabel })}
          onCheckedChange={(checked) => replace(checked ? [defaultPeriod] : [])}
        />
        <span className="w-8 text-sm text-muted-foreground">
          {working
            ? t("customerService.businessHours.workingShort")
            : t("customerService.businessHours.rest")}
        </span>
      </div>
      {working ? (
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          {fields.map((item, index) => (
            <div className="flex items-center gap-2" key={item.fieldKey}>
              <Controller
                name={`${name}.${index}.start`}
                control={control}
                render={({ field, fieldState }) => (
                  <Input
                    {...field}
                    type="time"
                    step={60}
                    className="w-28"
                    aria-label={t("customerService.businessHours.periodStart", { day: dayLabel, number: index + 1 })}
                    aria-invalid={fieldState.invalid}
                    required
                  />
                )}
              />
              <span className="text-muted-foreground">–</span>
              <Controller
                name={`${name}.${index}.end`}
                control={control}
                render={({ field, fieldState }) => (
                  <Input
                    {...field}
                    type="time"
                    step={60}
                    className="w-28"
                    aria-label={t("customerService.businessHours.periodEnd", { day: dayLabel, number: index + 1 })}
                    aria-invalid={fieldState.invalid}
                    required
                  />
                )}
              />
              {index === 0 ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={t("customerService.businessHours.addPeriod", { day: dayLabel })}
                  title={t("customerService.businessHours.addPeriod", { day: dayLabel })}
                  onClick={() => append(nextPeriod)}
                >
                  <PlusIcon />
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  aria-label={t("customerService.businessHours.removePeriod", { day: dayLabel, number: index + 1 })}
                  title={t("customerService.businessHours.removePeriod", { day: dayLabel, number: index + 1 })}
                  onClick={() => remove(index)}
                >
                  <XIcon />
                </Button>
              )}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}
