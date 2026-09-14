/** 消息页中栏的搜索结果：范围与类型切换、分组结果、最近打开和快捷键提示。 */
import { useEffect, useRef, type MouseEvent, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

import {
  InboxScope,
  InboxSearchPersonKind,
  InboxSearchRange,
  OrganizationIdentityType,
  isAgentInboxConversation,
  type InboxConversation,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { ProfileAvatar } from "@/components/profile-avatar"
import { Button } from "@/components/ui/button"
import { ConversationAvatar } from "@/features/inbox/conversation-avatar"
import { useConversationName } from "@/features/inbox/use-conversation-name"
import { useConversationTime } from "@/features/inbox/use-conversation-time"
import type { InboxSearchState, InboxSearchType } from "@/features/inbox/use-inbox-search"
import { cn } from "@/lib/utils"

const searchGroupLimit = 6
const markClassName = "bg-transparent font-semibold text-primary"
const keyClassName = "mr-0.5 rounded border bg-muted px-1 font-sans text-[10px]"

/** 点击结果或切换项时保持搜索框焦点。 */
function keepSearchFocus(event: MouseEvent) {
  event.preventDefault()
}

/** 不区分大小写地标出名称中的首个匹配片段。 */
function highlightName(name: string, query: string): ReactNode {
  const index = query ? name.toLowerCase().indexOf(query.toLowerCase()) : -1
  if (index < 0) return name
  return (
    <>
      {name.slice(0, index)}
      <mark className={markClassName}>{name.slice(index, index + query.length)}</mark>
      {name.slice(index + query.length)}
    </>
  )
}

/** 搜索结果分组；传入查看全部文案且结果达到上限时显示查看全部入口。 */
function SearchGroup({ title, count, viewAllLabel, children }: { title: string; count: number; viewAllLabel?: string; children: ReactNode }) {
  if (count === 0) return null
  return (
    <section>
      <h3 className="px-3 pt-2.5 pb-1 text-[11px] font-semibold text-muted-foreground">{title}</h3>
      {children}
      {viewAllLabel && count >= searchGroupLimit ? (
        <Button type="button" variant="ghost" size="xs" disabled className="ml-1 text-primary">
          {viewAllLabel}
        </Button>
      ) : null}
    </section>
  )
}

/** 搜索结果行，选中态跟随键盘和鼠标。 */
function SearchResultRow({
  index,
  selected,
  disabled = false,
  avatar,
  title,
  detail,
  time,
  onSelect,
  onOpen,
}: {
  index: number
  selected: boolean
  disabled?: boolean
  avatar: ReactNode
  title: ReactNode
  detail?: ReactNode
  time?: string
  onSelect: (index: number) => void
  onOpen: () => void
}) {
  return (
    <button
      type="button"
      role="option"
      aria-selected={selected}
      aria-disabled={disabled}
      data-search-index={index}
      className={cn(
        "flex w-full min-w-0 items-center gap-2.5 rounded-md px-3 py-1.5 text-left",
        selected ? "bg-accent text-accent-foreground" : "hover:bg-muted",
        disabled && "cursor-not-allowed opacity-50",
      )}
      onMouseDown={keepSearchFocus}
      onMouseMove={() => onSelect(index)}
      onClick={() => {
        if (!disabled) onOpen()
      }}
    >
      <span className="shrink-0">{avatar}</span>
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[13px]">{title}</span>
        {detail ? (
          <span className={cn("block truncate text-[11px] text-muted-foreground", selected && "text-accent-foreground/75")}>{detail}</span>
        ) : null}
      </span>
      {time ? <span className="shrink-0 text-[11px] text-muted-foreground">{time}</span> : null}
    </button>
  )
}

/** 按搜索模式状态渲染范围、类型、分组结果和快捷键提示。 */
export function InboxSearchPanel({ search, scope }: { search: InboxSearchState; scope: InboxScope }) {
  const { t } = useTranslation(["inbox", "common"])
  const conversationName = useConversationName()
  const formatTime = useConversationTime()
  const listRef = useRef<HTMLDivElement>(null)
  const query = search.text.trim()

  useEffect(() => {
    // 键盘移动选择时保持选中项可见。
    listRef.current?.querySelector(`[data-search-index="${search.selectedIndex}"]`)?.scrollIntoView({ block: "nearest" })
  }, [search.selectedIndex])

  const rangeOptions = [
    ...(search.conversationId ? [{ value: InboxSearchRange.InboxSearchRangeConversation, label: t("searchRangeConversation") }] : []),
    ...(search.listRange
      ? [{ value: InboxSearchRange.InboxSearchRangeList, label: t(scope === InboxScope.InboxScopeCustomer ? "scopeCustomer" : "scopeInternal") }]
      : []),
    { value: InboxSearchRange.InboxSearchRangeReadable, label: t("searchRangeReadable") },
  ]
  const typeOptions: { value: InboxSearchType; label: string }[] = [
    { value: "all", label: t("searchTypeAll") },
    { value: "conversations", label: t("searchTypeConversations") },
    { value: "messages", label: t("searchTypeMessages") },
    { value: "people", label: t("searchTypePeople") },
  ]
  const conversationRows = (conversations: InboxConversation[]) =>
    conversations.map((conversation, index) => (
      <SearchResultRow
        key={conversation.id}
        index={index}
        selected={search.selectedIndex === index}
        avatar={<ConversationAvatar conversation={conversation} className="size-7" />}
        title={highlightName(
          isAgentInboxConversation(conversation) ? `${conversation.agent.agentName} · ${conversation.agent.title}` : conversationName(conversation),
          search.showRecent ? "" : query,
        )}
        onSelect={search.setActiveIndex}
        onOpen={() => search.open({ kind: "conversation", conversation })}
      />
    ))
  const messageOffset = search.conversations.length
  const peopleOffset = messageOffset + search.messages.length
  const empty = search.conversations.length + search.messages.length + search.people.length === 0

  let body: ReactNode = null
  if (search.showRecent) {
    body = (
      <SearchGroup title={t("searchRecent")} count={search.recentConversations.length}>
        {conversationRows(search.recentConversations)}
      </SearchGroup>
    )
  } else if (query && search.pending) {
    body = <LoadingIndicator className="justify-center py-10">{t("searchLoading")}</LoadingIndicator>
  } else if (query && search.error) {
    body = (
      <div className="flex flex-col items-center gap-3 px-6 py-10 text-[13px] text-muted-foreground">
        <p>{t("searchError")}</p>
        <Button type="button" variant="outline" size="sm" onClick={() => void search.retry()}>
          {t("common:actions.retry")}
        </Button>
      </div>
    )
  } else if (query && empty) {
    body = <p className="px-6 py-10 text-center text-[13px] text-muted-foreground">{t("searchNoResults", { query })}</p>
  } else if (query) {
    body = (
      <>
        <SearchGroup title={t("searchGroupConversations")} count={search.conversations.length} viewAllLabel={t("searchViewAll")}>
          {conversationRows(search.conversations)}
        </SearchGroup>
        <SearchGroup title={t("searchGroupMessages")} count={search.messages.length} viewAllLabel={t("searchViewAll")}>
          {search.messages.map((message, position) => (
            <SearchResultRow
              key={message.id}
              index={messageOffset + position}
              selected={search.selectedIndex === messageOffset + position}
              avatar={<ConversationAvatar conversation={message.conversation} className="size-7" />}
              title={conversationName(message.conversation)}
              detail={
                <>
                  {message.senderName ? t("searchMessageSender", { name: message.senderName }) : null}
                  {message.excerpt.map((segment, segmentIndex) =>
                    segment.match ? (
                      <mark key={segmentIndex} className={markClassName}>{segment.text}</mark>
                    ) : (
                      <span key={segmentIndex}>{segment.text}</span>
                    ),
                  )}
                </>
              }
              time={formatTime(message.originatedAt)}
              onSelect={search.setActiveIndex}
              onOpen={() => search.open({ kind: "message", message })}
            />
          ))}
        </SearchGroup>
        <SearchGroup title={t("searchGroupPeople")} count={search.people.length} viewAllLabel={t("searchViewAll")}>
          {search.people.map((person, position) => {
            const contact = person.kind === InboxSearchPersonKind.InboxSearchPersonContact
            const agent = person.identityType === OrganizationIdentityType.OrganizationIdentityTypeAgent
            return (
              <SearchResultRow
                key={`${person.kind}-${person.id}`}
                index={peopleOffset + position}
                selected={search.selectedIndex === peopleOffset + position}
                disabled={contact && !person.conversationId}
                avatar={<ProfileAvatar imageURL={person.avatarUrl} name={person.displayName} fallback={agent ? "agent" : "person"} className="size-7" />}
                title={highlightName(person.displayName, query)}
                detail={
                  contact
                    ? t(person.conversationId ? "searchPersonContact" : "searchPersonNoConversation")
                    : t(agent ? "contextIdentityAgent" : "contextIdentityMember")
                }
                onSelect={search.setActiveIndex}
                onOpen={() => search.open({ kind: "person", person })}
              />
            )
          })}
        </SearchGroup>
      </>
    )
  }

  return (
    <div data-slot="inbox-search" className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 flex-col gap-1.5 px-3 py-2">
        <div className="flex flex-wrap items-center gap-1.5">
          {rangeOptions.map((option) => (
            <Button
              key={option.value}
              type="button"
              size="xs"
              variant={search.range === option.value ? "secondary" : "ghost"}
              className="rounded-full"
              aria-pressed={search.range === option.value}
              onMouseDown={keepSearchFocus}
              onClick={() => search.setRange(option.value)}
            >
              {option.label}
            </Button>
          ))}
        </div>
        {search.range === InboxSearchRange.InboxSearchRangeConversation ? null : (
          <div className="flex flex-wrap items-center gap-1.5">
            {typeOptions.map((option) => (
              <Button
                key={option.value}
                type="button"
                size="xs"
                variant={search.type === option.value ? "secondary" : "ghost"}
                className="rounded-full"
                aria-pressed={search.type === option.value}
                onMouseDown={keepSearchFocus}
                onClick={() => search.setType(option.value)}
              >
                {option.label}
              </Button>
            ))}
          </div>
        )}
      </div>
      <div ref={listRef} role="listbox" aria-label={t("searchLabel")} className="min-h-0 flex-1 overflow-y-auto px-2 pb-2">
        {body}
      </div>
      <div className="flex shrink-0 gap-3 border-t px-3.5 py-2 text-[11px] text-muted-foreground">
        <span>
          <kbd className={keyClassName}>↑</kbd>
          <kbd className={keyClassName}>↓</kbd> {t("searchHintSelect")}
        </span>
        <span>
          <kbd className={keyClassName}>Enter</kbd> {t("searchHintOpen")}
        </span>
        <span>
          <kbd className={keyClassName}>Esc</kbd> {t("searchHintExit")}
        </span>
      </div>
    </div>
  )
}
