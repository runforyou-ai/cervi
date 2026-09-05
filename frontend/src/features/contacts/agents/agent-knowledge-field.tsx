/** 展示和选择 AI 员工可使用的企业知识库。 */
import { useTranslation } from "react-i18next"

import { listKnowledgeBases } from "@/api"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 渲染知识库范围，未提供修改回调时展示已选知识库。 */
export function AgentKnowledgeField({
  value,
  onChange,
  disabled = false,
}: {
  value: string[]
  onChange?: (ids: string[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation("contacts")
  const resource = useResource(
    resourceKeys.knowledgeBases(),
    () => listKnowledgeBases(),
    { staleTime: 0 },
  )
  if (resource.loading) {
    return <p className="text-sm text-muted-foreground">{t("agents.execution.knowledgeLoading")}</p>
  }
  if (resource.error) {
    return (
      <div className="flex items-center gap-2 text-sm">
        <span>{t("agents.execution.knowledgeLoadError")}</span>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => void resource.refresh()}
        >
          {t("agents.execution.knowledgeRetry")}
        </Button>
      </div>
    )
  }
  const bases = resource.data?.knowledgeBases ?? []
  // 将失效绑定合并为一项，支持一次移除后立即保存。
  const unavailable = value.filter((id) => !bases.some((base) => base.id === id))
  if (!onChange) {
    return (
      <span>
        {value.length === 0
          ? t("agents.execution.knowledgeDisabled")
          : value.map((id) =>
              bases.find((base) => base.id === id)?.name ?? t("agents.execution.knowledgeUnavailable"),
            ).join("、")}
      </span>
    )
  }
  const options = [
    ...bases.map((base) => ({ ids: [base.id], name: base.name, unavailable: false })),
    ...(unavailable.length > 0 ? [{
      ids: unavailable,
      name: t("agents.execution.knowledgeUnavailableCount", { count: unavailable.length }),
      unavailable: true,
    }] : []),
  ]
  return (
    <div className="grid gap-2" role="group" aria-label={t("agents.execution.knowledgeBases")}>
      {options.map((option) => (
        <label key={option.ids[0]} className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4 accent-primary aria-disabled:cursor-wait aria-disabled:opacity-60"
            aria-disabled={disabled}
            checked={option.ids.every((id) => value.includes(id))}
            onChange={(event) => {
              if (disabled) return
              onChange(event.target.checked ? [...value, ...option.ids] : value.filter((id) => !option.ids.includes(id)))
            }}
          />
          <span className={option.unavailable ? "text-destructive" : undefined}>{option.name}</span>
        </label>
      ))}
      <p className="text-sm text-muted-foreground">
        {options.length === 0 ? t("agents.execution.knowledgeEmpty") : t("agents.execution.knowledgeHint")}
      </p>
    </div>
  )
}
