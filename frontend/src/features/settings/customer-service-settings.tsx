/** 客服设置页签：工作时间与分配提醒，当前页签与地址同步。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router"

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { BusinessHoursSettings } from "@/features/settings/business-hours-form"
import { ServiceTimeoutsSettings } from "@/features/settings/service-timeouts-form"

/** 按地址中的页签显示工作时间或分配与提醒设置。 */
export function CustomerServiceSettings() {
  const { t } = useTranslation("settings")
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = searchParams.get("tab") === "assignment" ? "assignment" : "businessHours"

  // 缺省或无效页签统一写回地址，刷新时恢复同一页签。
  useEffect(() => {
    if (searchParams.get("tab") === tab) return
    const next = new URLSearchParams(searchParams)
    next.set("tab", tab)
    setSearchParams(next, { replace: true })
  }, [searchParams, setSearchParams, tab])

  return (
    <Tabs
      value={tab}
      onValueChange={(value) => {
        const next = new URLSearchParams(searchParams)
        next.set("tab", value)
        setSearchParams(next, { replace: true })
      }}
    >
      <TabsList>
        <TabsTrigger value="businessHours">
          {t("customerService.tabs.businessHours")}
        </TabsTrigger>
        <TabsTrigger value="assignment">
          {t("customerService.tabs.assignment")}
        </TabsTrigger>
      </TabsList>
      <TabsContent
        value="businessHours"
        forceMount
        className="mt-6 data-[state=inactive]:hidden"
      >
        <BusinessHoursSettings />
      </TabsContent>
      <TabsContent
        value="assignment"
        forceMount
        className="mt-6 data-[state=inactive]:hidden"
      >
        <ServiceTimeoutsSettings />
      </TabsContent>
    </Tabs>
  )
}
