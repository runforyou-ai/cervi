import { SmartphoneIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

export default function MobileApp() {
  const { t } = useTranslation("mobile")

  return (
    <main className="flex min-h-dvh items-center justify-center px-6 pt-[max(1.5rem,env(safe-area-inset-top))] pb-[max(1.5rem,env(safe-area-inset-bottom))]">
      <section className="max-w-sm text-center">
        <div className="mx-auto mb-5 flex size-12 items-center justify-center rounded-2xl border bg-background shadow-sm">
          <SmartphoneIcon className="size-5 text-muted-foreground" />
        </div>
        <p className="mb-2 text-sm font-semibold tracking-wide">Cervi · 鹿行</p>
        <h1 className="text-xl font-semibold tracking-tight">{t("title")}</h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          {t("description")}
        </p>
      </section>
    </main>
  )
}
