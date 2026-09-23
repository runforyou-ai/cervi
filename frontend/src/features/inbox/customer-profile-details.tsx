/** 客户会话侧栏的客户身份与当前周期访客上下文。 */
import type { ComponentType, ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { getCustomerProfile } from "@/api"
import { SidePanelField } from "@/features/inbox/side-panel-layout"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { openExternalURL } from "@/platform/external-navigation"

/** 客户资料字段行组件，需放在 dl 内。 */
export type CustomerProfileField = ComponentType<{
  label: string
  children: ReactNode
}>

/** 按会话最新消息刷新客户身份与访客上下文，只展示有值的字段；身份验证状态只在网站渠道展示；字段行默认使用侧栏样式。 */
export function CustomerProfileDetails({
  conversationID,
  lastMessageID,
  website,
  field: Field = SidePanelField,
}: {
  conversationID: string
  lastMessageID: string | null
  website: boolean
  field?: CustomerProfileField
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
        <Field label={t("contextVerification")}>
          {data.identityVerified ? t("contextVerified") : t("contextUnverified")}
        </Field>
      ) : null}
      {data.externalUserId ? (
        <Field label={t("contextExternalUserId")}>
          <span className="min-w-0 truncate" title={data.externalUserId}>
            {data.externalUserId}
          </span>
        </Field>
      ) : null}
      {data.email ? (
        <Field label={t("contextEmail")}>
          <span className="min-w-0 truncate" title={data.email}>
            {data.email}
          </span>
        </Field>
      ) : null}
      {visit?.pageUrl ? (
        <Field label={t("contextCurrentPage")}>
          <button
            type="button"
            className="min-w-0 truncate text-left hover:underline"
            title={visit.pageUrl}
            onClick={() => void openExternalURL(visit.pageUrl)}
          >
            {visit.pageTitle || visit.pageUrl}
          </button>
        </Field>
      ) : null}
      {visit?.referrerUrl ? (
        <Field label={t("contextReferrer")}>
          <button
            type="button"
            className="min-w-0 truncate text-left hover:underline"
            title={visit.referrerUrl}
            onClick={() => void openExternalURL(visit.referrerUrl)}
          >
            {visit.referrerUrl}
          </button>
        </Field>
      ) : null}
      {device ? (
        <Field label={t("contextDevice")}>
          <span className="min-w-0 truncate" title={device}>
            {device}
          </span>
        </Field>
      ) : null}
      {visit?.language ? (
        <Field label={t("contextLanguage")}>
          {visit.language}
        </Field>
      ) : null}
      {visit?.timeZone ? (
        <Field label={t("contextTimeZone")}>
          {visit.timeZone}
        </Field>
      ) : null}
      {visit?.country ? (
        <Field label={t("contextCountry")}>
          {visit.country}
        </Field>
      ) : null}
    </>
  )
}
