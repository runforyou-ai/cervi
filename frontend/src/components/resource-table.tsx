/** 管理列表的数据表格和操作列。 */
import type { ReactNode, Ref } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"

/** 一列的表头、单元格和样式；className 同时作用于表头和单元格，headerClassName 与 cellClassName 分别追加。 */
export type ResourceTableColumn<T> = {
  key: string
  header: ReactNode
  className?: string
  headerClassName?: string
  cellClassName?: string
  cell: (row: T) => ReactNode
}

/** 一行末尾的操作，直接展示的操作在前，低频操作收进三点菜单。 */
export type ResourceTableRowActions = {
  primary?: ReactNode
  menu?: ReactNode
  menuLabel?: string
  menuTriggerRef?: Ref<HTMLButtonElement>
}

/** 按列定义渲染表头和单元格，给出 actions 时在最右侧追加操作列，空列表展示占位行。 */
export function ResourceTable<T>({
  columns,
  rows,
  rowKey,
  empty,
  actions,
}: {
  columns: readonly ResourceTableColumn<T>[]
  rows: readonly T[]
  rowKey: (row: T) => string
  empty: ReactNode
  actions?: (row: T) => ResourceTableRowActions
}) {
  const { t } = useTranslation("common")

  return (
    <Table>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          {columns.map((column) => (
            <TableHead
              key={column.key}
              className={cn(column.className, column.headerClassName)}
            >
              {column.header}
            </TableHead>
          ))}
          {actions ? (
            <TableHead className="w-px">{t("table.actions")}</TableHead>
          ) : null}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow className="hover:bg-transparent">
            <TableCell
              colSpan={columns.length + (actions ? 1 : 0)}
              className="h-32 text-center text-muted-foreground"
            >
              {empty}
            </TableCell>
          </TableRow>
        ) : (
          rows.map((row) => (
            <TableRow key={rowKey(row)}>
              {columns.map((column) => (
                <TableCell
                  key={column.key}
                  className={cn(column.className, column.cellClassName)}
                >
                  {column.cell(row)}
                </TableCell>
              ))}
              {actions ? <ResourceTableActionsCell {...actions(row)} /> : null}
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
  )
}

/** 渲染一行最右侧的操作列。 */
function ResourceTableActionsCell({
  primary,
  menu,
  menuLabel,
  menuTriggerRef,
}: ResourceTableRowActions) {
  const { t } = useTranslation("common")
  const label = menuLabel ?? t("actions.more")

  return (
    <TableCell className="whitespace-nowrap">
      <div className="inline-flex items-center gap-2">
        {primary}
        {menu ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                ref={menuTriggerRef}
                variant="ghost"
                size="icon-sm"
                aria-label={label}
                title={label}
              >
                <MoreHorizontalIcon />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">{menu}</DropdownMenuContent>
          </DropdownMenu>
        ) : null}
      </div>
    </TableCell>
  )
}
