/** 初始化 i18next 并同步文档语言。 */
import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import {
  defaultNamespace,
  fallbackLanguage,
  resources,
  supportedLanguages,
} from "@/i18n/resources"

/** 根据浏览器语言选择应用语言。 */
export function resolveBrowserLanguage() {
  const browserLanguage = navigator.languages[0] ?? navigator.language
  const browserLocale = new Intl.Locale(browserLanguage).maximize()

  if (browserLocale.language === "zh" && browserLocale.script === "Hans") {
    return "zh-CN"
  }

  if (browserLocale.language === "en") {
    return "en-US"
  }

  return fallbackLanguage
}

/** 把文档语言和阅读方向同步到当前语言。 */
function syncDocumentLanguage(language: string) {
  document.documentElement.lang = language
  document.documentElement.dir = i18n.dir(language)
}

i18n.on("languageChanged", syncDocumentLanguage)

/** 初始化国际化资源。 */
export async function initializeI18n() {
  const language = resolveBrowserLanguage()

  await i18n.use(initReactI18next).init({
    resources,
    lng: language,
    defaultNS: defaultNamespace,
    fallbackLng: fallbackLanguage,
    supportedLngs: supportedLanguages,
    returnNull: false,
    interpolation: {
      escapeValue: false,
    },
    react: {
      useSuspense: false,
    },
  })

  return i18n
}

export { i18n }
