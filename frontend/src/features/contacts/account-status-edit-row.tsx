/** 企业成员与 AI 员工共用的账号状态编辑。 */
import { useEffect, type KeyboardEvent } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import { UserStatus, isApiError, isNotFoundApiError } from "@/api"
import { DetailEditRow } from "@/components/form/detail-edit-row"
import { StatusBadge } from "@/components/status-badge"
import { NativeSelect } from "@/components/ui/native-select"
import {
  accountStatuses,
  accountStatusSchema,
  type AccountStatusFormValues,
} from "@/features/contacts/account-status-schema"
import { userStatusLabel } from "@/features/contacts/external/contact-labels"
import type { useImmediateSave } from "@/hooks/use-immediate-save"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 即时保存账号状态，并与详情页共用保存锁。 */
export function AccountStatusEditRow<T>({
  status,
  editing,
  editEnabled,
  saveState,
  onEdit,
  onCancel,
  onSave,
  onSaved,
  onNotFound,
  errorMessage,
  entityName,
  entityId,
}: {
  status: UserStatus
  editing: boolean
  editEnabled: boolean
  saveState: ReturnType<typeof useImmediateSave>
  onEdit: () => void
  onCancel: () => void
  onSave: (status: UserStatus) => Promise<T>
  onSaved: (value: T) => void
  onNotFound: () => void
  errorMessage: string
  entityName: string
  entityId: string
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const form = useForm<AccountStatusFormValues>({
    resolver: zodResolver(accountStatusSchema),
    shouldUseNativeValidation: true,
    defaultValues: { status },
  })
  useEffect(() => {
    form.reset({ status })
  }, [editing, form, status])

  /** 校验并保存账号状态，失败时退出编辑。 */
  async function save(nextStatus: UserStatus) {
    if (nextStatus === status) {
      onCancel()
      return
    }
    const request = saveState.begin()
    if (request === null) return
    try {
      const valid = await form.trigger()
      if (!saveState.isCurrent(request) || !valid) return
      const saved = await onSave(nextStatus)
      if (!saveState.isCurrent(request)) return
      onCancel()
      onSaved(saved)
    } catch (error) {
      if (!saveState.isCurrent(request)) return
      onCancel()
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        onNotFound()
        return
      }
      console.warn(`修改${entityName}账号状态失败`, {
        id: entityId,
        status: nextStatus,
        error,
      })
      toast.error(isApiError(error) ? apiErrorMessage(error) : errorMessage)
    } finally {
      saveState.finish(request)
    }
  }

  return (
    <DetailEditRow
      label={t("columns.accountStatus")}
      value={
        <StatusBadge
          showDot={false}
          variant={status === UserStatus.UserStatusActive ? "success" : "muted"}
        >
          {userStatusLabel(status, t)}
        </StatusBadge>
      }
      editing={editing}
      editEnabled={editEnabled}
      required
      onEdit={onEdit}
    >
      <Controller
        name="status"
        control={form.control}
        render={({ field }) => (
          <NativeSelect
            {...field}
            autoFocus
            disabled={saveState.saving}
            onChange={(event) => {
              const nextStatus = event.target.value as UserStatus
              field.onChange(nextStatus)
              void save(nextStatus)
            }}
            onBlur={() => {
              field.onBlur()
              if (!saveState.isSaving()) onCancel()
            }}
            onKeyDown={(event: KeyboardEvent<HTMLSelectElement>) => {
              if (event.key !== "Escape") return
              event.preventDefault()
              event.stopPropagation()
              onCancel()
            }}
          >
            {accountStatuses.map((value) => (
              <option key={value} value={value}>
                {userStatusLabel(value, t)}
              </option>
            ))}
          </NativeSelect>
        )}
      />
    </DetailEditRow>
  )
}
