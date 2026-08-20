/** 加载收件箱数据并交给收件箱页面展示。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { ApiError, loadInbox, type InboxData } from "@/api"
import { Button } from "@/components/ui/button"
import { InboxPage } from "@/features/inbox/inbox-page"

/** 读取收件箱并处理未登录跳转。 */
export function InboxRoute() {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [data, setData] = useState<InboxData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  /** 加载当前用户的统一收件箱。 */
  const fetchInbox = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      setData(await loadInbox())
    } catch (requestError) {
      if (requestError instanceof ApiError && requestError.code === "AUTH_REQUIRED") {
        navigate("/login", { replace: true })
        return
      }
      setError(t("inboxLoadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void fetchInbox()
  }, [fetchInbox])

  if (loading && !data) {
    return (
      <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </div>
    )
  }

  if (!data) {
    return (
      <div className="flex flex-1 items-center justify-center p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">
            {error || t("inboxLoadError")}
          </p>
          <Button className="mt-4" variant="outline" onClick={fetchInbox}>
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </div>
      </div>
    )
  }

  return <InboxPage conversations={data.conversations} />
}
