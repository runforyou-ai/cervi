/** 注入请求认证信息，并把应用服务错误转换为前端错误。 */
import { CancelError, type CancellablePromise } from "@wailsio/runtime"

import {
  Locale,
  type Auth,
  type RequestMeta,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { i18n } from "@/i18n"
import { fallbackLanguage } from "@/i18n/resources"

const tokenStorageKey = "cervi.token"

type StoredToken = Pick<Auth, "token" | "expiresAt">

type ErrorCause = {
  status: number
  code: string
  message: string
  fields?: Record<string, string>
}

/** 应用服务返回的结构化业务错误。 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string>

  /** 创建结构化业务错误。 */
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

/** 注入认证和语言后调用应用服务。卸载时忽略结果，不取消绑定。 */
export async function call<T>(
  operation: (meta: RequestMeta) => CancellablePromise<T>,
  signal?: AbortSignal,
): Promise<T> {
  try {
    const result = await operation(requestMeta())
    if (signal?.aborted) {
      throw abortError()
    }
    return result
  } catch (error) {
    if (signal?.aborted) {
      throw abortError()
    }
    throw normalizeError(error)
  }
}

/** 返回调用方已离开后应忽略的中止错误。 */
function abortError() {
  return new DOMException("The operation was aborted.", "AbortError")
}

/** 把应用服务方法包装成自动注入认证信息的前端调用。 */
export function bind<A extends unknown[], R>(
  fn: (meta: RequestMeta, ...args: A) => CancellablePromise<R>,
) {
  return (...args: [...A, AbortSignal?]) => {
    const last = args[args.length - 1]
    const signal = last instanceof AbortSignal ? last : undefined
    const fnArgs = (
      (signal || last === undefined) && args.length > 0
        ? args.slice(0, -1)
        : args
    ) as A
    return call((meta) => fn(meta, ...fnArgs), signal)
  }
}

/** 保存登录令牌并返回当前身份。 */
export function storeToken(auth: Auth) {
  window.localStorage.setItem(
    tokenStorageKey,
    JSON.stringify({ token: auth.token, expiresAt: auth.expiresAt }),
  )
  return auth.identity
}

/** 清除本地保存的登录令牌。 */
export function clearToken() {
  window.localStorage.removeItem(tokenStorageKey)
}

/** 判断本地是否仍有未过期的登录令牌。 */
export function hasToken() {
  return loadToken() !== ""
}

/** 组装当前请求的令牌和语言。 */
function requestMeta(): RequestMeta {
  return {
    token: loadToken(),
    locale:
      (i18n.resolvedLanguage ?? fallbackLanguage) === "en-US"
        ? Locale.LocaleEnglishUnitedStates
        : Locale.LocaleChineseSimplified,
  }
}

/** 读取未过期的登录令牌。 */
function loadToken() {
  const value = window.localStorage.getItem(tokenStorageKey)
  if (!value) return ""
  const stored = JSON.parse(value) as StoredToken
  if (Date.parse(stored.expiresAt) <= Date.now()) {
    clearToken()
    return ""
  }
  return stored.token
}

/** 把应用服务异常转换为前端错误。 */
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

/** 判断异常原因是否为结构化业务错误。 */
function isErrorCause(value: unknown): value is ErrorCause {
  if (typeof value !== "object" || value === null) return false
  const cause = value as Partial<ErrorCause>
  return (
    typeof cause.status === "number" &&
    typeof cause.code === "string" &&
    typeof cause.message === "string"
  )
}
