/** 管理列表的数据表格和操作列。 */
import type { ReactNode, Ref } from "react"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
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

/** 按列定义渲染表头和单元格，给出 actions 时在最右侧追加操作列，空列表展示占位行；给出 onRowActivate 时整行可点击或用回车触发，给出 rowMenu 时整行右键弹出菜单，hideHeader 用于列含义已经一目了然的列表。 */
export function ResourceTable<T>({
  columns,
  rows,
  rowKey,
  empty,
  actions,
  onRowActivate,
  rowMenu,
  hideHeader = false,
}: {
  columns: readonly ResourceTableColumn<T>[]
  rows: readonly T[]
  rowKey: (row: T) => string
  empty: ReactNode
  actions?: (row: T) => ResourceTableRowActions
  onRowActivate?: (row: T) => void
  /** 返回该行右键菜单的菜单项。 */
  rowMenu?: (row: T) => ReactNode
  hideHeader?: boolean
}) {
  const { t } = useTranslation("common")

  return (
    <Table>
      {hideHeader ? null : (
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
      )}
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
          rows.map((row) => {
            const rowElement = (
              <TableRow
                key={rowKey(row)}
                // 右键菜单打开期间保持该行的悬停底色，标明菜单作用的行。
                className={cn(
                  "h-[65px] data-[state=open]:bg-muted/40",
                  onRowActivate && "cursor-pointer",
                )}
                tabIndex={onRowActivate ? 0 : undefined}
                onClick={onRowActivate ? () => onRowActivate(row) : undefined}
                onKeyDown={
                  onRowActivate
                    ? (event) => {
                        if (event.target !== event.currentTarget) return
                        if (event.key !== "Enter" && event.key !== " ") return
                        event.preventDefault()
                        onRowActivate(row)
                      }
                    : undefined
                }
              >
                {columns.map((column) => (
                  <TableCell
                    key={column.key}
                    className={cn(column.className, column.cellClassName)}
                  >
                    {column.cell(row)}
                  </TableCell>
                ))}
                {actions ? (
                  <ResourceTableActionsCell {...actions(row)} />
                ) : null}
              </TableRow>
            )
            if (!rowMenu) return rowElement
            return (
              <ContextMenu key={rowKey(row)}>
                <ContextMenuTrigger asChild>{rowElement}</ContextMenuTrigger>
                <ContextMenuContent>{rowMenu(row)}</ContextMenuContent>
              </ContextMenu>
            )
          })
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
    <TableCell className="w-px whitespace-nowrap">
      <div
        className="flex items-center gap-1"
        onClick={(event) => event.stopPropagation()}
      >
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
