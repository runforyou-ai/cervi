/** 渠道页布局，左栏列出渠道类型。 */
import { useTranslation } from "react-i18next"
import { Outlet, useLocation, useNavigate } from "react-router"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import { NativeSelect } from "@/components/ui/native-select"

const channelTypes: {
  id: "website" | "telegram" | "wechatOfficialAccount"
  labelKey: "types.website" | "types.telegram" | "types.wechatOfficialAccount"
  to?: string
}[] = [
  { id: "website", labelKey: "types.website", to: "/channels/website" },
  { id: "telegram", labelKey: "types.telegram" },
  { id: "wechatOfficialAccount", labelKey: "types.wechatOfficialAccount" },
]

/** 渠道类型分栏和当前渠道页面。 */
export function ChannelsLayout() {
  const { t } = useTranslation("channels")
  const location = useLocation()
  const navigate = useNavigate()
  const currentType =
    channelTypes.find(
      (type) => type.to && location.pathname.startsWith(type.to),
    )?.id ?? "website"

  return (
    <PageSplit
      paneWidth="sm"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("typeNavigation")} title={t("title")}>
          {channelTypes.map((type) => (
            <PagePaneLink key={type.id} to={type.to}>
              {t(type.labelKey)}
            </PagePaneLink>
          ))}
        </PagePaneNav>
      }
    >
      <div className="border-b px-4 py-3 md:hidden">
        <NativeSelect
          className="h-8 w-full"
          aria-label={t("typeNavigation")}
          value={currentType}
          onChange={(event) => {
            const selected = channelTypes.find(
              (type) => type.id === event.target.value,
            )
            if (!selected?.to) {
              console.warn("选中了未开放的渠道类型", event.target.value)
              return
            }
            navigate(selected.to)
          }}
        >
          {channelTypes.map((type) => (
            <option key={type.id} value={type.id} disabled={!type.to}>
              {type.to
                ? t(type.labelKey)
                : t("typeComingSoon", { name: t(type.labelKey) })}
            </option>
          ))}
        </NativeSelect>
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        <Outlet />
      </div>
    </PageSplit>
  )
}
