import { CancelError, type CancellablePromise } from "@wailsio/runtime"

import {
  Locale,
  type RequestMeta,
  type Session,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { i18n } from "@/i18n"
import { fallbackLanguage } from "@/i18n/resources"

const sessionStorageKey = "cervi.session"

type StoredSession = Pick<Session, "token" | "expiresAt">

type ErrorCause = {
  status: number
  code: string
  message: string
  fields?: Record<string, string>
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>

  constructor(
    status: number,
    code: string,
    message: string,
    fields: Record<string, string> = {},
  ) {
    super(message)
    this.name = "ApiError"
    this.status = status
    this.code = code
    this.fields = fields
  }
}

export async function call<T>(
  operation: (meta: RequestMeta) => CancellablePromise<T>,
  signal?: AbortSignal,
): Promise<T> {
  try {
    const pending = operation(requestMeta())
    return await (signal ? pending.cancelOn(signal) : pending)
  } catch (error) {
    throw normalizeError(error)
  }
}

export function acceptSession(session: Session) {
  window.sessionStorage.setItem(
    sessionStorageKey,
    JSON.stringify({ token: session.token, expiresAt: session.expiresAt }),
  )
  return session.principal
}

export function clearSession() {
  window.sessionStorage.removeItem(sessionStorageKey)
}

export function hasSession() {
  return loadToken() !== ""
}

function requestMeta(): RequestMeta {
  return {
    token: loadToken(),
    locale:
      (i18n.resolvedLanguage ?? fallbackLanguage) === "en-US"
        ? Locale.LocaleEnglishUnitedStates
        : Locale.LocaleChineseSimplified,
  }
}

function loadToken() {
  const value = window.sessionStorage.getItem(sessionStorageKey)
  if (!value) return ""
  const session = JSON.parse(value) as StoredSession
  if (Date.parse(session.expiresAt) <= Date.now()) {
    clearSession()
    return ""
  }
  return session.token
}

function normalizeError(error: unknown) {
  if (error instanceof ApiError) return error
  if (error instanceof Error) {
    const cause = (error as Error & { cause?: unknown }).cause
    if (error instanceof CancelError && cause instanceof Error) return cause
    if (isErrorCause(cause)) {
      return new ApiError(
        cause.status,
        cause.code,
        cause.message,
        cause.fields,
      )
    }
    return error
  }
  return new ApiError(500, "UNKNOWN_ERROR", "Request failed")
}

function isErrorCause(value: unknown): value is ErrorCause {
  if (typeof value !== "object" || value === null) return false
  const cause = value as Partial<ErrorCause>
  return (
    typeof cause.status === "number" &&
    typeof cause.code === "string" &&
    typeof cause.message === "string"
  )
}
