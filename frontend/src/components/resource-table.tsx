/** 管理列表的数据表格和行操作菜单。 */
import { useRef, useState, type ReactNode } from "react"
import { flushSync } from "react-dom"
import { MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
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

/** 一行的一项操作，同时出现在右键菜单和行尾「⋯」菜单中；separatorBefore 在该项前加分隔线。 */
export type ResourceRowAction = {
  key: string
  label: string
  onSelect: () => void
  disabled?: boolean
  destructive?: boolean
  separatorBefore?: boolean
}

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

/** 渲染一行数据；actions 非空时整行右键和行尾「⋯」按钮打开同一份操作菜单，菜单项执行后焦点交给该行的「⋯」按钮。 */
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
  const { t } = useTranslation("common")
  const [contextOpen, setContextOpen] = useState(false)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const moreButton = useRef<HTMLButtonElement>(null)
  const actionSelected = useRef(false)
  const menuOpen = contextOpen || dropdownOpen
  const moreLabel = t("actions.moreActions")

  /** 同步关闭菜单解除焦点锁定，把焦点交给「⋯」按钮后执行操作，操作打开的弹窗关闭时焦点回到该按钮。 */
  function selectAction(event: Event, action: ResourceRowAction) {
    event.preventDefault()
    actionSelected.current = true
    flushSync(() => {
      setContextOpen(false)
      setDropdownOpen(false)
    })
    moreButton.current?.focus()
    action.onSelect()
  }

  /** 菜单关闭后焦点回到「⋯」按钮；已执行操作时焦点已交给按钮或操作打开的弹窗。 */
  function restoreFocus(event: Event) {
    event.preventDefault()
    if (!actionSelected.current) moreButton.current?.focus()
    actionSelected.current = false
  }

  const rowElement = (
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
          {actions.length > 0 ? (
            <div
              className="flex justify-end"
              onClick={(event) => event.stopPropagation()}
            >
              <DropdownMenu open={dropdownOpen} onOpenChange={setDropdownOpen}>
                <DropdownMenuTrigger asChild>
                  <Button
                    ref={moreButton}
                    variant="ghost"
                    size="icon-sm"
                    aria-label={moreLabel}
                    title={moreLabel}
                    // 悬停或键盘聚焦该行、聚焦按钮以及菜单打开期间显示。
                    className={cn(
                      "opacity-0 group-hover/row:opacity-100 group-focus-visible/row:opacity-100 focus-visible:opacity-100",
                      menuOpen && "opacity-100",
                    )}
                  >
                    <MoreHorizontalIcon />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent
                  align="end"
                  onCloseAutoFocus={restoreFocus}
                  // 菜单内容经 Portal 渲染但仍在该行的事件树中，右键不再冒泡打开整行的右键菜单。
                  onContextMenu={(event) => {
                    event.preventDefault()
                    event.stopPropagation()
                  }}
                >
                  {actions.map((action, index) => (
                    <DropdownMenuActionItem
                      key={action.key}
                      action={action}
                      separated={Boolean(action.separatorBefore) && index > 0}
                      onSelect={selectAction}
                    />
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          ) : null}
        </TableCell>
      ) : null}
    </TableRow>
  )
  if (!actions?.length) return rowElement

  return (
    <ContextMenu open={contextOpen} onOpenChange={setContextOpen}>
      <ContextMenuTrigger asChild>{rowElement}</ContextMenuTrigger>
      <ContextMenuContent onCloseAutoFocus={restoreFocus}>
        {actions.map((action, index) => (
          <ContextMenuActionItem
            key={action.key}
            action={action}
            separated={Boolean(action.separatorBefore) && index > 0}
            onSelect={selectAction}
          />
        ))}
      </ContextMenuContent>
    </ContextMenu>
  )
}

/** 行尾「⋯」菜单中的一项操作，separated 时在前面加分隔线。 */
function DropdownMenuActionItem({
  action,
  separated,
  onSelect,
}: {
  action: ResourceRowAction
  separated: boolean
  onSelect: (event: Event, action: ResourceRowAction) => void
}) {
  return (
    <>
      {separated ? <DropdownMenuSeparator /> : null}
      <DropdownMenuItem
        disabled={action.disabled}
        destructive={action.destructive}
        onSelect={(event) => onSelect(event, action)}
      >
        {action.label}
      </DropdownMenuItem>
    </>
  )
}

/** 整行右键菜单中的一项操作，separated 时在前面加分隔线。 */
function ContextMenuActionItem({
  action,
  separated,
  onSelect,
}: {
  action: ResourceRowAction
  separated: boolean
  onSelect: (event: Event, action: ResourceRowAction) => void
}) {
  return (
    <>
      {separated ? <ContextMenuSeparator /> : null}
      <ContextMenuItem
        disabled={action.disabled}
        destructive={action.destructive}
        onSelect={(event) => onSelect(event, action)}
      >
        {action.label}
      </ContextMenuItem>
    </>
  )
}
