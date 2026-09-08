/** 移动端群主添加真人成员，保留选择并返回原群详情。 */
import { useEffect, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useController, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useOutletContext } from "react-router"
import { z } from "zod"

import {
  addGroupConversationMembers,
  ConversationStatus,
  type MemberOption,
} from "@/api"
import {
  mobileGroupMemberLimit,
  type MobileGroupDetailsContext,
} from "@/apps/mobile/mobile-group-context"
import { MobileGroupMemberPicker } from "@/apps/mobile/mobile-group-member-picker"
import { useMobileBack } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader } from "@/apps/mobile/mobile-page"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { Button } from "@/components/ui/button"

/** 按当前群成员和剩余名额选择真人，成功后刷新群聊事实。 */
export function MobileAddGroupMembersPage() {
  const { t } = useTranslation("mobile")
  const { t: tInbox } = useTranslation("inbox")
  const { group, canManage, busy, onSave } =
    useOutletContext<MobileGroupDetailsContext>()
  const { identity } = useMobileWorkspace()
  const close = useMobileBack(`/inbox/group/${group.id}/details`)
  const alive = useRef(false)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])
  const existingIDs = group.participants.map((member) => member.identityId)
  const remaining = Math.max(0, mobileGroupMemberLimit - existingIDs.length)
  const schema = z.object({
    members: z.array(z.custom<MemberOption>())
      .min(1, tInbox("groupMembersRequired"))
      .max(remaining, tInbox("groupMemberLimitReached")),
  })
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { members: [] },
  })
  const { field } = useController({ control: form.control, name: "members" })
  useEffect(() => {
    // 已从其他端入群的成员退出候选和勾选，其余选择继续保留。
    const selected = field.value.filter((member) =>
      !group.participants.some((participant) => participant.identityId === member.id),
    )
    if (selected.length !== field.value.length) field.onChange(selected)
  }, [field.value, field.onChange, group.participants])
  const notice = !canManage
    ? t(
        group.status === ConversationStatus.ConversationStatusArchived
          ? "group.addArchived"
          : "group.addOwnerOnly",
      )
    : remaining === 0
      ? tInbox("groupMemberLimitReached")
      : null

  /** 批量添加所选成员，离开页面后忽略迟到的返回导航。 */
  async function addMembers(values: z.infer<typeof schema>) {
    if (!canManage) return
    const success = await onSave(() =>
      addGroupConversationMembers(group.id, {
        memberIdentityIds: values.members.map((member) => member.id),
      }),
    )
    if (success && alive.current) close()
  }

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobilePageHeader
        title={t("group.addMembers")}
        backTo={`/inbox/group/${group.id}/details`}
        backDisabled={busy}
      />
      <form
        className="min-h-0 flex-1 space-y-9 overflow-y-auto p-4"
        noValidate
        onSubmit={form.handleSubmit(addMembers)}
      >
        <div className="space-y-3">
          {notice ? (
            <p className="text-sm text-muted-foreground" role="status">
              {notice}
            </p>
          ) : null}
          <MobileGroupMemberPicker
            showSelectionSummary={false}
            currentIdentityID={identity.user.identityId}
            excludedIdentityIDs={existingIDs}
            selectionLimit={remaining}
            selected={field.value}
            onChange={field.onChange}
            onBlur={field.onBlur}
            inputRef={field.ref}
            disabled={busy || !canManage}
          />
        </div>
        <div>
          <Button
            type="submit"
            className="min-h-11 w-full"
            disabled={
              busy || !canManage ||
              !field.value.length || field.value.length > remaining
            }
          >
            {t("group.complete")}
          </Button>
        </div>
      </form>
    </section>
  )
}
