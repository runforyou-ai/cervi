/** 渠道模块的分栏外壳：中间栏按渠道类别导航，规划中的渠道展示为不可点击项。 */
import { useTranslation } from "react-i18next"
import { Outlet } from "react-router"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import {
  messageChannelTypeDefinitions,
  plannedChannelDefinitions,
} from "@/lib/message-channel-types"

/** 按类别拆分渠道列表，不同类别的列表可展示各自的列。 */
export function ChannelsLayout() {
  const { t } = useTranslation("channels")

  return (
    <PageSplit
      paneWidth="nav"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigation.label")} title={t("list.title")}>
          {messageChannelTypeDefinitions.map((definition) => (
            <PagePaneLink
              key={definition.type}
              to={`/channels/${definition.type}`}
              icon={definition.icon}
            >
              {t(`types.${definition.translationKey}`)}
            </PagePaneLink>
          ))}
          {/* 尚未接入的渠道紧跟在已接入类别后面，直接置灰、不可点击。 */}
          {plannedChannelDefinitions.map((definition) => (
            <PagePaneLink
              key={definition.key}
              icon={definition.icon}
              comingSoonHint={false}
            >
              {t(`plannedTypes.${definition.key}`)}
            </PagePaneLink>
          ))}
        </PagePaneNav>
      }
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <Outlet />
      </div>
    </PageSplit>
  )
}
