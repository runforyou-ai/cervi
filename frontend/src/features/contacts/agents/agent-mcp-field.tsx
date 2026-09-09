/** 通过模态框编辑 AI 员工表单中的 MCP 服务选择。 */
import { useRef, useState, type RefObject } from "react"
import { useTranslation } from "react-i18next"

import { listAgentMCPServerOptions } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

/** 显示表单选择摘要，确认模态框后才修改表单。 */
export function AgentMCPField({
  value,
  onChange,
  disabled,
}: {
  value: string[]
  onChange: (ids: string[]) => void
  disabled: boolean
}) {
  const { t } = useTranslation("contacts")
  const [open, setOpen] = useState(false)
  const trigger = useRef<HTMLButtonElement>(null)
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <div className="flex items-center gap-3">
        <span className="text-sm text-muted-foreground">
          {value.length === 0 ? t("agents.mcp.unconfigured") : t("agents.mcp.selected", { count: value.length })}
        </span>
        <Button ref={trigger} type="button" variant="outline" size="sm" disabled={disabled} onClick={() => setOpen(true)}>
          {t("agents.configure")}
        </Button>
      </div>
      {open && (
        <AgentMCPDialog
          value={value}
          trigger={trigger}
          onCancel={() => setOpen(false)}
          onConfirm={(ids) => {
            onChange(ids)
            setOpen(false)
          }}
        />
      )}
    </Dialog>
  )
}

/** 展示企业服务卡片并维护本次模态框的独立草稿。 */
function AgentMCPDialog({
  value,
  trigger,
  onCancel,
  onConfirm,
}: {
  value: string[]
  trigger: RefObject<HTMLButtonElement | null>
  onCancel: () => void
  onConfirm: (ids: string[]) => void
}) {
  const { t } = useTranslation(["contacts", "common"])
  const [selected, setSelected] = useState(value)
  const resource = useResource(resourceKeys.agentMCPServerOptions(), () => listAgentMCPServerOptions(), { staleTime: 0 })
  const services = resource.data ?? []
  return (
    <DialogContent
      className="max-w-4xl grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden"
      aria-describedby={undefined}
      onCloseAutoFocus={(event) => {
        event.preventDefault()
        trigger.current?.focus()
      }}
    >
      <DialogHeader>
        <DialogTitle>{t("agents.mcp.title")}</DialogTitle>
      </DialogHeader>
      <div className="min-h-48 overflow-y-auto p-1">
        {resource.loading ? (
          <LoadingIndicator className="min-h-48 justify-center">{t("common:status.loading")}</LoadingIndicator>
        ) : resource.error ? (
          <div className="flex min-h-48 flex-col items-center justify-center gap-3">
            <p className="text-sm text-muted-foreground">{t("agents.mcp.loadError")}</p>
            <Button type="button" size="sm" variant="outline" onClick={() => void resource.refresh()}>{t("common:actions.retry")}</Button>
          </div>
        ) : services.length === 0 ? (
          <p className="flex min-h-48 items-center justify-center text-sm text-muted-foreground">{t("agents.mcp.empty")}</p>
        ) : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3" role="group" aria-label={t("agents.mcp.services")}>
            {services.map((service) => (
              <label key={service.id} className={cn(
                "flex min-w-0 cursor-pointer items-start gap-3 rounded-lg border bg-card p-4 focus-within:border-primary",
                selected.includes(service.id) ? "border-primary bg-primary/5" : "hover:bg-accent/50",
              )}>
                <input
                  type="checkbox"
                  className="mt-0.5 size-4 shrink-0 accent-primary"
                  aria-label={service.name}
                  checked={selected.includes(service.id)}
                  onChange={(event) => setSelected(event.target.checked ? [...selected, service.id] : selected.filter((id) => id !== service.id))}
                />
                <span className="min-w-0 space-y-2">
                  <span className="line-clamp-2 break-all text-sm font-medium" title={service.name}>{service.name}</span>
                  <span className="block text-xs text-muted-foreground">{t("agents.mcp.tools", { count: service.toolCount })}</span>
                </span>
              </label>
            ))}
          </div>
        )}
      </div>
      <div className="flex justify-end gap-2">
        <Button type="button" variant="outline" onClick={onCancel}>{t("common:actions.cancel")}</Button>
        <Button type="button" disabled={resource.loading || Boolean(resource.error) || resource.refreshing} onClick={() => {
          // 确认当前目录中的选择，已删除的服务不会重新带回表单。
          onConfirm(services.filter((service) => selected.includes(service.id)).map((service) => service.id).sort())
        }}>{t("common:actions.confirm")}</Button>
      </div>
    </DialogContent>
  )
}
