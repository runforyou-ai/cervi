/** 网站渠道创建页和编辑页。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router"

import {
  getWebsiteChannel,
  isNotFoundApiError,
  type WebsiteChannel,
} from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { WebsiteChannelForm } from "@/features/channels/website/website-channel-form"
import { WebsiteChannelChatInterfaceForm } from "@/features/channels/website/website-channel-chat-interface-form"
import {
  WebsiteChannelUsagePanel,
  type WebsiteChannelAccess,
} from "@/features/channels/website/website-channel-usage-panel"

const editTabs = ["basic", "chat-interface", "usage"] as const

type EditTab = (typeof editTabs)[number]

/** 判断值是否为渠道编辑页签。 */
function isEditTab(value: string | null): value is EditTab {
  return editTabs.some((tab) => tab === value)
}

/** 判断值是否为渠道访问方式页签。 */
function isAccessTab(value: string | null): value is WebsiteChannelAccess {
  return value === "embed" || value === "link"
}

/** 网站渠道编辑页签，与 URL 同步。 */
function WebsiteChannelEditTabs({
  channel,
  onChannelChange,
}: {
  channel: WebsiteChannel
  onChannelChange: (channel: WebsiteChannel) => void
}) {
  const { t } = useTranslation("channels")
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedTab = searchParams.get("tab")
  const requestedAccess = searchParams.get("access")
  const activeTab = isEditTab(requestedTab) ? requestedTab : "basic"
  const activeAccess: WebsiteChannelAccess =
    requestedAccess === "link" ? "link" : "embed"

  useEffect(() => {
    const tabValid = isEditTab(requestedTab)
    const accessValid = isAccessTab(requestedAccess)
    if (tabValid && (activeTab !== "usage" || accessValid)) {
      return
    }
    const nextParams = new URLSearchParams(searchParams)
    if (!tabValid) {
      nextParams.set("tab", "basic")
    }
    if (activeTab === "usage" && !accessValid) {
      nextParams.set("access", "embed")
    }
    setSearchParams(nextParams, { replace: true })
  }, [activeTab, requestedAccess, requestedTab, searchParams, setSearchParams])

  /** 切换渠道编辑页签并同步 URL。 */
  function setTab(value: string) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", value)
    if (value === "usage" && !isAccessTab(nextParams.get("access"))) {
      nextParams.set("access", "embed")
    }
    setSearchParams(nextParams, { replace: true })
  }

  /** 切换渠道访问方式并同步 URL。 */
  function setAccess(value: WebsiteChannelAccess) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", "usage")
    nextParams.set("access", value)
    setSearchParams(nextParams, { replace: true })
  }

  return (
    <Tabs value={activeTab} onValueChange={setTab}>
      <TabsList>
        <TabsTrigger value="basic">{t("tabs.basic")}</TabsTrigger>
        <TabsTrigger value="chat-interface">
          {t("tabs.chatInterface")}
        </TabsTrigger>
        <TabsTrigger value="usage">{t("tabs.usage")}</TabsTrigger>
      </TabsList>
      <TabsContent
        value="basic"
        forceMount
        className="mt-6 data-[state=inactive]:hidden"
      >
        <WebsiteChannelForm
          channel={channel}
          onUpdated={(updated) =>
            onChannelChange({ ...channel, ...updated })
          }
        />
      </TabsContent>
      <TabsContent
        value="chat-interface"
        forceMount
        className="mt-6 data-[state=inactive]:hidden"
      >
        <WebsiteChannelChatInterfaceForm
          channel={channel}
          onUpdated={(chatInterface) =>
            onChannelChange({ ...channel, chatInterface })
          }
        />
      </TabsContent>
      <TabsContent
        value="usage"
        forceMount
        className="mt-6 data-[state=inactive]:hidden"
      >
        <WebsiteChannelUsagePanel
          channelId={channel.id}
          access={activeAccess}
          onAccessChange={setAccess}
        />
      </TabsContent>
    </Tabs>
  )
}

/** 创建或编辑网站渠道。 */
export function WebsiteChannelFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const { channelId = "" } = useParams()
  const [channel, setChannel] = useState<WebsiteChannel | null>(null)
  const [loading, setLoading] = useState(mode === "edit")
  const [error, setError] = useState("")
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    if (mode !== "edit") {
      return
    }

    let active = true
    setLoading(true)
    setError("")
    void getWebsiteChannel(channelId)
      .then((loadedChannel) => {
        if (active) {
          setChannel(loadedChannel)
        }
      })
      .catch((requestError: unknown) => {
        if (!active) {
          return
        }
        if (recoverSession(requestError, navigate)) {
          return
        }
        if (isNotFoundApiError(requestError)) {
          console.warn("网站渠道不存在", { channel_id: channelId })
          navigate("/integrations/channels", { replace: true })
          return
        }
        console.warn("网站渠道详情加载失败", requestError)
        setError(t("form.loadError"))
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [channelId, mode, navigate, reloadKey, t])

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
      <PageHeader
        title={
          mode === "create"
            ? t("create.title")
            : channel?.name ?? t("edit.title")
        }
      />
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("loading")}
          </div>
        ) : mode === "edit" && !channel ? (
          <div className="flex min-h-48 items-center justify-center text-center">
            <div>
              <p className="text-sm text-muted-foreground">{error}</p>
              <Button
                className="mt-4"
                variant="outline"
                onClick={() => setReloadKey((current) => current + 1)}
              >
                {t("retry")}
              </Button>
            </div>
          </div>
        ) : mode === "edit" && channel ? (
          <WebsiteChannelEditTabs
            channel={channel}
            onChannelChange={setChannel}
          />
        ) : (
          <WebsiteChannelForm />
        )}
      </PageContent>
    </div>
  )
}
