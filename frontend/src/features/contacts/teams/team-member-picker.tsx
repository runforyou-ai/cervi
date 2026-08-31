/** 团队成员候选搜索、多选和批量添加。 */
import { useEffect, useState } from "react"
import { SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  OrganizationIdentityType,
  addTeamMembers,
  isApiError,
  listTeamMemberCandidates,
  type Team,
  type TeamMemberCandidate,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const memberPageSize = 50

/** 展示成员头像，图片不可用时回退到姓名首字。 */
function MemberAvatar({ member }: { member: TeamMemberCandidate }) {
  const [failed, setFailed] = useState(false)

  useEffect(() => setFailed(false), [member.avatarUrl])

  return (
    <span className="flex size-9 shrink-0 items-center justify-center overflow-hidden rounded-full bg-sidebar-primary text-sm font-semibold text-sidebar-primary-foreground">
      {member.avatarUrl && !failed ? (
        <img
          className="size-full object-cover"
          src={member.avatarUrl}
          alt={member.displayName}
          onError={() => setFailed(true)}
        />
      ) : (
        member.displayName.slice(0, 1).toUpperCase()
      )}
    </span>
  )
}

/** 搜索并批量选择尚未加入团队的企业身份。 */
export function TeamMemberPicker({
  team,
  onSaved,
  onCancel,
}: {
  team: Team
  onSaved: (team: Team) => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  const invalidate = useResourceInvalidator()
  const [search, setSearch] = useState("")
  const [query, setQuery] = useState("")
  const [currentPage, setCurrentPage] = useState(1)
  const [selected, setSelected] = useState<Map<string, TeamMemberCandidate>>(
    new Map(),
  )
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      setQuery(search.trim())
      setCurrentPage(1)
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [search])

  const candidates = useResource(
    resourceKeys.teamMemberCandidates(team.id, {
      query,
      page: currentPage,
      pageSize: memberPageSize,
    }),
    () =>
      listTeamMemberCandidates(team.id, {
        query,
        page: currentPage,
        pageSize: memberPageSize,
      }),
  )
  const members = candidates.data?.members ?? []
  const pageInfo = candidates.data?.page ?? {
    number: currentPage,
    size: memberPageSize,
    total: 0,
  }
  const loading = candidates.loading
  const failed = Boolean(candidates.error)

  const totalPages = Math.max(1, Math.ceil(pageInfo.total / pageInfo.size))

  /** 切换候选成员的选中状态。 */
  function toggleMember(member: TeamMemberCandidate, checked: boolean) {
    setSelected((current) => {
      const next = new Map(current)
      const key = `${member.identityType}:${member.identityId}`
      if (checked) {
        next.set(key, member)
      } else {
        next.delete(key)
      }
      return next
    })
  }

  /** 将选中成员加入团队。 */
  async function saveMembers() {
    if (selected.size === 0) return
    setSaving(true)
    try {
      const saved = await addTeamMembers(team.id, {
        members: [...selected.values()].map((member) => ({
          identityType: member.identityType,
          identityId: member.identityId,
        })),
      })
      void invalidate(resourceKeys.teamMemberCandidates(team.id))
      toast.success(t("teams.members.added", { count: selected.size }))
      onSaved(saved)
    } catch (error) {
      console.warn("添加团队成员失败", error)
      toast.error(
        isApiError(error) ? error.message : t("teams.members.addError"),
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="grid min-h-0 gap-4">
      <div className="grid gap-2">
        <Label htmlFor="team-member-search">{t("teams.members.search")}</Label>
        <div className="relative">
          <SearchIcon className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="team-member-search"
            className="pl-9"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </div>
      </div>

      <div className="max-h-[min(24rem,50svh)] min-h-56 overflow-y-auto rounded-md border">
        {loading ? (
          <LoadingIndicator className="min-h-56 justify-center">
            {t("loading")}
          </LoadingIndicator>
        ) : failed ? (
          <div className="flex min-h-56 flex-col items-center justify-center gap-3 text-sm text-muted-foreground">
            <span>{t("teams.members.loadError")}</span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void candidates.refresh()}
            >
              {t("retry")}
            </Button>
          </div>
        ) : members.length === 0 ? (
          <div className="flex min-h-56 items-center justify-center text-sm text-muted-foreground">
            {t(
              query ? "teams.members.noMatches" : "teams.members.noCandidates",
            )}
          </div>
        ) : (
          <div className="divide-y">
            {members.map((member) => {
              const key = `${member.identityType}:${member.identityId}`
              return (
                <label
                  key={key}
                  className={cn(
                    "flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/50",
                    selected.has(key) && "bg-muted/50",
                  )}
                >
                  <input
                    type="checkbox"
                    className="size-4 shrink-0 accent-primary"
                    checked={selected.has(key)}
                    onChange={(event) =>
                      toggleMember(member, event.target.checked)
                    }
                  />
                  <MemberAvatar member={member} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">
                      {member.displayName}
                    </span>
                    <span className="mt-0.5 flex flex-wrap gap-x-2 text-xs text-muted-foreground">
                      <span>
                        {t(
                          member.identityType ===
                            OrganizationIdentityType.OrganizationIdentityTypeAgent
                            ? "identityCategories.agent"
                            : "identityCategories.user",
                        )}
                      </span>
                    </span>
                  </span>
                </label>
              )
            })}
          </div>
        )}
      </div>

      {totalPages > 1 ? (
        <div className="flex items-center justify-between gap-3 text-sm text-muted-foreground">
          <span>{t("pagination.total", { count: pageInfo.total })}</span>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={currentPage <= 1 || loading}
              onClick={() => setCurrentPage((page) => page - 1)}
            >
              {t("pagination.previous")}
            </Button>
            <span>
              {t("pagination.page", {
                current: pageInfo.number,
                total: totalPages,
              })}
            </span>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={currentPage >= totalPages || loading}
              onClick={() => setCurrentPage((page) => page + 1)}
            >
              {t("pagination.next")}
            </Button>
          </div>
        </div>
      ) : null}

      <div className="flex items-center justify-between gap-3">
        <span className="text-sm text-muted-foreground">
          {t("teams.members.selected", { count: selected.size })}
        </span>
        <div className="flex items-center gap-2">
          <Button type="button" variant="outline" onClick={onCancel}>
            {t("teams.form.cancel")}
          </Button>
          <Button
            type="button"
            disabled={selected.size === 0 || saving}
            onClick={() => void saveMembers()}
          >
            {saving ? t("teams.members.adding") : t("teams.members.add")}
          </Button>
        </div>
      </div>
    </div>
  )
}
