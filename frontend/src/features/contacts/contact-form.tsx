/** 新建联系人表单。 */
import { useMemo } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ContactMethodType,
  ContactStage,
  createContact,
  isApiError,
  recoverSession,
  type ChannelSummary,
  type ContactDetail,
  type ContactInput,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { PhoneInput } from "@/components/ui/phone-input"
import { Textarea } from "@/components/ui/textarea"
import { channelTypeLabel } from "@/features/contacts/contact-labels"
import {
  createContactSchema,
  type ContactFormValues,
} from "@/features/contacts/contact-schema"
import { apiErrorMessage } from "@/lib/form-errors"

/** 创建联系人。 */
export function ContactForm({
  channels,
  onSaved,
  onCancel,
}: {
  channels: ChannelSummary[]
  onSaved: (detail: ContactDetail) => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      createContactSchema({
        identityRequired: t("validation.identityRequired"),
        channelRequired: t("validation.channelRequired"),
        nameTooLong: t("validation.nameTooLong"),
        emailInvalid: t("validation.emailInvalid"),
        phoneInvalid: t("validation.phoneInvalid"),
        notesTooLong: t("validation.notesTooLong"),
      }),
    [t],
  )
  const form = useForm<ContactFormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      displayName: "",
      channelId: "",
      stage: ContactStage.ContactStageVisitor,
      email: "",
      phone: "",
      notes: "",
    },
  })

  /** 提交新建联系人。 */
  async function submit(values: ContactFormValues) {
    const input: ContactInput = {
      displayName: values.displayName,
      channelId: values.channelId,
      stage: values.stage,
      notes: values.notes,
      methods: [
        ...(values.email
          ? [{ type: ContactMethodType.ContactMethodTypeEmail, value: values.email, label: "", isPrimary: true }]
          : []),
        ...(values.phone
          ? [{ type: ContactMethodType.ContactMethodTypePhone, value: values.phone, label: "", isPrimary: true }]
          : []),
      ],
    }
    try {
      const saved = await createContact(input)
      console.info("联系人已创建", { contact_id: saved.contact.id })
      toast.success(t("form.created"))
      onSaved(saved)
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      if (isApiError(error)) {
        console.warn("创建联系人失败", error)
        toast.error(apiErrorMessage(error, ["displayName", "channelId", "stage", "methods", "notes"]))
        return
      }
      console.warn("创建联系人失败", error)
      toast.error(t("form.networkError"))
    }
  }

  return (
    <form
      onSubmit={form.handleSubmit(
        submit,
        () => toast.error(t("validation.checkFields")),
      )}
      noValidate
    >
      <FieldGroup className="gap-5">
        <FormInputField
          name="displayName"
          control={form.control}
          label={t("form.displayName")}
          required={false}
          autoFocus
        />

        <Controller
          name="channelId"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("form.channel")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                required
                aria-invalid={fieldState.invalid}
              >
                <option value="" disabled>
                  {t("form.channelPlaceholder")}
                </option>
                {channels.map((channel) => (
                  <option key={channel.id} value={channel.id}>
                    {channelTypeLabel(channel.type, t)} · {channel.name}
                  </option>
                ))}
              </NativeSelect>
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />

        <Controller
          name="stage"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name} required>
                {t("form.stage")}
              </FieldLabel>
              <NativeSelect
                {...field}
                id={field.name}
                required
                aria-invalid={fieldState.invalid}
              >
                <option value={ContactStage.ContactStageVisitor}>{t("stages.visitor")}</option>
                <option value={ContactStage.ContactStageLead}>{t("stages.lead")}</option>
                <option value={ContactStage.ContactStageCustomer}>{t("stages.customer")}</option>
              </NativeSelect>
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />

        <FormInputField
          name="email"
          control={form.control}
          label={t("form.email")}
          type="email"
          required={false}
        />

        <Controller
          name="phone"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field>
              <FieldLabel htmlFor={field.name}>{t("form.phone")}</FieldLabel>
              <PhoneInput
                ref={field.ref}
                id={field.name}
                name={field.name}
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                aria-invalid={fieldState.invalid}
                autoComplete="tel"
              />
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />

        <Controller
          name="notes"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={field.name}>{t("form.notes")}</FieldLabel>
              <Textarea
                {...field}
                id={field.name}
                aria-invalid={fieldState.invalid}
              />
              <FieldError errors={[fieldState.error]} />
            </Field>
          )}
        />

        <div className="flex items-center gap-2 pt-2">
          <Button type="submit" disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting ? t("form.saving") : t("form.save")}
          </Button>
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("form.cancel")}
          </Button>
        </div>
      </FieldGroup>
    </form>
  )
}
