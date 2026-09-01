/** 群聊资料栏中的资料编辑和成员管理交互。 */
import {
  useEffect,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  addGroupConversationMembers,
  ConversationStatus,
  getGroupConversation,
  GroupParticipantRole,
  isApiError,
  leaveGroupConversation,
  removeGroupConversationMember,
  transferGroupConversationOwner,
  updateGroupConversation,
  type GroupConversationData,
  type MemberOption,
} from "@/api"
import { DetailEditRow } from "@/components/form/detail-edit-row"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs"
import { GroupParticipantList } from "@/features/inbox/group-participant-list"
import { useDateTime } from "@/hooks/use-date-time"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { useImmediateSave } from "@/hooks/use-immediate-save"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const groupTitleMaxLength = 100

/** 展示群资料中的只读字段。 */
function ReadonlyGroupRow({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <div className="flex min-h-11 items-start gap-3 px-2 py-1.5 text-sm">
      <div className="w-28 shrink-0 pt-1 text-muted-foreground">
        {label}
      </div>
      <div className="min-w-0 flex-1 pt-1">{children}</div>
      <div className="w-14 shrink-0" />
    </div>
  )
}

/** 展示群资料并按通讯录详情模式即时修改群名称。 */
function GroupConversationProfile({
  group,
  createdAt,
  canManage,
  onRename,
}: {
  group: GroupConversationData
  createdAt: string | null
  canManage: boolean
  onRename: (title: string) => Promise<void>
}) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const { formatDateTime } = useDateTime()
  const [editing, setEditing] = useState(false)
  const [title, setTitle] = useState(group.title)
  const saveState = useImmediateSave()
  const owner = group.participants.find(
    (participant) =>
      participant.role ===
      GroupParticipantRole.GroupParticipantRoleOwner,
  )

  useEffect(() => {
    setTitle(group.title)
  }, [group.title])

  /** 放弃尚未提交的群名称。 */
  function cancelEdit() {
    setTitle(group.title)
    setEditing(false)
  }

  /** 保存群名称并退出编辑。 */
  async function saveTitle(input: HTMLInputElement) {
    const nextTitle = title.trim()
    if (!nextTitle) {
      input.setCustomValidity(t("groupTitleRequired"))
      input.reportValidity()
      input.focus()
      return
    }
    if (nextTitle === group.title) {
      cancelEdit()
      return
    }
    const request = saveState.begin()
    if (request === null) return
    try {
      await onRename(nextTitle)
      if (!saveState.isCurrent(request)) return
      setTitle(nextTitle)
      setEditing(false)
    } catch (error) {
      if (!saveState.isCurrent(request)) return
      if (recoverSession(error, navigate)) return
      console.warn("修改群聊名称失败", error)
      cancelEdit()
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["title"])
          : t("groupRenameError"),
      )
    } finally {
      saveState.finish(request)
    }
  }

  /** 处理群名称编辑快捷键。 */
  function handleTitleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Escape") {
      event.preventDefault()
      event.stopPropagation()
      cancelEdit()
      return
    }
    if (event.key === "Enter") {
      event.preventDefault()
      event.currentTarget.blur()
    }
  }

  return (
    <div className="space-y-0.5">
      <DetailEditRow
        label={t("groupTitleLabel")}
        value={group.title}
        editing={editing}
        editEnabled={canManage && !saveState.saving}
        compact
        onEdit={() => {
          setTitle(group.title)
          setEditing(true)
        }}
      >
        <Input
          autoFocus
          value={title}
          required
          maxLength={groupTitleMaxLength}
          disabled={saveState.saving}
          aria-label={t("groupTitleLabel")}
          onChange={(event) => {
            event.currentTarget.setCustomValidity("")
            setTitle(event.target.value)
          }}
          onBlur={(event) => void saveTitle(event.currentTarget)}
          onKeyDown={handleTitleKeyDown}
        />
      </DetailEditRow>
      <ReadonlyGroupRow label={t("groupOwner")}>
        {owner?.displayName ?? "—"}
      </ReadonlyGroupRow>
      <ReadonlyGroupRow label={t("groupCreatedAt")}>
        {createdAt ? formatDateTime(createdAt) : "—"}
      </ReadonlyGroupRow>
    </div>
  )
}

/** 展示群资料读取状态。 */
function GroupResourceState({
  loading,
  failed,
  onRetry,
}: {
  loading: boolean
  failed: boolean
  onRetry: () => void
}) {
  const { t } = useTranslation("inbox")
  if (loading) {
    return (
      <LoadingIndicator className="min-h-48 justify-center">
        {t("groupDetailsLoading")}
      </LoadingIndicator>
    )
  }
  if (!failed) return null
  return (
    <div className="flex min-h-48 flex-col items-center justify-center p-6 text-center">
      <p className="text-sm text-muted-foreground">
        {t("groupDetailsLoadError")}
      </p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="mt-3"
        onClick={onRetry}
      >
        {t("messagesRetry")}
      </Button>
    </div>
  )
}

/** 在现有资料栏框架内协调群资料和成员管理状态。 */
export function GroupConversationContext({
  conversationID,
  currentIdentityID,
  draft,
  onDraftChange,
  onSummaryChange,
  onLeft,
}: {
  conversationID: string
  currentIdentityID: string
  draft: GroupConversationData | null
  onDraftChange: (group: GroupConversationData) => void
  onSummaryChange: (changes: {
    title?: string
    memberCount?: number
    status?: GroupConversationData["status"]
  }) => void
  onLeft: () => void
}) {
  const { t } = useTranslation("inbox")
  const resource = useResource(
    resourceKeys.groupConversation(conversationID),
    () => getGroupConversation(conversationID),
  )
  const invalidate = useResourceInvalidator()
  const group = draft ?? resource.data
  const currentParticipant = group?.participants.find(
    (participant) => participant.identityId === currentIdentityID,
  )
  const dissolved =
    group?.status === ConversationStatus.ConversationStatusArchived
  const canManage =
    !dissolved &&
    currentParticipant?.role ===
    GroupParticipantRole.GroupParticipantRoleOwner

  /** 应用服务端返回的群资料并刷新相关读取。 */
  function applyGroupResult(result: GroupConversationData) {
    onDraftChange(result)
    onSummaryChange({
      title: result.title,
      memberCount: result.participants.length,
    })
    void invalidate(resourceKeys.groupConversation(conversationID), {
      exact: true,
      refetchType: "none",
    })
    void invalidate(resourceKeys.conversationMessages(conversationID), {
      exact: true,
    })
    void invalidate(resourceKeys.inbox())
  }

  /** 修改群名称并采用服务端事实。 */
  async function renameGroup(title: string) {
    const result = await updateGroupConversation(conversationID, { title })
    applyGroupResult(result)
  }

  /** 将选中的有效成员加入群聊。 */
  async function addMembers(members: MemberOption[]) {
    const result = await addGroupConversationMembers(conversationID, {
      memberIdentityIds: members.map((member) => member.id),
    })
    applyGroupResult(result)
  }

  /** 将群主转让给指定成员。 */
  async function transferOwner(identityID: string) {
    const result = await transferGroupConversationOwner(conversationID, {
      ownerIdentityId: identityID,
    })
    applyGroupResult(result)
  }

  /** 将指定成员移出群聊。 */
  async function removeMember(identityID: string) {
    const result = await removeGroupConversationMember(conversationID, {
      memberIdentityId: identityID,
    })
    applyGroupResult(result)
  }

  /** 退出群聊后切换会话，解散群聊后保留当前历史视图。 */
  async function leaveGroup(successorIdentityID?: string) {
    const dissolving = Boolean(
      group &&
        canManage &&
        group.participants.length === 1 &&
        !successorIdentityID,
    )
    await leaveGroupConversation(conversationID, {
      successorIdentityId: successorIdentityID ?? "",
    })
    await invalidate(resourceKeys.groupConversation(conversationID), {
      exact: true,
      refetchType: "none",
    })
    if (dissolving && group) {
      const status = ConversationStatus.ConversationStatusArchived
      onDraftChange({ ...group, status })
      onSummaryChange({ status })
      await invalidate(resourceKeys.conversationMessages(conversationID), {
        exact: true,
      })
      void invalidate(resourceKeys.inbox())
      return
    }
    await invalidate(resourceKeys.conversationMessages(conversationID), {
      exact: true,
      refetchType: "none",
    })
    void invalidate(resourceKeys.inbox())
    onLeft()
  }

  const failed = Boolean(resource.error) || (!resource.loading && !group)

  return (
    <Tabs
      key={conversationID}
      defaultValue="profile"
      className="min-h-0 flex-1"
    >
      <TabsList
        aria-label={t("contextTabsLabel")}
        className="h-auto shrink-0 justify-start gap-1 px-3 py-2"
      >
        <TabsTrigger
          value="profile"
          className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
        >
          {t("contextGroupProfileTab")}
        </TabsTrigger>
        <TabsTrigger
          value="members"
          className="-mb-0 rounded-md border-b-0 px-2.5 py-1.5 text-xs data-[state=active]:bg-primary data-[state=active]:text-primary-foreground"
        >
          {t("contextGroupMembersTab")}
        </TabsTrigger>
      </TabsList>

      <TabsContent
        value="profile"
        className="mt-0 min-h-0 flex-1 overflow-y-auto overscroll-contain p-3"
      >
        {group ? (
          <GroupConversationProfile
            group={group}
            createdAt={group.createdAt}
            canManage={canManage}
            onRename={renameGroup}
          />
        ) : (
          <GroupResourceState
            loading={resource.loading}
            failed={failed}
            onRetry={() => void resource.refresh()}
          />
        )}
      </TabsContent>

      <TabsContent
        value="members"
        className="mt-0 min-h-0 flex-1 overflow-hidden"
      >
        {group ? (
          <GroupParticipantList
            participants={group.participants}
            currentIdentityID={currentIdentityID}
            canManage={canManage}
            readOnly={dissolved}
            onAdd={addMembers}
            onTransferOwner={transferOwner}
            onRemove={removeMember}
            onLeave={leaveGroup}
          />
        ) : (
          <GroupResourceState
            loading={resource.loading}
            failed={failed}
            onRetry={() => void resource.refresh()}
          />
        )}
      </TabsContent>
    </Tabs>
  )
}
