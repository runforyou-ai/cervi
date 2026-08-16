import type { ApiError } from "@/api/client"

export function apiErrorMessage(
  error: ApiError,
  fieldNames: readonly string[] = [],
) {
  for (const name of fieldNames) {
    const message = error.fields[name]
    if (message) {
      return message
    }
  }

  return Object.values(error.fields).find(Boolean) ?? error.message
}
