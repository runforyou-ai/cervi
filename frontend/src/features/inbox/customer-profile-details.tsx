/** 客户会话侧栏的客户身份与当前周期访客上下文。 */
import { useTranslation } from "react-i18next"

import { getCustomerProfile } from "@/api"
import { SidePanelField } from "@/features/inbox/side-panel-layout"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { openExternalURL } from "@/platform/external-navigation"

/** 按会话最新消息刷新客户身份与访客上下文，只展示有值的字段；身份验证状态只在网站渠道展示。 */
export function CustomerProfileDetails({
  conversationID,
  lastMessageID,
  website,
}: {
  conversationID: string
  lastMessageID: string | null
  website: boolean
}) {
  const { t } = useTranslation("inbox")
  const profile = useResource(
    resourceKeys.customerProfile(conversationID, { lastMessageID }),
    () => getCustomerProfile(conversationID),
  )
  const data = profile.data
  if (profile.error) {
    return (
      <p className="text-xs leading-5 text-muted-foreground">
        {t("contextProfileLoadError")}
      </p>
    )
  }
  if (!data) return null
  const visit = data.visit
  // 设备类型、浏览器与操作系统合为一行。
  const deviceTypes: Record<string, string> = {
    desktop: t("contextDevice_desktop"),
    mobile: t("contextDevice_mobile"),
    tablet: t("contextDevice_tablet"),
  }
  const device = visit
    ? [deviceTypes[visit.deviceType] ?? "", visit.browser, visit.os]
        .filter(Boolean)
        .join(" · ")
    : ""

  return (
    <>
      {website ? (
        <SidePanelField label={t("contextVerification")}>
          {data.identityVerified ? t("contextVerified") : t("contextUnverified")}
        </SidePanelField>
      ) : null}
      {data.externalUserId ? (
        <SidePanelField label={t("contextExternalUserId")}>
          <span className="min-w-0 truncate" title={data.externalUserId}>
            {data.externalUserId}
          </span>
        </SidePanelField>
      ) : null}
      {data.email ? (
        <SidePanelField label={t("contextEmail")}>
          <span className="min-w-0 truncate" title={data.email}>
            {data.email}
          </span>
        </SidePanelField>
      ) : null}
      {visit?.pageUrl ? (
        <SidePanelField label={t("contextCurrentPage")}>
          <button
            type="button"
            className="min-w-0 truncate text-left hover:underline"
            title={visit.pageUrl}
            onClick={() => void openExternalURL(visit.pageUrl)}
          >
            {visit.pageTitle || visit.pageUrl}
          </button>
        </SidePanelField>
      ) : null}
      {visit?.referrerUrl ? (
        <SidePanelField label={t("contextReferrer")}>
          <button
            type="button"
            className="min-w-0 truncate text-left hover:underline"
            title={visit.referrerUrl}
            onClick={() => void openExternalURL(visit.referrerUrl)}
          >
            {visit.referrerUrl}
          </button>
        </SidePanelField>
      ) : null}
      {device ? (
        <SidePanelField label={t("contextDevice")}>
          <span className="min-w-0 truncate" title={device}>
            {device}
          </span>
        </SidePanelField>
      ) : null}
      {visit?.language ? (
        <SidePanelField label={t("contextLanguage")}>
          {visit.language}
        </SidePanelField>
      ) : null}
      {visit?.timeZone ? (
        <SidePanelField label={t("contextTimeZone")}>
          {visit.timeZone}
        </SidePanelField>
      ) : null}
      {visit?.country ? (
        <SidePanelField label={t("contextCountry")}>
          {visit.country}
        </SidePanelField>
      ) : null}
    </>
  )
}
