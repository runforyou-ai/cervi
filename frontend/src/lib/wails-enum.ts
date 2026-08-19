import { z } from "zod"

type WailsStringEnum = Record<string, string> & { $zero: string }

type WailsEnumValue<Enum extends WailsStringEnum> = Enum[keyof Enum]

export function requiredWailsEnum<Enum extends WailsStringEnum>(
  values: Enum,
  message?: string,
) {
  return z.nativeEnum(values).refine(
    (value): boolean => value !== values.$zero,
    message,
  )
}

export function parseWailsEnum<Enum extends WailsStringEnum>(
  values: Enum,
  value: string | null,
  fallback: WailsEnumValue<Enum>,
): WailsEnumValue<Enum> {
  return value !== null && Object.values(values).includes(value)
    ? (value as WailsEnumValue<Enum>)
    : fallback
}
