/** 列表中已加入团队的摘要单元格。 */
import { useTranslation } from "react-i18next"

import type { TeamSummary } from "@/api"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

/** 显示团队摘要，并在悬停或聚焦时逐行列出全部团队。 */
export function JoinedTeamsCell({ teams }: { teams: TeamSummary[] }) {
  const { t } = useTranslation("contacts")

  if (teams.length === 0) return "—"

  const summary =
    teams.length === 1
      ? teams[0].name
      : teams.length === 2
        ? t("teams.joinedPair", {
            first: teams[0].name,
            second: teams[1].name,
          })
        : t("teams.joinedSummary", {
            first: teams[0].name,
            second: teams[1].name,
            count: teams.length,
          })

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span
          tabIndex={0}
          className="block max-w-xs cursor-help truncate outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          {summary}
        </span>
      </TooltipTrigger>
      <TooltipContent side="bottom" sideOffset={4} className="max-w-xs">
        <ul className="grid gap-1 text-left">
          {teams.map((team) => (
            <li key={team.id} className="break-words">
              {team.name}
            </li>
          ))}
        </ul>
      </TooltipContent>
    </Tooltip>
  )
}
