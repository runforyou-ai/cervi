/** 客户会话的客户语言入口：切换整段显示原文，锁定或恢复自动识别的回复语言。 */
import { LanguagesIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import { isApiError } from "@/api"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { apiErrorMessage } from "@/lib/form-errors"
import { languageDisplayName, translationLanguages } from "@/lib/languages"

import { useCustomerTranslation } from "./customer-translation"

/** 返回客户语言入口是否需要展示：客户语言未知或与本人语言不同，或客服锁定了回复语言。 */
export function useCustomerLanguageVisible() {
  const translation = useCustomerTranslation()
  return Boolean(translation && (translation.replyNeedsTranslation || translation.state.replyLanguageLocked))
}

/** 会话头标题旁的客户语言标签，点击打开显示原文与回复语言菜单。 */
export function CustomerLanguageChip() {
  const { t, i18n } = useTranslation("inbox")
  const translation = useCustomerTranslation()
  const visible = useCustomerLanguageVisible()
  if (!translation || !visible) return null
  const language = translation.state.customerLanguage
    ? languageDisplayName(translation.state.customerLanguage, i18n.language)
    : t("translationCustomerLanguageUnknown")
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 shrink-0 gap-1 px-1.5 text-xs font-normal text-muted-foreground"
              aria-label={t("translationCustomerLanguage", { language })}
            >
              <LanguagesIcon className="size-3.5" />
              {language}
            </Button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent>{t("translationCustomerLanguage", { language })}</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="start" className="min-w-52">
        <CustomerLanguageMenuItems />
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** 显示原文开关与回复语言子菜单，桌面端标签菜单与移动端会话菜单共用。 */
export function CustomerLanguageMenuItems({ itemClassName }: { itemClassName?: string }) {
  const { t, i18n } = useTranslation("inbox")
  const translation = useCustomerTranslation()
  if (!translation) return null
  const { state } = translation
  const locked = state.replyLanguageLocked
  const customerLanguage = languageDisplayName(state.customerLanguage, i18n.language)
  // 已锁定的语言不在常用列表中时补到列表首位。
  const options = locked && !translationLanguages.includes(state.customerLanguage)
    ? [state.customerLanguage, ...translationLanguages]
    : translationLanguages

  /** 保存回复语言，失败时提示。 */
  async function selectLanguage(language: string) {
    try {
      await translation?.setReplyLanguage(language)
    } catch (error) {
      console.warn("修改回复语言失败", error)
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("translationReplyLanguageError"))
    }
  }

  return (
    <>
      <DropdownMenuCheckboxItem
        className={itemClassName}
        checked={translation.showOriginal}
        onCheckedChange={translation.setShowOriginal}
      >
        {t("translationShowAllOriginal")}
      </DropdownMenuCheckboxItem>
      <DropdownMenuSeparator />
      <DropdownMenuSub>
        <DropdownMenuSubTrigger className={itemClassName}>
          <span className="min-w-0 flex-1 truncate">
            {t("translationReplyLanguage", { language: locked ? customerLanguage : t("translationReplyLanguageAuto") })}
          </span>
        </DropdownMenuSubTrigger>
        <DropdownMenuSubContent className="max-h-80 min-w-48">
          <DropdownMenuCheckboxItem
            className={itemClassName}
            checked={!locked}
            onCheckedChange={() => void selectLanguage("")}
          >
            {t("translationReplyLanguageAuto")}
          </DropdownMenuCheckboxItem>
          <DropdownMenuSeparator />
          {options.map((language) => (
            <DropdownMenuCheckboxItem
              key={language}
              className={itemClassName}
              checked={locked && language === state.customerLanguage}
              onCheckedChange={() => void selectLanguage(language)}
            >
              {languageDisplayName(language, i18n.language)}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuSubContent>
      </DropdownMenuSub>
    </>
  )
}
