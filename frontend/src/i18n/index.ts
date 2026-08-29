/** 初始化 i18next 并同步文档语言。 */
import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import {
  defaultNamespace,
  fallbackLanguage,
  resources,
  supportedLanguages,
  type SupportedLanguage,
} from "@/i18n/resources"

const localeStorageKey = "cervi.locale"

/** 判断语言是否受当前应用支持。 */
function isSupportedLanguage(language: string): language is SupportedLanguage {
  return supportedLanguages.some((supported) => supported === language)
}

/** 读取本机缓存的最近一次界面语言。 */
function readStoredLanguage(): SupportedLanguage | null {
  try {
    const language = window.localStorage.getItem(localeStorageKey)
    return language && isSupportedLanguage(language) ? language : null
  } catch (error) {
    console.warn("读取本机语言偏好失败", error)
    return null
  }
}

/** 缓存当前界面语言供下次启动使用。 */
function storeLanguage(language: SupportedLanguage) {
  try {
    window.localStorage.setItem(localeStorageKey, language)
  } catch (error) {
    console.warn("保存本机语言偏好失败", error)
  }
}

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

/** 切换界面语言并同步本机启动缓存。 */
export async function changeAppLanguage(language: string) {
  if (!isSupportedLanguage(language)) {
    console.warn("忽略不支持的界面语言", { language })
    return
  }
  await i18n.changeLanguage(language)
  storeLanguage(language)
}

/** 把文档语言和阅读方向同步到当前语言。 */
function syncDocumentLanguage(language: string) {
  document.documentElement.lang = language
  document.documentElement.dir = i18n.dir(language)
}

i18n.on("languageChanged", syncDocumentLanguage)

/** 初始化国际化资源。 */
export async function initializeI18n() {
  const language = readStoredLanguage() ?? resolveBrowserLanguage()

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
