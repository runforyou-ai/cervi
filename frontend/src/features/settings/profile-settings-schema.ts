/** 个人资料表单校验规则。 */
import { z } from "zod"

type ProfileTranslator = (
  key:
    | "profile.validation.displayNameRequired"
    | "profile.validation.emailRequired"
    | "profile.validation.emailInvalid",
) => string

/** 创建个人资料表单校验。 */
export function createProfileSettingsSchema(t: ProfileTranslator) {
  return z.object({
    displayName: z
      .string()
      .trim()
      .min(1, t("profile.validation.displayNameRequired")),
    email: z
      .string()
      .trim()
      .min(1, t("profile.validation.emailRequired"))
      .email(t("profile.validation.emailInvalid")),
  })
}

export type ProfileSettingsFormValues = z.infer<
  ReturnType<typeof createProfileSettingsSchema>
>
