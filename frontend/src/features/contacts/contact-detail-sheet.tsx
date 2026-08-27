/** 通讯录详情侧栏的共享框架。 */
import { useRef, type ReactNode } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

/** 渲染详情侧栏的标题、加载态和内容区。 */
export function ContactDetailSheet({
  open,
  onClose,
  title,
  description,
  loading,
  children,
}: {
  open: boolean
  onClose: () => void
  title: ReactNode
  description: ReactNode
  loading: boolean
  children: ReactNode
}) {
  const { t } = useTranslation("contacts")
  const detailTitleRef = useRef<HTMLHeadingElement>(null)

  return (
    <Sheet open={open} onOpenChange={(next) => !next && onClose()}>
      <SheetContent
        className="w-full gap-0 p-0 sm:max-w-xl"
        onOpenAutoFocus={(event) => {
          event.preventDefault()
          detailTitleRef.current?.focus()
        }}
      >
        <SheetHeader className="border-b px-6 py-4 pr-12">
          <SheetTitle
            ref={detailTitleRef}
            tabIndex={-1}
            className="outline-none"
          >
            {title}
          </SheetTitle>
          <SheetDescription>{description}</SheetDescription>
        </SheetHeader>
        <ScrollArea className="min-h-0 flex-1">
          <div className="p-6">
            {loading ? (
              <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
                <LoaderCircleIcon className="size-4 animate-spin" />
                {t("loading")}
              </div>
            ) : (
              children
            )}
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}
