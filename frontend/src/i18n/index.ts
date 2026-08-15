import i18n from "i18next"
import { initReactI18next } from "react-i18next"

import {
  defaultNamespace,
  fallbackLanguage,
  resources,
  supportedLanguages,
  type SupportedLanguage,
} from "@/i18n/resources"

function resolveBrowserLanguage(): SupportedLanguage {
  const browserLanguage = navigator.languages[0] ?? navigator.language
  const normalizedLanguage = browserLanguage.toLowerCase()

  if (normalizedLanguage.startsWith("zh")) {
    return "zh-CN"
  }

  if (normalizedLanguage.startsWith("en")) {
    return "en-US"
  }

  console.warn("[i18n] 浏览器语言暂未支持，使用默认语言", {
    browserLanguage,
    fallbackLanguage,
  })
  return fallbackLanguage
}

function syncDocumentLanguage(language: string) {
  document.documentElement.lang = language
  document.documentElement.dir = i18n.dir(language)
}

i18n.on("languageChanged", syncDocumentLanguage)

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

  console.info("[i18n] 初始化完成", {
    language: i18n.resolvedLanguage ?? language,
  })

  return i18n
}

export async function changeLanguage(language: SupportedLanguage) {
  await i18n.changeLanguage(language)
  console.info("[i18n] 语言已切换", { language })
}

export function followBrowserLanguage() {
  return changeLanguage(resolveBrowserLanguage())
}

export { i18n }
