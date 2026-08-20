/** 列表页工具栏、搜索框和筛选器。 */
import type { InputHTMLAttributes, ReactNode } from "react"
import { CheckIcon, SearchIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

export type ListToolbarOption = {
  value: string
  label: string
}

/** 列表工具栏容器。 */
export function ListToolbar({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-wrap items-center gap-2 border-b px-4 py-3 select-none sm:px-6">
      {children}
    </div>
  )
}

/** 列表搜索输入框。 */
export function ListToolbarSearch({
  className,
  ...props
}: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div className={cn("relative w-full sm:w-64", className)}>
      <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input {...props} className="h-8 pl-8" />
    </div>
  )
}

/** 列表筛选下拉。 */
export function ListToolbarFilter({
  label,
  allLabel,
  value,
  options,
  onValueChange,
  align = "start",
}: {
  label: string
  allLabel?: string
  value: string
  options: ListToolbarOption[]
  onValueChange: (value: string) => void
  align?: "start" | "center" | "end"
}) {
  const selected = options.find((option) => option.value === value)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="sm">
          {label}
          {selected ? (
            <>
              <span className="h-4 w-px bg-border" />
              <span className="font-normal">{selected.label}</span>
            </>
          ) : null}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align={align} className="min-w-44">
        <DropdownMenuLabel>{label}</DropdownMenuLabel>
        <DropdownMenuSeparator />
        {allLabel ? (
          <DropdownMenuItem onSelect={() => onValueChange("")}>
            <CheckIcon className={cn(!value && "opacity-100", value && "opacity-0")} />
            {allLabel}
          </DropdownMenuItem>
        ) : null}
        {options.map((option) => (
          <DropdownMenuItem
            key={option.value}
            onSelect={() => onValueChange(option.value)}
          >
            <CheckIcon className={cn(value === option.value ? "opacity-100" : "opacity-0")} />
            {option.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** 清除列表筛选。 */
export function ListToolbarReset({
  children,
  onClick,
}: {
  children: ReactNode
  onClick: () => void
}) {
  return (
    <Button variant="ghost" size="sm" onClick={onClick}>
      {children}
    </Button>
  )
}
