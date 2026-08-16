import type { ComponentProps, ReactNode } from "react"
import {
  Controller,
  type Control,
  type FieldPathByValue,
  type FieldValues,
} from "react-hook-form"

import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"

type FormInputFieldProps<T extends FieldValues> = {
  control: Control<T>
  name: FieldPathByValue<T, string>
  label: ReactNode
  id?: string
  required?: boolean
} & Omit<
  ComponentProps<typeof Input>,
  | "id"
  | "name"
  | "value"
  | "defaultValue"
  | "onChange"
  | "onBlur"
  | "ref"
  | "required"
  | "aria-invalid"
  | "aria-describedby"
>

export function FormInputField<T extends FieldValues>({
  control,
  name,
  label,
  id = name,
  required = true,
  ...inputProps
}: FormInputFieldProps<T>) {
  return (
    <Controller
      name={name}
      control={control}
      render={({ field }) => (
        <Field className="gap-1.5">
          <FieldLabel htmlFor={id} required={required}>
            {label}
          </FieldLabel>
          <Input
            {...field}
            {...inputProps}
            id={id}
            required={required}
          />
        </Field>
      )}
    />
  )
}
