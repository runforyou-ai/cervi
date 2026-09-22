/** 管理列表的数据表格和行操作菜单。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import {
  RowActionsMenu,
  type ResourceRowAction,
} from "@/components/row-actions-menu"
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

export type { ResourceRowAction } from "@/components/row-actions-menu"

/** 按列定义渲染表头和单元格，空列表展示占位行；给出 onRowActivate 时整行可点击或用回车触发，给出 rowActions 时在最右侧追加操作列，有操作的行可右键或点「⋯」打开同一份操作菜单，hideHeader 用于列含义已经一目了然的列表。 */
export function ResourceTable<T>({
  columns,
  rows,
  rowKey,
  empty,
  rowActions,
  onRowActivate,
  hideHeader = false,
}: {
  columns: readonly ResourceTableColumn<T>[]
  rows: readonly T[]
  rowKey: (row: T) => string
  empty: ReactNode
  rowActions?: (row: T) => ResourceRowAction[]
  onRowActivate?: (row: T) => void
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
            {rowActions ? (
              <TableHead className="w-px">{t("table.actions")}</TableHead>
            ) : null}
          </TableRow>
        </TableHeader>
      )}
      <TableBody>
        {rows.length === 0 ? (
          <TableRow className="hover:bg-transparent">
            <TableCell
              colSpan={columns.length + (rowActions ? 1 : 0)}
              className="h-32 text-center text-muted-foreground"
            >
              {empty}
            </TableCell>
          </TableRow>
        ) : (
          rows.map((row) => (
            <ResourceTableRow
              key={rowKey(row)}
              row={row}
              columns={columns}
              actions={rowActions?.(row)}
              onRowActivate={onRowActivate}
            />
          ))
        )}
      </TableBody>
    </Table>
  )
}

/** 渲染一行数据；actions 非空时整行右键和行尾「⋯」按钮打开同一份操作菜单。 */
function ResourceTableRow<T>({
  row,
  columns,
  actions,
  onRowActivate,
}: {
  row: T
  columns: readonly ResourceTableColumn<T>[]
  actions: ResourceRowAction[] | undefined
  onRowActivate?: (row: T) => void
}) {
  return (
    <RowActionsMenu actions={actions ?? []}>
      {({ moreButton, menuOpen }) => (
        <TableRow
          // 菜单打开期间保持该行的悬停底色，标明菜单作用的行。
          className={cn(
            "group/row h-[65px]",
            menuOpen && "bg-muted/40",
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
            <TableCell className="w-px whitespace-nowrap">
              <div className="flex justify-end">{moreButton}</div>
            </TableCell>
          ) : null}
        </TableRow>
      )}
    </RowActionsMenu>
  )
}
