import type {
  FieldPath,
  FieldValues,
  UseFormSetError,
} from "react-hook-form"

export function applyServerFieldErrors<T extends FieldValues>(
  setError: UseFormSetError<T>,
  fields: Record<string, string>,
  fieldNames: readonly FieldPath<T>[],
) {
  const errors = Object.entries(fields).filter(([, message]) => message)
  const allowedFields = new Set<string>(fieldNames)
  const fieldErrors = errors.filter(([name]) => allowedFields.has(name))

  fieldErrors.forEach(([name, message], index) => {
    setError(
      name as FieldPath<T>,
      { type: "server", message },
      { shouldFocus: index === 0 },
    )
  })

  return errors.length > 0 && fieldErrors.length === errors.length
}
