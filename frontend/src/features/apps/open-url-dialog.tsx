/** 输入自定义网址并按平台打开的弹窗。 */
import { useEffect, useMemo, useRef } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { z } from "zod"

import { isApiError } from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { FieldGroup } from "@/components/ui/field"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { openExternalPage } from "@/platform/external-navigation"

const urlMaxLength = 2048

/** 校验地址为不含认证信息的完整 HTTP 或 HTTPS 地址。 */
function validExternalPageUrl(value: string) {
  let parsed: URL
  try {
    parsed = new URL(value)
  } catch {
    return false
  }
  return (
    (parsed.protocol === "http:" || parsed.protocol === "https:") &&
    parsed.host !== "" &&
    parsed.username === "" &&
    parsed.password === ""
  )
}

/** 输入网址后在应用内新窗口或新浏览器标签中打开。 */
export function OpenUrlDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("apps")
  const navigate = useNavigate()
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      z.object({
        url: z
          .string()
          .trim()
          .max(urlMaxLength, t("openUrl.invalid"))
          .refine(validExternalPageUrl, t("openUrl.invalid")),
      }),
    [t],
  )
  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { url: "" },
  })

  useEffect(() => {
    mounted.current = true
    if (open) {
      form.reset({ url: "" })
    }
    return () => {
      mounted.current = false
    }
  }, [form, open])

  /** 打开输入的网址并关闭弹窗。 */
  async function submit(values: z.infer<typeof schema>) {
    try {
      await openExternalPage({
        title: new URL(values.url).host,
        url: values.url,
      })
      if (!mounted.current) return
      onOpenChange(false)
      console.info("自定义网址已打开", { url: values.url })
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) return
      console.warn("自定义网址打开失败", { error })
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("openUrl.error"),
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent aria-describedby={undefined}>
        <DialogHeader>
          <DialogTitle>{t("openUrl.title")}</DialogTitle>
        </DialogHeader>
        <form
          className="grid gap-9"
          onSubmit={form.handleSubmit(submit)}
          noValidate
        >
          <FieldGroup>
            <FormInputField
              name="url"
              control={form.control}
              label={t("openUrl.label")}
              autoFocus
              type="url"
              maxLength={urlMaxLength}
            />
          </FieldGroup>
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={form.formState.isSubmitting}
              onClick={() => onOpenChange(false)}
            >
              {t("openUrl.cancel")}
            </Button>
            <Button type="submit" disabled={form.formState.isSubmitting}>
              {form.formState.isSubmitting
                ? t("openUrl.opening")
                : t("openUrl.open")}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  )
}
