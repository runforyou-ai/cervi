/** 展示和选择 AI 员工绑定的企业本地知识库。 */
import { useTranslation } from "react-i18next"

import { listKnowledgeBases } from "@/api"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 勾选 AI 员工可检索的知识库范围。 */
export function AgentKnowledgeField({
  value,
  onChange,
  disabled = false,
}: {
  value: string[]
  onChange: (ids: string[]) => void
  disabled?: boolean
}) {
  const { t } = useTranslation(["contacts", "common"])
  const resource = useResource(
    resourceKeys.knowledgeBases(),
    () => listKnowledgeBases(),
    { staleTime: 0 },
  )
  if (resource.loading) {
    return (
      <p className="text-sm text-muted-foreground">
        {t("agents.execution.knowledgeLoading")}
      </p>
    )
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
          {t("common:actions.retry")}
        </Button>
      </div>
    )
  }
  const bases = resource.data?.knowledgeBases ?? []
  // 将失效绑定合并为一项，便于一次移除。
  const unavailable = value.filter(
    (id) => !bases.some((base) => base.id === id),
  )
  const options = [
    ...bases.map((base) => ({
      ids: [base.id],
      name: base.name,
      unavailable: false,
    })),
    ...(unavailable.length > 0
      ? [
          {
            ids: unavailable,
            name: t("agents.execution.knowledgeUnavailableCount", {
              count: unavailable.length,
            }),
            unavailable: true,
          },
        ]
      : []),
  ]
  return (
    <div
      className="grid gap-2"
      role="group"
      aria-label={t("agents.execution.knowledgeBases")}
    >
      {options.map((option) => (
        <label key={option.ids[0]} className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4 accent-primary disabled:cursor-wait disabled:opacity-60"
            disabled={disabled}
            checked={option.ids.every((id) => value.includes(id))}
            onChange={(event) => {
              onChange(
                event.target.checked
                  ? [...value, ...option.ids]
                  : value.filter((id) => !option.ids.includes(id)),
              )
            }}
          />
          <span className={option.unavailable ? "text-destructive" : undefined}>
            {option.name}
          </span>
        </label>
      ))}
      {options.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {t("agents.execution.knowledgeEmpty")}
        </p>
      ) : null}
    </div>
  )
}
