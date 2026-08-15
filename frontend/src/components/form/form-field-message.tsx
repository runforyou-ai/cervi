import { FieldError } from "@/components/ui/field"
import { cn } from "@/lib/utils"

type FormFieldMessageProps = {
  id?: string
  error?: { message?: string }
  className?: string
}

export function FormFieldMessage({
  id,
  error,
  className,
}: FormFieldMessageProps) {
  return (
    <div id={id} className={cn("min-h-5", className)}>
      {error ? <FieldError errors={[error]} /> : null}
    </div>
  )
}
