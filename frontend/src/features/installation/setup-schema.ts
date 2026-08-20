/** 企业初始化表单校验规则。 */
import { z } from "zod"

type SetupTranslator = (
  key:
    | "organizationNameRequired"
    | "displayNameRequired"
    | "emailRequired"
    | "emailInvalid"
    | "passwordRequired"
    | "passwordTooShort"
    | "passwordTooLong",
) => string

/** 创建企业初始化表单校验。 */
export function createSetupSchema(t: SetupTranslator) {
  return z.object({
    organizationName: z.string().trim().min(1, t("organizationNameRequired")),
    displayName: z.string().trim().min(1, t("displayNameRequired")),
    email: z
      .string()
      .trim()
      .min(1, t("emailRequired"))
      .email(t("emailInvalid")),
    password: z
      .string()
      .min(1, t("passwordRequired"))
      .min(8, t("passwordTooShort"))
      .refine(
        (password) => new TextEncoder().encode(password).length <= 72,
        t("passwordTooLong"),
      ),
  })
}

export type SetupFormValues = z.infer<ReturnType<typeof createSetupSchema>>
