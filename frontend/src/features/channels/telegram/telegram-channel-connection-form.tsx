/** Telegram 机器人 Token 测试和保存表单。 */
import { useMemo, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  isNotFoundApiError,
  isTelegramBotReuseConfirmationError,
  saveTelegramChannelConnection,
  testTelegramChannelConnection,
  type TelegramChannel,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import { FieldGroup } from "@/components/ui/field"
import { resolveChannelServerURL } from "@/features/channels/channel-server-url"
import {
  createTelegramChannelConnectionSchema,
  type TelegramChannelConnectionFormValues,
} from "@/features/channels/telegram/telegram-channel-connection-schema"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 编辑 Telegram 机器人连接。 */
export function TelegramChannelConnectionForm({
  channel,
  onUpdated,
  onSavingChange,
}: {
  channel: TelegramChannel
  onUpdated: (channel: TelegramChannel) => void
  onSavingChange: (saving: boolean) => void
}) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [pendingBotReuse, setPendingBotReuse] =
    useState<TelegramChannelConnectionFormValues | null>(null)
  const schema = useMemo(
    () =>
      createTelegramChannelConnectionSchema({
        tokenRequired: t("telegramConnection.validation.tokenRequired"),
        tokenTooLong: t("telegramConnection.validation.tokenTooLong"),
      }),
    [t],
  )
  const form = useForm<TelegramChannelConnectionFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { botToken: channel.connection.botToken },
  })

  /** 保存 Token、机器人信息和回调基础地址。 */
  async function save(
    values: TelegramChannelConnectionFormValues,
    confirmBotReuse = false,
  ) {
    if (saving) return
    setSaving(true)
    onSavingChange(true)
    try {
      const webhookBaseURL = await resolveChannelServerURL()
      const updated = await saveTelegramChannelConnection(channel.id, {
        botToken: values.botToken,
        webhookBaseURL,
        confirmBotReuse,
      })
      form.reset({ botToken: updated.connection.botToken })
      onUpdated(updated)
      toast.success(t("telegramConnection.saved"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        console.warn("Telegram 渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
        return
      }
      if (!confirmBotReuse && isTelegramBotReuseConfirmationError(error)) {
        setPendingBotReuse({ ...values })
        return
      }
      console.warn("保存 Telegram 连接失败", {
        channel_id: channel.id,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["botToken", "webhookBaseURL"])
          : t("telegramConnection.saveError"),
      )
    } finally {
      setSaving(false)
      onSavingChange(false)
    }
  }

  /** 确认复用 Bot 后重新提交保存。 */
  function confirmBotReuse() {
    if (!pendingBotReuse) return
    const values = pendingBotReuse
    setPendingBotReuse(null)
    void save(values, true)
  }

  /** 仅通过 getMe 测试当前草稿 Token。 */
  async function test(values: TelegramChannelConnectionFormValues) {
    if (testing) return
    setTesting(true)
    try {
      await testTelegramChannelConnection(channel.id, {
        botToken: values.botToken,
      })
      toast.success(t("telegramConnection.tested"))
    } catch (error) {
      if (recoverSession(error, navigate)) return
      if (isNotFoundApiError(error)) {
        console.warn("Telegram 渠道不存在", { channel_id: channel.id })
        navigate("/integrations/channels", { replace: true })
        return
      }
      console.warn("测试 Telegram 连接失败", {
        channel_id: channel.id,
        error,
      })
      toast.error(
        isApiError(error)
          ? apiErrorMessage(error, ["botToken"])
          : t("telegramConnection.testError"),
      )
    } finally {
      setTesting(false)
    }
  }

  return (
    <>
      <form
        className="w-full max-w-2xl space-y-9"
        onSubmit={form.handleSubmit((values) => save(values))}
        noValidate
      >
        <FieldGroup>
          <FormInputField
            name="botToken"
            control={form.control}
            label={t("telegramConnection.form.botToken")}
            autoFocus
            autoComplete="off"
            maxLength={512}
            passwordVisibilityLabels={{
              show: t("telegramConnection.form.showToken"),
              hide: t("telegramConnection.form.hideToken"),
            }}
          />
        </FieldGroup>
        <div className="flex items-center gap-2">
          <Button type="submit" disabled={saving || testing}>
            {saving ? <LoaderCircleIcon className="animate-spin" /> : null}
            {saving
              ? t("telegramConnection.form.saving")
              : t("telegramConnection.form.save")}
          </Button>
          <Button
            type="button"
            variant="outline"
            disabled={testing || saving}
            onClick={form.handleSubmit(test)}
          >
            {testing ? <LoaderCircleIcon className="animate-spin" /> : null}
            {testing
              ? t("telegramConnection.form.testing")
              : t("telegramConnection.form.test")}
          </Button>
          <Button type="button" variant="outline" asChild>
            <Link to="/integrations/channels">
              {t("telegramConnection.form.cancel")}
            </Link>
          </Button>
        </div>
      </form>
      <AlertDialog
        open={pendingBotReuse !== null}
        onOpenChange={(open) => {
          if (!open) setPendingBotReuse(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("telegramConnection.reuseConfirmation.title")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("telegramConnection.reuseConfirmation.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>
              {t("telegramConnection.reuseConfirmation.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              className="bg-primary text-primary-foreground hover:bg-primary/90"
              onClick={confirmBotReuse}
            >
              {t("telegramConnection.reuseConfirmation.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
