/** 企业客服分配与提醒设置表单。 */
import { useEffect, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import { getServiceTimeouts, isApiError, updateServiceTimeouts } from "@/api"
import { ResourceContent } from "@/components/resource-content"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { resourceKeys } from "@/hooks/resource-keys"
import { useAutoSave } from "@/hooks/use-auto-save"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 表单中的三个时长字段，按页面顺序排列。 */
const timeoutFields = [
  "responseReminderMinutes",
  "responseReclaimMinutes",
  "queueReminderMinutes",
] as const

type ServiceTimeoutsFormValues = Record<(typeof timeoutFields)[number], string>

/** 创建超时时长校验：各时长为正整数分钟，回收时长大于提醒时长。 */
function createServiceTimeoutsSchema(messages: {
  minutesInvalid: string
  reclaimAfterReminder: string
}) {
  const minutes = z.string().trim().regex(/^[1-9]\d*$/, messages.minutesInvalid)
  return z
    .object({
      responseReminderMinutes: minutes,
      responseReclaimMinutes: minutes,
      queueReminderMinutes: minutes,
    })
    .superRefine((values, context) => {
      if (Number(values.responseReclaimMinutes) <= Number(values.responseReminderMinutes)) {
        context.addIssue({
          code: "custom",
          path: ["responseReclaimMinutes"],
          message: messages.reclaimAfterReminder,
        })
      }
    })
}

/** 读取客服超时时长并显示设置表单。 */
export function ServiceTimeoutsSettings() {
  const { t } = useTranslation("settings")
  const timeouts = useResource(resourceKeys.serviceTimeouts(), () => getServiceTimeouts())
  return (
    <ResourceContent resources={timeouts} errorMessage={t("customerService.timeouts.loadError")}>
      {timeouts.data ? (
        <ServiceTimeoutsForm
          values={{
            responseReminderMinutes: String(timeouts.data.responseReminderMinutes),
            responseReclaimMinutes: String(timeouts.data.responseReclaimMinutes),
            queueReminderMinutes: String(timeouts.data.queueReminderMinutes),
          }}
        />
      ) : null}
    </ResourceContent>
  )
}

/** 维护未回复提醒、未回复回收与队列等待提醒时长，修改后自动保存。 */
function ServiceTimeoutsForm({ values }: { values: ServiceTimeoutsFormValues }) {
  const { t } = useTranslation("settings")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createServiceTimeoutsSchema({
        minutesInvalid: t("customerService.timeouts.validation.minutesInvalid"),
        reclaimAfterReminder: t("customerService.timeouts.validation.reclaimAfterReminder"),
      }),
    [t],
  )
  const form = useForm<ServiceTimeoutsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    mode: "onChange",
    defaultValues: values,
  })
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const { markSaved } = useAutoSave({ form, schema, save })
  // 提醒时长变化后重新校验回收时长。
  useEffect(() => {
    const subscription = form.watch((_, { name }) => {
      if (name === "responseReminderMinutes") void form.trigger("responseReclaimMinutes")
    })
    return () => subscription.unsubscribe()
  }, [form])

  /** 保存客服超时时长。 */
  async function save(submitted: ServiceTimeoutsFormValues) {
    try {
      await updateServiceTimeouts({
        responseReminderMinutes: Number(submitted.responseReminderMinutes),
        responseReclaimMinutes: Number(submitted.responseReclaimMinutes),
        queueReminderMinutes: Number(submitted.queueReminderMinutes),
      })
      void invalidate(resourceKeys.serviceTimeouts())
      if (!mounted.current) return true
      markSaved(submitted)
      return true
    } catch (error) {
      // 离开页面后提交的改动失败时同样提示。
      if (recoverSession(error, navigate)) return false
      console.warn("保存客服超时时长失败", error)
      if (isApiError(error)) {
        toast.error(apiErrorMessage(error, [...timeoutFields]))
        return false
      }
      toast.error(t("customerService.timeouts.saveError"))
      return false
    }
  }

  return (
    <form
      className="w-full"
      aria-label={t("customerService.timeouts.formLabel")}
      onSubmit={form.handleSubmit(save)}
      noValidate
    >
      <FieldGroup>
        {timeoutFields.map((name) => (
          <Controller
            key={name}
            name={name}
            control={form.control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor={field.name} required>
                  {t(`customerService.timeouts.${name}`)}
                </FieldLabel>
                <div className="flex items-center gap-2">
                  <Input
                    {...field}
                    id={field.name}
                    type="number"
                    inputMode="numeric"
                    min={1}
                    step={1}
                    className="w-28"
                    aria-invalid={fieldState.invalid}
                    aria-describedby={`${field.name}-description`}
                    required
                  />
                  <span className="text-sm text-muted-foreground">
                    {t("customerService.timeouts.minutes")}
                  </span>
                </div>
                <FieldDescription id={`${field.name}-description`}>
                  {t(`customerService.timeouts.${name}Description`)}
                </FieldDescription>
              </Field>
            )}
          />
        ))}
      </FieldGroup>
    </form>
  )
}
