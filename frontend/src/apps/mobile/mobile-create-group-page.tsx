/** 移动端群名称、初始成员表单和创建后的聊天导航。 */
import { useEffect, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useController, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import { createGroupConversation, isApiError, type MemberOption } from "@/api"
import { MobileGroupMemberPicker } from "@/apps/mobile/mobile-group-member-picker"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { Button } from "@/components/ui/button"
import { FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 创建群聊并替换表单路由，保留原消息列表的返回来源。 */
export function MobileCreateGroupPage() {
  const { t } = useTranslation("mobile")
  const { t: tInbox } = useTranslation("inbox")
  const { identity } = useMobileWorkspace()
  const { inboxURL } = useMobileNavigation()
  const location = useLocation()
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const mounted = useRef(false)
  const submitting = useRef(false)
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  // 已选成员作为表单值保存，候选刷新后仍可移除。
  const schema = z.object({
    title: z.string().trim()
      .min(1, tInbox("groupTitleRequired"))
      .max(100, tInbox("groupTitleTooLong")),
    members: z.array(z.custom<MemberOption>())
      .min(1, tInbox("groupMembersRequired"))
      .max(99, tInbox("groupMembersTooMany")),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { title: "", members: [] },
  })
  const { field } = useController({
    control: form.control,
    name: "members",
  })
  const title = form.watch("title")

  /** 提交初始成员，失败时保留表单，离开页面后忽略返回结果。 */
  async function create(values: z.infer<typeof schema>) {
    if (submitting.current) return
    submitting.current = true
    setSaving(true)
    try {
      const conversation = await createGroupConversation({
        title: values.title,
        memberIdentityIds: values.members.map((member) => member.id),
        description: "",
        imageFileId: "",
      })
      if (!mounted.current) return
      void invalidate(resourceKeys.inbox())
      console.info("移动端创建群聊成功", { conversationID: conversation.id })
      void navigate(`/inbox/group/${conversation.id}`, {
        replace: true,
        state: {
          mobileBack: Boolean(
            (location.state as { mobileBack?: boolean } | null)?.mobileBack,
          ),
        },
      })
    } catch (error) {
      if (!mounted.current || recoverSession(error, navigate)) return
      console.warn("移动端创建群聊失败", { error })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["title", "memberIdentityIds"])
          : tInbox("groupCreateError"),
      )
    } finally {
      submitting.current = false
      if (mounted.current) setSaving(false)
    }
  }

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t("group.create")}
        backTo={inboxURL}
        actions={
          <Button
            type="submit"
            form="mobile-create-group"
            variant="ghost"
            className="min-h-11"
            disabled={saving || !title.trim() || field.value.length === 0}
          >
            {t("group.complete")}
          </Button>
        }
      />
      <form
        id="mobile-create-group"
        className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-4"
        noValidate
        onSubmit={form.handleSubmit(create)}
      >
        <div className="space-y-5">
          <div className="space-y-2">
            <FieldLabel htmlFor="mobile-group-title" required>
              {tInbox("groupTitleLabel")}
            </FieldLabel>
            <Input
              {...form.register("title")}
              id="mobile-group-title"
              className="min-h-11 md:text-base"
              autoComplete="off"
              maxLength={100}
              required
              disabled={saving}
            />
          </div>
          <MobileGroupMemberPicker
            currentIdentityID={identity.user.identityId}
            selected={field.value}
            onChange={field.onChange}
            onBlur={field.onBlur}
            inputRef={field.ref}
            disabled={saving}
          />
        </div>
      </form>
    </section>
  )
}
