import i18n from "i18next"
import LanguageDetector from "i18next-browser-languagedetector"
import { initReactI18next } from "react-i18next"

import {
  defaultNamespace,
  fallbackLanguage,
  resources,
  supportedLanguages,
  type SupportedLanguage,
} from "@/i18n/resources"

const languagePreferenceKey = "cervi.language"

function normalizeLanguage(language: string): SupportedLanguage {
  return language.toLowerCase().startsWith("zh") ? "zh-CN" : "en-US"
}

function syncDocumentLanguage(language: string) {
  const normalizedLanguage = normalizeLanguage(language)
  document.documentElement.lang = normalizedLanguage
  document.documentElement.dir = i18n.dir(normalizedLanguage)
}

i18n.on("languageChanged", syncDocumentLanguage)

export const i18nReady = i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    defaultNS: defaultNamespace,
    fallbackLng: fallbackLanguage,
    supportedLngs: supportedLanguages,
    returnNull: false,
    interpolation: {
      escapeValue: false,
    },
    detection: {
      order: ["localStorage", "navigator"],
      lookupLocalStorage: languagePreferenceKey,
      caches: [],
      convertDetectedLanguage: normalizeLanguage,
    },
    react: {
      useSuspense: false,
    },
  })
  .then(() => {
    syncDocumentLanguage(i18n.resolvedLanguage ?? fallbackLanguage)
    return i18n
  })

export function changeLanguage(language: SupportedLanguage) {
  try {
    localStorage.setItem(languagePreferenceKey, language)
  } catch {
    // Language switching still works when storage is unavailable.
  }
  return i18n.changeLanguage(language)
}

export function followBrowserLanguage() {
  try {
    localStorage.removeItem(languagePreferenceKey)
  } catch {
    // Falling back to the browser language does not require storage.
  }
  const browserLanguage = navigator.languages[0] ?? navigator.language
  return i18n.changeLanguage(normalizeLanguage(browserLanguage))
}

export { i18n }
