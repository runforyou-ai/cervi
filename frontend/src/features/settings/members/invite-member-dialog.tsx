/** 邀请成员弹窗：填写邮箱、显示名称和角色后得到邀请链接；也用于展示重新生成的链接。 */
import { useEffect, useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  RoleKind,
  createInvitation,
  isApiError,
  type InvitationCreated,
  type RoleData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field"
import { RoleSelectField } from "@/features/contacts/role-select-field"
import { apiErrorMessage } from "@/lib/form-errors"
import { displayNamePattern } from "@/lib/display-name"
import { recoverSession } from "@/lib/session-navigation"

/** 展示只返回一次的邀请链接并提供复制。 */
export function InvitationLink({ created }: { created: InvitationCreated }) {
  const { t } = useTranslation("contacts")
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!copied) return
    const timeout = window.setTimeout(() => setCopied(false), 2000)
    return () => window.clearTimeout(timeout)
  }, [copied])

  /** 复制邀请链接并反馈结果。 */
  async function copy() {
    try {
      await navigator.clipboard.writeText(created.link)
      setCopied(true)
    } catch (error) {
      console.warn("复制邀请链接失败", error)
      toast.error(t("members.invite.copyError"))
    }
  }

  return (
    <Field>
      <FieldLabel>{t("members.invite.link")}</FieldLabel>
      <div className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
        <code className="flex min-h-8 min-w-0 flex-1 items-center font-mono text-xs break-all">{created.link}</code>
        <Button type="button" variant="outline" size="sm" className="shrink-0" onClick={() => void copy()}>
          {copied ? t("members.invite.copied") : t("members.invite.copy")}
        </Button>
      </div>
      <FieldDescription>
        {created.emailSent
          ? t("members.invite.linkHelpEmailSent", { email: created.invitation.email })
          : t("members.invite.linkHelp")}
      </FieldDescription>
    </Field>
  )
}

/** 发起邀请，成功后在同一弹窗内展示邀请链接。 */
export function InviteMemberDialog({
  open,
  roles,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  roles: RoleData[]
  onOpenChange: (open: boolean) => void
  onCreated: () => void
}) {
  const { t } = useTranslation("contacts")
  const [created, setCreated] = useState<InvitationCreated | null>(null)

  // 关闭弹窗后下次打开重新填写。
  useEffect(() => {
    if (!open) setCreated(null)
  }, [open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{created ? t("members.invite.createdTitle") : t("members.invite.title")}</DialogTitle>
          <DialogDescription>{created ? t("members.invite.createdDescription") : t("members.invite.description")}</DialogDescription>
        </DialogHeader>
        {created ? (
          <div className="space-y-9">
            <InvitationLink created={created} />
            <div className="flex justify-end">
              <Button type="button" onClick={() => onOpenChange(false)}>
                {t("members.invite.done")}
              </Button>
            </div>
          </div>
        ) : open && roles.length === 0 ? (
          <LoadingIndicator className="py-10">
            <span className="sr-only">Loading</span>
          </LoadingIndicator>
        ) : open ? (
          <InviteMemberForm
            roles={roles}
            onCancel={() => onOpenChange(false)}
            onCreated={(value) => {
              setCreated(value)
              onCreated()
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

/** 邀请表单，显示名称选填，为空时使用受邀人的账号名称。 */
function InviteMemberForm({
  roles,
  onCancel,
  onCreated,
}: {
  roles: RoleData[]
  onCancel: () => void
  onCreated: (created: InvitationCreated) => void
}) {
  const { t } = useTranslation(["contacts", "common"])
  const navigate = useNavigate()
  const schema = useMemo(
    () =>
      z.object({
        email: z.string().trim().min(1, t("members.validation.emailRequired")).email(t("members.validation.emailInvalid")),
        displayName: z
          .string()
          .trim()
          .refine((value) => value === "" || displayNamePattern.test(value), t("members.validation.nameInvalid")),
        roleId: z.string().uuid(t("members.validation.roleRequired")),
      }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: {
      email: "",
      displayName: "",
      roleId: roles.find((role) => role.kind === RoleKind.RoleKindMember)?.id ?? roles[0]?.id ?? "",
    },
  })

  /** 提交邀请。 */
  async function submit(values: z.infer<typeof schema>) {
    try {
      onCreated(await createInvitation(values))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("创建邀请失败", error)
      toast.error(isApiError(error) ? apiErrorMessage(error, ["email", "displayName", "roleId"]) : t("members.form.networkError"))
    }
  }

  const { isSubmitting } = form.formState

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <FieldGroup className="gap-5">
        <FormInputField name="email" control={form.control} label={t("members.form.email")} type="email" autoFocus />
        <FormInputField name="displayName" control={form.control} label={t("members.invite.displayName")} required={false} />
        <Controller
          name="roleId"
          control={form.control}
          render={({ field, fieldState }) => (
            <RoleSelectField {...field} id={field.name} aria-invalid={fieldState.invalid} roles={roles} />
          )}
        />
      </FieldGroup>
      <div className="flex items-center justify-end gap-2">
        <Button type="button" variant="outline" disabled={isSubmitting} onClick={onCancel}>
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? <LoaderCircleIcon className="animate-spin" /> : null}
          {isSubmitting ? t("members.invite.submitting") : t("members.invite.submit")}
        </Button>
      </div>
    </form>
  )
}
