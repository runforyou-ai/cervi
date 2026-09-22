/** 联系人详情和分节编辑。 */
import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
} from "react"
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
  type ContactMethodInput,
} from "@/api"
import { DetailEditRow } from "@/components/form/detail-edit-row"
import { InlineEditField } from "@/components/form/inline-edit-field"
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
import { useImmediateSave } from "@/hooks/use-immediate-save"
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

/** 各分节校验和提示使用的字段。 */
const sectionFields = {
  name: ["displayName"],
  stage: ["stage"],
  methods: ["email", "phone"],
  notes: ["notes"],
} satisfies Record<Exclude<EditingSection, null>, (keyof ContactFormValues)[]>

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
  const saveState = useImmediateSave()
  const { saving } = saveState
  const root = useRef<HTMLDivElement>(null)
  // Esc 放弃后，编辑区卸载触发的失焦跳过保存。
  const cancelled = useRef(false)
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
    cancelled.current = true
    form.reset(valuesFromDetail(detail))
    setEditing(null)
  }

  /** 开始编辑指定分节。 */
  function startEditing(section: Exclude<EditingSection, null>) {
    cancelled.current = false
    form.reset(valuesFromDetail(detail))
    setEditing(section)
  }

  /**
   * 保存指定分节：先在可编辑状态下校验，原生提示显示在当前分节的输入框上；与当前资料相同时直接退出编辑。
   * 请求发出后即使侧栏关闭也写完并刷新缓存，只在仍打开时更新编辑状态。失败时文本输入保留草稿和编辑态，
   * 传入 draft 的即选即存控件恢复为已保存的值。
   */
  async function saveContact(
    section: Exclude<EditingSection, null>,
    changed?: ContactFormValues,
  ) {
    const draft = changed ?? form.getValues()
    if (cancelled.current || saveState.isSaving()) return
    const fields = sectionFields[section]
    if (!(await form.trigger(fields, { shouldFocus: true }))) return
    const parsed = schema.safeParse(draft)
    if (!parsed.success) {
      // 跨字段规则（至少保留一项身份信息）提示在当前分节的首个输入框上。
      const input = root.current?.querySelector<HTMLInputElement>(
        `[name="${fields[0]}"]`,
      )
      if (input) {
        input.setCustomValidity(parsed.error.issues[0]?.message ?? "")
        input.reportValidity()
        input.addEventListener("input", () => input.setCustomValidity(""), {
          once: true,
        })
        input.focus()
      }
      return
    }
    const current = valuesFromDetail(detail)
    if (
      draft.displayName === current.displayName &&
      draft.stage === current.stage &&
      draft.email === current.email &&
      draft.phone === current.phone &&
      draft.notes === current.notes
    ) {
      setEditing(null)
      return
    }
    const request = saveState.begin()
    if (request === null) return
    try {
      const saved = await updateContact(detail.contact.id, {
        displayName: draft.displayName,
        channelId: detail.contact.sourceChannelId,
        stage: draft.stage,
        notes: draft.notes,
        methods: methodsFromDetail(detail, draft),
      })
      onSaved(saved)
      if (saveState.isCurrent(request)) setEditing(null)
    } catch (error) {
      if (changed && saveState.isCurrent(request)) form.reset(valuesFromDetail(detail))
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        console.warn("联系人不存在", { contact_id: detail.contact.id })
        // 只在详情仍是这次请求所属的联系人时关闭，迟到的结果不影响之后打开的详情。
        if (saveState.isCurrent(request)) onNotFound()
        return
      }
      console.warn("保存联系人失败", error)
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["displayName", "stage", "methods", "notes"])
          : t("form.networkError"),
      )
    } finally {
      saveState.finish(request)
    }
  }

  /** 编辑区按 Esc 放弃修改。 */
  function handleEscape(event: KeyboardEvent) {
    if (event.key !== "Escape") return
    event.preventDefault()
    event.stopPropagation()
    cancelEdit()
  }

  /** 焦点离开整个编辑区时保存。 */
  function handleSectionBlur(event: FocusEvent<HTMLElement>) {
    if (event.currentTarget.contains(event.relatedTarget)) return
    void saveContact("methods")
  }

  const empty = (
    <span className="text-muted-foreground">{t("detail.empty")}</span>
  )
  const stage = form.watch("stage")

  return (
    <div ref={root} className="flex flex-col gap-7">
      <section>
        <h3 className="mb-2 text-sm font-medium">
          {t("detail.basicInformation")}
        </h3>
        <div className="divide-y">
          <InlineEditField
            control={form.control}
            name="displayName"
            label={t("columns.name")}
            empty={empty}
            disabled={saving}
            editing={editing === "name"}
            editEnabled={editing === null && !saving}
            onEditingChange={(next) => {
              if (next) startEditing("name")
            }}
            onCommit={() => void saveContact("name")}
            onCancel={cancelEdit}
          />

          <DetailEditRow
            label={t("columns.stage")}
            value={
              detail.contact.stage ? t(`stages.${detail.contact.stage}`) : ""
            }
            editing={editing === "stage"}
            editEnabled={editing === null && !saving}
            required
            onEdit={() => startEditing("stage")}
          >
            <NativeSelect
              {...form.register("stage")}
              autoFocus
              value={stage}
              disabled={saving}
              onChange={(event) => {
                const next = event.target.value as ContactFormValues["stage"]
                form.setValue("stage", next)
                void saveContact("stage", { ...form.getValues(), stage: next })
              }}
              onBlur={() => {
                if (!saveState.isSaving()) cancelEdit()
              }}
              onKeyDown={handleEscape}
            >
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
          <div
            className="grid gap-4"
            onBlur={handleSectionBlur}
            onKeyDown={(event) => {
              handleEscape(event)
              // 回车提交邮箱和电话，焦点离开编辑区后保存。
              if (event.key === "Enter" && event.target instanceof HTMLInputElement) {
                event.preventDefault()
                event.target.blur()
              }
            }}
          >
            <Field>
              <FieldLabel htmlFor="contact-detail-email">
                {t("form.email")}
              </FieldLabel>
              <Input
                id="contact-detail-email"
                type="email"
                {...form.register("email")}
                autoFocus
                disabled={saving}
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
                    disabled={saving}
                  />
                </Field>
              )}
            />
          </div>
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
          <Textarea
            {...form.register("notes")}
            autoFocus
            rows={5}
            disabled={saving}
            onBlur={() => void saveContact("notes")}
            onKeyDown={handleEscape}
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
