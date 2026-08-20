/** 注册应用支持的语言和翻译资源。 */
import enUS from "@/i18n/locales/en-US"
import zhCN from "@/i18n/locales/zh-CN"

type LocaleShape<T> = {
  readonly [Key in keyof T]: T[Key] extends string
    ? string
    : LocaleShape<T[Key]>
}

export const defaultNamespace = "common"
export const fallbackLanguage = "en-US"
export const supportedLanguages = ["zh-CN", "en-US"] as const

export type SupportedLanguage = (typeof supportedLanguages)[number]

const validatedZhCN: LocaleShape<typeof enUS> = zhCN

export const resources = {
  "en-US": enUS,
  "zh-CN": validatedZhCN,
} as const
