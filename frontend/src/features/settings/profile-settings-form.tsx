/** 个人资料设置表单占位。 */
import { useTranslation } from "react-i18next"

/** 为后续个人资料字段保留表单区域。 */
export function ProfileSettingsForm() {
  const { t } = useTranslation("settings")

  return <form aria-label={t("profile.formLabel")} />
}
