/** 只读展示角色对 AI 员工的内置工作规则与可用工具。 */
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"

/** 返回工具代码对应的显示名称。 */
function agentToolLabel(tool: string, t: ReturnType<typeof useTranslation<"common">>["t"]) {
  switch (tool) {
    case "search_knowledge":
      return t("agentTools.searchKnowledge")
    case "search_customer_history":
      return t("agentTools.searchCustomerHistory")
    case "ask_customer":
      return t("agentTools.askCustomer")
    case "handoff_to_human":
      return t("agentTools.handoffToHuman")
    case "mcp":
      return t("agentTools.mcp")
    default:
      return tool
  }
}

/** 折叠展示内置规则正文，工具清单常驻显示。 */
export function AgentBehaviorSummary({
  behavior,
}: {
  behavior: { instruction: string; tools: string[] }
}) {
  const { t } = useTranslation("common")
  const [open, setOpen] = useState(false)
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-center gap-2 text-sm">
        <span className="text-muted-foreground">{t("agentBehavior.tools")}</span>
        {behavior.tools.map((tool) => (
          <span key={tool} className="rounded-md border px-2 py-0.5">
            {agentToolLabel(tool, t)}
          </span>
        ))}
      </div>
      <Collapsible open={open} onOpenChange={setOpen}>
        <CollapsibleTrigger asChild>
          <Button type="button" variant="outline" size="sm">
            {open ? t("agentBehavior.hide") : t("agentBehavior.show")}
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <pre className="mt-2 whitespace-pre-wrap rounded-md border bg-muted/40 p-3 font-sans text-sm leading-6">
            {behavior.instruction}
          </pre>
        </CollapsibleContent>
      </Collapsible>
    </div>
  )
}
