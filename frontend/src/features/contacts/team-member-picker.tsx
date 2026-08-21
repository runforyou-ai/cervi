/** 团队成员候选搜索、多选和批量添加。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  MemberIdentityType,
  addTeamMembers,
  isApiError,
  listTeamMemberCandidates,
  type Team,
  type TeamMemberCandidate,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { roleDisplayName } from "@/features/roles/role-labels"
import { cn } from "@/lib/utils"

type LoadState = "loading" | "ready" | "error"

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

/** 搜索并批量选择尚未加入团队的企业成员。 */
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
  const { t: tCommon } = useTranslation("common")
  const [search, setSearch] = useState("")
  const [query, setQuery] = useState("")
  const [members, setMembers] = useState<TeamMemberCandidate[]>([])
  const [selected, setSelected] = useState<Map<string, TeamMemberCandidate>>(
    new Map(),
  )
  const [loadState, setLoadState] = useState<LoadState>("loading")
  const [saving, setSaving] = useState(false)
  const requestID = useRef(0)

  useEffect(() => {
    const timeout = window.setTimeout(() => setQuery(search.trim()), 300)
    return () => window.clearTimeout(timeout)
  }, [search])

  /** 按当前关键字加载候选成员。 */
  const loadMembers = useCallback(
    async function loadMembers() {
      const currentRequestID = ++requestID.current
      setLoadState("loading")
      try {
        const output = await listTeamMemberCandidates(team.id, {
          query,
          page: 1,
          pageSize: 100,
        })
        if (currentRequestID !== requestID.current) return
        setMembers(output.members)
        setLoadState("ready")
      } catch (error) {
        if (currentRequestID !== requestID.current) return
        console.warn("读取团队成员候选失败", error)
        setMembers([])
        setLoadState("error")
      }
    },
    [query, team.id],
  )

  useEffect(() => {
    void loadMembers()
    return () => {
      requestID.current += 1
    }
  }, [loadMembers])

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
        <Label htmlFor="team-member-search">
          {t("teams.members.search")}
        </Label>
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
        {loadState === "loading" ? (
          <div className="flex min-h-56 items-center justify-center gap-2 text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("loading")}
          </div>
        ) : loadState === "error" ? (
          <div className="flex min-h-56 flex-col items-center justify-center gap-3 text-sm text-muted-foreground">
            <span>{t("teams.members.loadError")}</span>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void loadMembers()}
            >
              {t("retry")}
            </Button>
          </div>
        ) : members.length === 0 ? (
          <div className="flex min-h-56 items-center justify-center text-sm text-muted-foreground">
            {t(query ? "teams.members.noMatches" : "teams.members.noCandidates")}
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
                      <span>{roleDisplayName(member.role, tCommon)}</span>
                      <span aria-hidden="true">·</span>
                      <span>
                        {t(
                          member.identityType ===
                            MemberIdentityType.MemberIdentityTypeAgent
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
