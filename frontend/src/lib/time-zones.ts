/** 提供浏览器时区默认值和完整 IANA 时区选项。 */

type IntlWithSupportedValues = typeof Intl & {
  supportedValuesOf?: (key: "timeZone") => string[]
}

/** 返回浏览器当前 IANA 时区，无法读取时使用 UTC。 */
export function resolveBrowserTimeZone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC"
}

/** 返回当前运行环境支持的全部 IANA 时区。 */
export function supportedTimeZones(current?: string) {
  const values =
    (Intl as IntlWithSupportedValues).supportedValuesOf?.("timeZone") ?? []
  return Array.from(
    new Set(["UTC", current ?? resolveBrowserTimeZone(), ...values]),
  ).sort((left, right) => left.localeCompare(right))
}
