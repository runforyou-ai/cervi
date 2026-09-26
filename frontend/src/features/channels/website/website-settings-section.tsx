/** 网站渠道聊天窗口页签中的设置分区。 */
import { useId, type ReactNode } from "react"

/** 带小标题的设置分区，同一页签内的分区按固定间距纵向排列。 */
export function WebsiteSettingsSection({
  title,
  children,
}: {
  title: string
  children: ReactNode
}) {
  const id = useId()
  return (
    <section aria-labelledby={id}>
      <h3 id={id} className="mb-4 text-base font-medium">
        {title}
      </h3>
      {children}
    </section>
  )
}
