/** 联系人详情和分节编辑。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  ContactMethodType,
  ContactStage,
  isApiError,
  isNotFoundApiError,
  updateContact,
  type ContactDetail,
  type ContactInput,
  type ContactMethodInput,
} from "@/api"
import {
  DetailEditActions,
  DetailEditRow,
} from "@/components/form/detail-edit-row"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { NativeSelect } from "@/components/ui/native-select"
import { PhoneInput } from "@/components/ui/phone-input"
import { Textarea } from "@/components/ui/textarea"
import { channelTypeLabel } from "@/features/contacts/external/contact-labels"
import {
  createContactSchema,
  type ContactFormValues,
} from "@/features/contacts/external/contact-schema"
import { useDateTime } from "@/hooks/use-date-time"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

type EditingSection = "name" | "stage" | "methods" | "notes" | null

/** 把联系人详情转换为表单值。 */
function valuesFromDetail(detail: ContactDetail): ContactFormValues {
  return {
    displayName: detail.contact.displayName ?? "",
    channelId: detail.contact.sourceChannelId,
    stage: detail.contact.stage,
    email:
      detail.methods.find(
        (method) => method.type === ContactMethodType.ContactMethodTypeEmail,
      )?.value ?? "",
    phone:
      detail.methods.find(
        (method) => method.type === ContactMethodType.ContactMethodTypePhone,
      )?.value ?? "",
    notes: detail.contact.notes ?? "",
  }
}

/** 用表单值更新每类联系方式的首项，其余项保持不变。 */
function methodsFromDetail(
  detail: ContactDetail,
  values: ContactFormValues,
): ContactMethodInput[] {
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
        label: method.label ?? "",
        isPrimary: method.isPrimary,
      })
      continue
    }
    methods.push({
      type: method.type,
      value: method.value,
      label: method.label ?? "",
      isPrimary: method.isPrimary,
    })
  }

  for (const type of [
    ContactMethodType.ContactMethodTypeEmail,
    ContactMethodType.ContactMethodTypePhone,
  ]) {
    if (!handled[type] && editedValues[type]) {
      methods.push({
        type,
        value: editedValues[type],
        label: "",
        isPrimary: true,
      })
    }
  }
  return methods
}

/** 分节编辑联系人详情。 */
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
    shouldUseNativeValidation: true,
    defaultValues: valuesFromDetail(detail),
  })
  useEffect(() => {
    form.reset(valuesFromDetail(detail))
    setEditing(null)
  }, [detail, form])

  /** 取消当前分节编辑。 */
  function cancelEdit() {
    form.reset(valuesFromDetail(detail))
    setEditing(null)
  }

  /** 开始编辑指定分节。 */
  function startEditing(section: Exclude<EditingSection, null>) {
    form.reset(valuesFromDetail(detail))
    setEditing(section)
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
      console.info("联系人已保存", { contact_id: detail.contact.id })
      toast.success(t("form.updated"))
      onSaved(saved)
    } catch (error) {
      if (recoverSession(error, navigate)) {
        return
      }
      if (isNotFoundApiError(error)) {
        console.warn("联系人不存在", { contact_id: detail.contact.id })
        onNotFound()
        return
      }
      if (isApiError(error)) {
        console.warn("保存联系人失败", error)
        toast.error(
          apiErrorMessage(error, ["displayName", "stage", "methods", "notes"]),
        )
        return
      }
      console.warn("保存联系人失败", error)
      toast.error(t("form.networkError"))
    } finally {
      setSaving(false)
    }
  })

  const empty = (
    <span className="text-muted-foreground">{t("detail.empty")}</span>
  )
  const stage = form.watch("stage")

  return (
    <div className="flex flex-col gap-7">
      <section>
        <h3 className="mb-2 text-sm font-medium">
          {t("detail.basicInformation")}
        </h3>
        <div className="divide-y">
          <DetailEditRow
            label={t("columns.name")}
            value={detail.contact.displayName || empty}
            editing={editing === "name"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("name")}
          >
            <Input {...form.register("displayName")} autoFocus />
            <DetailEditActions
              saving={saving}
              onSave={() => void save()}
              onCancel={cancelEdit}
            />
          </DetailEditRow>

          <DetailEditRow
            label={t("columns.stage")}
            value={
              detail.contact.stage ? t(`stages.${detail.contact.stage}`) : ""
            }
            editing={editing === "stage"}
            editEnabled={editing === null && !saving}
            onEdit={() => startEditing("stage")}
          >
            <NativeSelect {...form.register("stage")} autoFocus value={stage}>
              <option value={ContactStage.ContactStageVisitor}>
                {t("stages.visitor")}
              </option>
              <option value={ContactStage.ContactStageLead}>
                {t("stages.lead")}
              </option>
              <option value={ContactStage.ContactStageCustomer}>
                {t("stages.customer")}
              </option>
            </NativeSelect>
            <DetailEditActions
              saving={saving}
              onSave={() => void save()}
              onCancel={cancelEdit}
            />
          </DetailEditRow>

          <div className="flex items-start gap-3 px-2 py-3 text-sm">
            <div className="w-28 shrink-0 text-muted-foreground">
              {t("detail.sourceChannel")}
            </div>
            <div className="min-w-0 flex-1">
              {detail.sourceChannel
                ? `${channelTypeLabel(detail.sourceChannel.type, t)} · ${detail.sourceChannel.name}`
                : empty}
            </div>
          </div>
        </div>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">
          {t("detail.contactMethods")}
        </h3>
        <DetailEditRow
          label={t("detail.emailAndPhone")}
          value={
            <div className="grid gap-1.5">
              <div>
                {t("form.email")}: {form.getValues("email") || empty}
              </div>
              <div>
                {t("form.phone")}: {form.getValues("phone") || empty}
              </div>
            </div>
          }
          editing={editing === "methods"}
          editEnabled={editing === null && !saving}
          onEdit={() => startEditing("methods")}
        >
          <div className="grid gap-4">
            <Field>
              <FieldLabel htmlFor="contact-detail-email">
                {t("form.email")}
              </FieldLabel>
              <Input
                id="contact-detail-email"
                type="email"
                {...form.register("email")}
                autoFocus
              />
            </Field>
            <Controller
              name="phone"
              control={form.control}
              render={({ field, fieldState }) => (
                <Field>
                  <FieldLabel htmlFor="contact-detail-phone">
                    {t("form.phone")}
                  </FieldLabel>
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
                </Field>
              )}
            />
          </div>
          <DetailEditActions
            saving={saving}
            onSave={() => void save()}
            onCancel={cancelEdit}
          />
        </DetailEditRow>
      </section>

      <section>
        <h3 className="mb-2 text-sm font-medium">{t("form.notes")}</h3>
        <DetailEditRow
          label={t("form.notes")}
          value={detail.contact.notes || empty}
          editing={editing === "notes"}
          editEnabled={editing === null && !saving}
          onEdit={() => startEditing("notes")}
        >
          <Textarea {...form.register("notes")} autoFocus rows={5} />
          <DetailEditActions
            saving={saving}
            onSave={() => void save()}
            onCancel={cancelEdit}
          />
        </DetailEditRow>
      </section>

      <section>
        <h3 className="mb-3 text-sm font-medium">
          {t("detail.otherInformation")}
        </h3>
        <dl className="grid gap-4 px-2 text-sm">
          <div className="flex gap-3">
            <dt className="w-28 shrink-0 text-muted-foreground">
              {t("columns.addedAt")}
            </dt>
            <dd>{formatDateTime(detail.contact.createdAt)}</dd>
          </div>
          <div className="flex gap-3">
            <dt className="w-28 shrink-0 text-muted-foreground">
              {t("detail.linkedChannels")}
            </dt>
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
