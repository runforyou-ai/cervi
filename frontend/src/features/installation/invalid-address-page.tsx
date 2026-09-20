/** 企业地址无效页。 */
import { useTranslation } from "react-i18next"

/** 展示当前访问地址没有对应企业的说明。 */
export function InvalidAddressPage() {
  const { t } = useTranslation("setup")

  return (
    <main className="flex min-h-svh w-full items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <p className="text-lg font-semibold tracking-tight">Cervi</p>
        </div>
        <div className="space-y-2 text-center">
          <h1 className="text-xl font-semibold tracking-tight">
            {t("addressInvalidTitle")}
          </h1>
          <p className="text-muted-foreground text-sm">
            {t("addressInvalidDescription")}
          </p>
        </div>
      </div>
    </main>
  )
}
