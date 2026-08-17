import { type ComponentProps, type ReactNode, useState } from "react"
import { EyeIcon, EyeOffIcon } from "lucide-react"
import {
  Controller,
  type Control,
  type FieldPathByValue,
  type FieldValues,
} from "react-hook-form"

import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

type FormInputFieldProps<T extends FieldValues> = {
  control: Control<T>
  name: FieldPathByValue<T, string>
  label: ReactNode
  id?: string
  required?: boolean
  passwordVisibilityLabels?: {
    show: string
    hide: string
  }
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
  passwordVisibilityLabels,
  className,
  ...inputProps
}: FormInputFieldProps<T>) {
  const [passwordVisible, setPasswordVisible] = useState(false)

  return (
    <Controller
      name={name}
      control={control}
      render={({ field }) => {
        const input = (
          <Input
            {...field}
            {...inputProps}
            id={id}
            type={
              passwordVisibilityLabels
                ? passwordVisible
                  ? "text"
                  : "password"
                : inputProps.type
            }
            className={cn(passwordVisibilityLabels && "pr-10", className)}
            required={required}
          />
        )

        return (
          <Field>
            <FieldLabel htmlFor={id} required={required}>
              {label}
            </FieldLabel>
            {passwordVisibilityLabels ? (
              <div className="relative">
                {input}
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="absolute top-1/2 right-1 -translate-y-1/2"
                  aria-label={
                    passwordVisible
                      ? passwordVisibilityLabels.hide
                      : passwordVisibilityLabels.show
                  }
                  aria-pressed={passwordVisible}
                  onClick={() => setPasswordVisible((visible) => !visible)}
                >
                  {passwordVisible ? <EyeOffIcon /> : <EyeIcon />}
                </Button>
              </div>
            ) : (
              input
            )}
          </Field>
        )
      }}
    />
  )
}
