import { fallbackLanguage } from "@/i18n/resources"
import { i18n } from "@/i18n"

type ErrorResponse = {
  error?: {
    code?: string
    message?: string
    fields?: Record<string, string>
  }
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

export async function request<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const headers = new Headers(init?.headers)
  if (init?.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json")
  }
  if (!headers.has("Accept-Language")) {
    headers.set(
      "Accept-Language",
      i18n.resolvedLanguage ?? fallbackLanguage,
    )
  }

  const response = await fetch(`/api${path}`, {
    ...init,
    headers,
    credentials: "include",
  })

  if (!response.ok) {
    const payload = (await response.json().catch(() => ({}))) as ErrorResponse
    throw new ApiError(
      response.status,
      payload.error?.code ?? "UNKNOWN_ERROR",
      payload.error?.message ?? "Request failed",
      payload.error?.fields,
    )
  }

  if (response.status === 204) {
    return undefined as T
  }

  return (await response.json()) as T
}
