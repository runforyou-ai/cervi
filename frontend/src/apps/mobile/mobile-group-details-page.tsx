/** 移动端群详情占位页面。 */
import { useTranslation } from "react-i18next"
import { useParams } from "react-router"

import { MobilePageHeader, MobilePageState } from "@/apps/mobile/mobile-page"

/** 显示群详情占位状态并提供返回当前群聊的入口。 */
export function MobileGroupDetailsPage() {
  const { t } = useTranslation("mobile")
  const { conversationID = "" } = useParams()

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobilePageHeader
        title={t("group.details")}
        backTo={`/inbox/group/${conversationID}`}
      />
      <MobilePageState title={t("unavailable")} />
    </section>
  )
}
