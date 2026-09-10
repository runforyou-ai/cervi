/** 管理列表的数据表格和操作列。 */
import type { ReactNode } from "react"
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

/** 表头内容与列样式，列样式只作用于表头，单元格样式写在各自的 TableCell 上。 */
export type ResourceTableColumn = {
  key: string
  header: ReactNode
  className?: string
}

/** 渲染带表头的数据表格，空列表按当前列数展示占位行；children 返回该行的各个 TableCell。 */
export function ResourceTable<T>({
  columns,
  rows,
  rowKey,
  empty,
  children,
}: {
  columns: readonly ResourceTableColumn[]
  rows: readonly T[]
  rowKey: (row: T) => string
  empty: ReactNode
  children: (row: T) => ReactNode
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow className="hover:bg-transparent">
          {columns.map((column) => (
            <TableHead key={column.key} className={column.className}>
              {column.header}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.length === 0 ? (
          <TableRow className="hover:bg-transparent">
            <TableCell
              colSpan={columns.length}
              className="h-32 text-center text-muted-foreground"
            >
              {empty}
            </TableCell>
          </TableRow>
        ) : (
          rows.map((row) => (
            <TableRow key={rowKey(row)}>{children(row)}</TableRow>
          ))
        )}
      </TableBody>
    </Table>
  )
}

/** 表格最右侧的操作列，自带 TableCell；直接展示的操作在前，低频操作收进三点菜单。 */
export function ResourceTableActions({
  children,
  menu,
}: {
  children?: ReactNode
  menu?: ReactNode
}) {
  const { t } = useTranslation("common")

  return (
    <TableCell className="whitespace-nowrap">
      <div className="inline-flex items-center gap-2">
        {children}
        {menu ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                aria-label={t("actions.more")}
                title={t("actions.more")}
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
