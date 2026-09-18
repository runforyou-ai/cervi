/** 通讯录表单共用的角色选择字段。 */
import type { ComponentProps } from "react"
import { useTranslation } from "react-i18next"

import type { RoleData } from "@/api"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { roleDisplayName } from "@/lib/role-labels"

/** 展示调用方允许选择的角色并保留原生表单校验，hint 说明当前选择对调用方的含义。 */
export function RoleSelectField({
  roles,
  hint,
  ...props
}: ComponentProps<typeof NativeSelect> & {
  roles: RoleData[]
  hint?: string
}) {
  const { t } = useTranslation("contacts")
  const { t: tCommon } = useTranslation("common")
  return (
    <Field data-invalid={props["aria-invalid"]}>
      <FieldLabel htmlFor={props.id} required>
        {t("members.form.role")}
      </FieldLabel>
      <NativeSelect {...props}>
        {roles.map((role) => (
          <option key={role.id} value={role.id}>
            {roleDisplayName(role, tCommon)}
          </option>
        ))}
      </NativeSelect>
      {hint ? <FieldDescription>{hint}</FieldDescription> : null}
    </Field>
  )
}
