/** 企业成员、AI 员工与助理列表共用的账号状态筛选、启停操作和确认流程。 */
import type { QueryKey } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"

import { UserStatus } from "@/api"
import { ListToolbarFilter } from "@/components/list-toolbar"
import type { ResourceRowAction } from "@/components/resource-table"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"

type AccountScope = "members" | "agents" | "assistants"

type AccountItem = {
  id: string
  status: UserStatus
  displayName: string
}

/** 按账号状态筛选列表，默认显示正常账号。 */
export function AccountStatusFilter({
  value,
  setParameters,
}: {
  value: UserStatus
  setParameters: (changes: Record<string, string | null>) => void
}) {
  const { t } = useTranslation("contacts")
  return (
    <ListToolbarFilter
      label={t("filters.accountStatus")}
      value={value}
      options={[
        { value: UserStatus.UserStatusActive, label: t("statuses.active") },
        { value: UserStatus.UserStatusInactive, label: t("statuses.inactive") },
      ]}
      onValueChange={(next) =>
        setParameters({
          status: next === UserStatus.UserStatusActive ? null : next,
          page: null,
          selected: null,
        })
      }
    />
  )
}

/** 确认后禁用正常账号或恢复已禁用账号，返回行操作菜单项和确认框属性。 */
export function useAccountStatusToggle<T extends AccountItem>({
  scope,
  deactivate,
  reactivate,
  invalidateKeys,
  logLabel,
}: {
  scope: AccountScope
  deactivate: (id: string) => Promise<unknown>
  reactivate: (id: string) => Promise<unknown>
  invalidateKeys: (item: T) => QueryKey[]
  logLabel: string
}) {
  const { t } = useTranslation("contacts")
  const action = useConfirmedAction<T>({
    action: (item) =>
      item.status === UserStatus.UserStatusActive
        ? deactivate(item.id)
        : reactivate(item.id),
    invalidateKeys,
    successMessage: (item) =>
      t(
        item.status === UserStatus.UserStatusActive
          ? `${scope}.status.deactivated`
          : `${scope}.status.reactivated`,
      ),
    errorMessage: () => t(`${scope}.status.error`),
    logLabel,
  })
  const deactivating = action.item?.status === UserStatus.UserStatusActive

  return {
    /** 返回列表行的启停菜单项：正常账号为危险的禁用操作，已禁用账号为恢复正常。 */
    rowAction: (item: T): ResourceRowAction => {
      const active = item.status === UserStatus.UserStatusActive
      return {
        key: "status",
        label: t(active ? `${scope}.status.deactivate` : `${scope}.status.reactivate`),
        onSelect: () => action.select(item),
        destructive: active,
        separatorBefore: active,
      }
    },
    dialog: {
      open: action.item !== null,
      pending: action.pending,
      title: t(
        deactivating ? `${scope}.status.deactivateTitle` : `${scope}.status.reactivateTitle`,
        { name: action.item?.displayName ?? "" },
      ),
      description: t(
        deactivating
          ? `${scope}.status.deactivateDescription`
          : `${scope}.status.reactivateDescription`,
      ),
      destructive: deactivating,
      pendingLabel: t(`${scope}.status.saving`),
      onOpenChange: (open: boolean) => {
        if (!open) action.select(null)
      },
      onConfirm: () => void action.confirm(),
    },
  }
}
