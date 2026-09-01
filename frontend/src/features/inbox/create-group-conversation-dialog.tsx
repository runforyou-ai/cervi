/** 企业内部群聊创建表单。 */
import { useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon, SearchIcon, UserRoundIcon } from "lucide-react"
import { useController, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import {
  createGroupConversation,
  isApiError,
  isGroupInboxConversation,
  OrganizationIdentityType,
  type GroupInboxConversationData,
} from "@/api"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { listAllMemberOptions } from "@/features/inbox/list-all-member-options"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const groupTitleMaxLength = 100
const groupAdditionalMemberMaxCount = 99

/** 创建群聊表单校验规则。 */
function createGroupConversationSchema(messages: {
  titleRequired: string
  titleTooLong: string
  membersRequired: string
  membersTooMany: string
}) {
  return z.object({
    title: z
      .string()
      .trim()
      .min(1, messages.titleRequired)
      .max(groupTitleMaxLength, messages.titleTooLong),
    memberIdentityIds: z
      .array(z.string())
      .min(1, messages.membersRequired)
      .max(groupAdditionalMemberMaxCount, messages.membersTooMany),
  })
}

type GroupConversationValues = z.infer<
  ReturnType<typeof createGroupConversationSchema>
>

/** 选择初始成员并创建企业内部群聊。 */
export function CreateGroupConversationDialog({
  open,
  currentIdentityID,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  currentIdentityID: string
  onOpenChange: (open: boolean) => void
  onCreated: (conversation: GroupInboxConversationData) => void
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const [query, setQuery] = useState("")
  const schema = useMemo(
    () =>
      createGroupConversationSchema({
        titleRequired: t("groupTitleRequired"),
        titleTooLong: t("groupTitleTooLong"),
        membersRequired: t("groupMembersRequired"),
        membersTooMany: t("groupMembersTooMany"),
      }),
    [t],
  )
  const form = useForm<GroupConversationValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { title: "", memberIdentityIds: [] },
  })
  const { field: memberIdentityIDsField } = useController({
    control: form.control,
    name: "memberIdentityIds",
  })
  const selectedIdentityIDs = memberIdentityIDsField.value
  const { data, loading, error, refresh } = useResource(
    resourceKeys.memberOptions(),
    listAllMemberOptions,
    { enabled: open, staleTime: 0 },
  )
  const candidates = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase()
    return (data ?? []).filter(
      (member) =>
        member.id !== currentIdentityID &&
        member.type ===
          OrganizationIdentityType.OrganizationIdentityTypeUser &&
        (!normalizedQuery ||
          member.displayName.toLocaleLowerCase().includes(normalizedQuery)),
    )
  }, [currentIdentityID, data, query])

  /** 创建群聊并关闭表单。 */
  async function create(values: GroupConversationValues) {
    try {
      const conversation = await createGroupConversation({
        title: values.title.trim(),
        memberIdentityIds: values.memberIdentityIds,
      })
      if (!isGroupInboxConversation(conversation)) {
        throw new Error("企业群聊响应结构无效")
      }
      onCreated(conversation)
      changeOpen(false)
    } catch (createError) {
      if (recoverSession(createError, navigate)) return
      console.warn("创建企业内部群聊失败", { error: createError })
      toast.error(
        isApiError(createError)
          ? apiErrorMessage(createError, ["title", "memberIdentityIds"])
          : t("groupCreateError"),
      )
    }
  }

  /** 关闭时清空尚未提交的群聊表单。 */
  function changeOpen(nextOpen: boolean) {
    if (!nextOpen) {
      form.reset()
      setQuery("")
    }
    onOpenChange(nextOpen)
  }

  const { isSubmitting } = form.formState

  return (
    <Dialog open={open} onOpenChange={changeOpen}>
      <DialogContent className="max-h-[min(46rem,calc(100svh-2rem))] overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t("groupCreateTitle")}</DialogTitle>
          <DialogDescription>{t("groupCreateDescription")}</DialogDescription>
        </DialogHeader>
        <form
          className="min-h-0 space-y-9"
          onSubmit={form.handleSubmit(create)}
          noValidate
        >
          <div className="grid min-h-0 gap-5">
            <div className="space-y-1.5">
              <label htmlFor="group-title" className="text-sm font-medium">
                {t("groupTitleLabel")}
              </label>
              <Input
                {...form.register("title")}
                id="group-title"
                autoComplete="off"
                maxLength={groupTitleMaxLength}
                required
              />
            </div>
            <div className="grid min-h-0 gap-2">
              <div className="flex items-center justify-between gap-3">
                <label
                  htmlFor="group-member-search"
                  className="text-sm font-medium"
                >
                  {t("groupMembersLabel")}
                </label>
                <span className="text-xs text-muted-foreground">
                  {t("groupMembersSelected", {
                    count: selectedIdentityIDs.length,
                  })}
                </span>
              </div>
              <div className="relative">
                <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  id="group-member-search"
                  ref={memberIdentityIDsField.ref}
                  value={query}
                  autoComplete="off"
                  className="pl-9"
                  onChange={(event) => setQuery(event.target.value)}
                />
              </div>
              <ScrollArea className="h-64 rounded-md border">
                {loading ? (
                  <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
                    {t("groupMembersLoading")}
                  </div>
                ) : error ? (
                  <div className="flex h-64 flex-col items-center justify-center p-6 text-center">
                    <p className="text-sm text-muted-foreground">
                      {t("groupMembersLoadError")}
                    </p>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="mt-3"
                      onClick={() => void refresh()}
                    >
                      {t("messagesRetry")}
                    </Button>
                  </div>
                ) : candidates.length === 0 ? (
                  <p className="px-6 py-12 text-center text-sm text-muted-foreground">
                    {t("groupMembersEmpty")}
                  </p>
                ) : (
                  <div className="grid p-1.5">
                    {candidates.map((member) => {
                      const selected = selectedIdentityIDs.includes(member.id)
                      const selectionFull =
                        selectedIdentityIDs.length >=
                        groupAdditionalMemberMaxCount
                      return (
                        <label
                          key={member.id}
                          className="flex items-center gap-3 rounded-md px-3 py-2.5 transition-colors hover:bg-muted"
                        >
                          <input
                            type="checkbox"
                            name={memberIdentityIDsField.name}
                            checked={selected}
                            disabled={!selected && selectionFull}
                            className="size-4 rounded border-input accent-primary"
                            onBlur={memberIdentityIDsField.onBlur}
                            onChange={(event) => {
                              memberIdentityIDsField.onChange(
                                event.target.checked
                                  ? [...selectedIdentityIDs, member.id]
                                  : selectedIdentityIDs.filter(
                                      (identityID) => identityID !== member.id,
                                    ),
                              )
                            }}
                          />
                          <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground">
                            {member.displayName ? (
                              Array.from(
                                member.displayName,
                              )[0]?.toLocaleUpperCase()
                            ) : (
                              <UserRoundIcon className="size-4" />
                            )}
                          </span>
                          <span className="min-w-0 flex-1 truncate text-sm">
                            {member.displayName}
                          </span>
                        </label>
                      )
                    })}
                  </div>
                )}
              </ScrollArea>
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={isSubmitting}
              onClick={() => changeOpen(false)}
            >
              {t("groupCreateCancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? (
                <LoaderCircleIcon className="animate-spin" />
              ) : null}
              {t("groupCreateSubmit")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
