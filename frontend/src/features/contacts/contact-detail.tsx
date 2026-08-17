import { useEffect, useMemo, useState, type ReactNode } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, type FieldErrors } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { ApiError } from "@/api/client"
import {
  updateContact,
  type ContactDetail,
  type ContactInput,
  type ContactMethodInput,
  type ContactStage,
} from "@/api/contacts"
import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { PhoneInput } from "@/components/ui/phone-input"
import { Textarea } from "@/components/ui/textarea"
import {
  createContactSchema,
  type ContactFormValues,
} from "@/features/contacts/contact-schema"
import { useDateTime } from "@/hooks/use-date-time"
import { apiErrorMessage } from "@/lib/form-errors"

type EditingSection = "name" | "stage" | "methods" | "notes" | null

function valuesFromDetail(detail: ContactDetail): ContactFormValues {
  return {
    displayName: detail.contact.displayName ?? "",
    channelId: detail.contact.sourceChannelId,
    stage: detail.contact.stage,
    email: detail.methods.find((method) => method.type === "email")?.value ?? "",
    phone: detail.methods.find((method) => method.type === "phone")?.value ?? "",
    notes: detail.contact.notes ?? "",
  }
}

function methodsFromDetail(
  detail: ContactDetail,
  values: ContactFormValues,
): ContactMethodInput[] {
  // 详情表单只编辑每种类型的首项，其余联系方式保持不变。
  const editedValues = {
    email: values.email,
    phone: values.phone,
  }
  const handled = {
    email: false,
    phone: false,
  }
  const methods: ContactMethodInput[] = []

  for (const method of detail.methods) {
    if (!handled[method.type]) {
      handled[method.type] = true
      const value = editedValues[method.type]
      if (!value) {
        continue
      }
      methods.push({
        type: method.type,
        value,
        label: method.label ?? undefined,
        isPrimary: method.isPrimary,
      })
      continue
    }
    methods.push({
      type: method.type,
      value: method.value,
      label: method.label ?? undefined,
      isPrimary: method.isPrimary,
    })
  }

  for (const type of ["email", "phone"] as const) {
    if (!handled[type] && editedValues[type]) {
      methods.push({ type, value: editedValues[type], isPrimary: true })
    }
  }
  return methods
}

function DetailRow({
  label,
  value,
  editing,
  editEnabled,
  onEdit,
  children,
}: {
  label: string
  value: ReactNode
  editing: boolean
  editEnabled: boolean
  onEdit: () => void
  children: ReactNode
}) {
  const { t } = useTranslation("contacts")

  return (
    <div className="group rounded-md px-2 py-2.5 transition-colors hover:bg-muted/50 focus-within:bg-muted/50">
      <div className="flex items-start gap-3">
        <div className="w-28 shrink-0 pt-1 text-sm text-muted-foreground">{label}</div>
        <div className="min-w-0 flex-1">
          {editing ? children : <div className="pt-1 text-sm break-words">{value}</div>}
        </div>
        {!editing && editEnabled ? (
          <Button
            variant="ghost"
            size="sm"
            className="opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
            aria-label={t("detail.editField", { field: label })}
            onClick={onEdit}
          >
            {t("detail.edit")}
          </Button>
        ) : null}
      </div>
    </div>
  )
}

function EditActions({
  saving,
  onSave,
  onCancel,
}: {
  saving: boolean
  onSave: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  return (
    <div className="mt-3 flex items-center gap-2">
      <Button size="sm" disabled={saving} onClick={onSave}>
        {saving ? t("form.saving") : t("form.save")}
      </Button>
      <Button variant="outline" size="sm" disabled={saving} onClick={onCancel}>
        {t("form.cancel")}
      </Button>
    </div>
  )
}

export function ContactDetailView({
  detail,
  onSaved,
  onNotFound,
}: {
  detail: ContactDetail
  onSaved: (detail: ContactDetail) => void
  onNotFound: () => void
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [editing, setEditing] = useState<EditingSection>(null)
  const [saving, setSaving] = useState(false)
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
    defaultValues: valuesFromDetail(detail),
  })

  useEffect(() => {
    form.reset(valuesFromDetail(detail))
    setEditing(null)
  }, [detail, form])

  function cancelEdit() {
    form.reset(valuesFromDetail(detail))
    setEditing(null)
  }

  function startEditing(section: Exclude<EditingSection, null>) {
    form.reset(valuesFromDetail(detail))
    setEditing(section)
  }

  function invalid(_errors: FieldErrors<ContactFormValues>) {
    toast.error(t("validation.checkFields"))
  }

  const save = form.handleSubmit(async (values) => {
    const input: ContactInput = {
      displayName: values.displayName,
      channelId: detail.contact.sourceChannelId,
      stage: values.stage,
      notes: values.notes,
      methods: methodsFromDetail(detail, values),
    }
    setSaving(true)
    try {
      const saved = await updateContact(detail.contact.id, input)
      toast.success(t("form.updated"))
      onSaved(saved)
    } catch (error) {
      if (error instanceof ApiError) {
        if (error.code === "AUTH_REQUIRED") {
          navigate("/login", { replace: true })
          return
        }
        if (error.code === "CONTACT_NOT_FOUND") {
          onNotFound()
          return
        }
        toast.error(apiErrorMessage(error, ["displayName", "stage", "methods", "notes"]))
        return
      }
      toast.error(t("form.networkError"))
    } finally {
      setSaving(false)
    }
  }, invalid)

  const empty = <span className="text-muted-foreground">{t("detail.empty")}</span>
  const stage = form.watch("stage") as ContactStage

  return (
    <div className="flex flex-col gap-7">
      <section>
        <h3 className="mb-2 text-sm font-medium">{t("detail.basicInformation")}</h3>
        <div className="divide-y">
          <DetailRow
            label={t("columns.name")}
            value={detail.contact.displayName || empty}
            editing={editing === "name"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("name")}
          >
            <Input {...form.register("displayName")} autoFocus />
            <FieldError errors={[form.formState.errors.displayName]} />
            <EditActions saving={saving} onSave={() => void save()} onCancel={cancelEdit} />
          </DetailRow>

          <DetailRow
            label={t("columns.stage")}
            value={t(`stages.${detail.contact.stage}`)}
            editing={editing === "stage"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("stage")}
          >
            <NativeSelect {...form.register("stage")} autoFocus value={stage}>
              <option value="visitor">{t("stages.visitor")}</option>
              <option value="lead">{t("stages.lead")}</option>
              <option value="customer">{t("stages.customer")}</option>
            </NativeSelect>
            <EditActions saving={saving} onSave={() => void save()} onCancel={cancelEdit} />
          </DetailRow>

          <div className="flex items-start gap-3 px-2 py-3 text-sm">
            <div className="w-28 shrink-0 text-muted-foreground">{t("detail.sourceChannel")}</div>
            <div className="min-w-0 flex-1">
              {detail.sourceChannel
                ? `${t(`channelTypes.${detail.sourceChannel.type}`, { defaultValue: detail.sourceChannel.type })} · ${detail.sourceChannel.name}`
                : empty}
            </div>
          </div>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("detail.contactMethods")}</h3>
        <DetailRow
          label={t("detail.emailAndPhone")}
          value={
            <div className="grid gap-1.5">
              <div>{t("form.email")}: {form.getValues("email") || empty}</div>
              <div>{t("form.phone")}: {form.getValues("phone") || empty}</div>
            </div>
          }
          editing={editing === "methods"}
          editEnabled={editing === null && !saving}
          onEdit={() => startEditing("methods")}
        >
          <div className="grid gap-4">
            <Field>
              <FieldLabel htmlFor="contact-detail-email">{t("form.email")}</FieldLabel>
              <Input id="contact-detail-email" type="email" {...form.register("email")} autoFocus />
              <FieldError errors={[form.formState.errors.email]} />
            </Field>
            <Controller
              name="phone"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field>
                  <FieldLabel htmlFor="contact-detail-phone">{t("form.phone")}</FieldLabel>
                  <PhoneInput
                    ref={field.ref}
                    id="contact-detail-phone"
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
          </div>
          <EditActions saving={saving} onSave={() => void save()} onCancel={cancelEdit} />
        </DetailRow>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("form.notes")}</h3>
        <DetailRow
          label={t("form.notes")}
          value={detail.contact.notes || empty}
          editing={editing === "notes"}
          editEnabled={editing === null && !saving}
          onEdit={() => startEditing("notes")}
        >
          <Textarea {...form.register("notes")} autoFocus rows={5} />
          <FieldError errors={[form.formState.errors.notes]} />
          <EditActions saving={saving} onSave={() => void save()} onCancel={cancelEdit} />
        </DetailRow>
      </section>

      <section>
        <h3 className="mb-3 text-sm font-medium">{t("detail.otherInformation")}</h3>
        <dl className="grid gap-4 px-2 text-sm">
          <div className="flex gap-3">
            <dt className="w-28 shrink-0 text-muted-foreground">{t("columns.addedAt")}</dt>
            <dd>{formatDateTime(detail.contact.createdAt)}</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-28 shrink-0 text-muted-foreground">{t("detail.linkedChannels")}</dt>
            <dd className="grid gap-2">
              {detail.channelIdentities.length > 0
                ? detail.channelIdentities.map((identity) => (
                    <div key={`${identity.channelId}:${identity.externalId}`}>
                      <div>{identity.channelName}</div>
                      <div className="text-xs text-muted-foreground">
                        {identity.displayName || identity.externalId}
                      </div>
                    </div>
                  ))
                : empty}
            </dd>
          </div>
        </dl>
      </section>
    </div>
  )
}
