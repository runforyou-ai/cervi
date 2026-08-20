/** 网站渠道创建页和编辑页。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router"

import { ApiError, getWebsiteChannel, type WebsiteChannel } from "@/api"
import { PageBackHeader } from "@/components/page-back-header"
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

function isEditTab(value: string | null): value is EditTab {
  return editTabs.some((tab) => tab === value)
}

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

  function setTab(value: string) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", value)
    if (value === "usage" && !isAccessTab(nextParams.get("access"))) {
      nextParams.set("access", "embed")
    }
    setSearchParams(nextParams, { replace: true })
  }

  function setAccess(value: WebsiteChannelAccess) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", "usage")
    nextParams.set("access", value)
    setSearchParams(nextParams, { replace: true })
  }

  return (
    <Tabs className="mt-6" value={activeTab} onValueChange={setTab}>
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
        className="data-[state=inactive]:hidden"
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
        className="data-[state=inactive]:hidden"
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
        className="data-[state=inactive]:hidden"
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
        if (requestError instanceof ApiError) {
          if (requestError.code === "AUTH_REQUIRED") {
            navigate("/login", { replace: true })
            return
          }
          if (requestError.code === "CHANNEL_NOT_FOUND") {
            console.warn("网站渠道不存在", { channel_id: channelId })
            navigate("/channels/website", { replace: true })
            return
          }
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

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </div>
    )
  }

  if (mode === "edit" && !channel) {
    return (
      <div className="flex flex-1 items-center justify-center p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">
            {error || t("form.loadError")}
          </p>
          <Button
            className="mt-4"
            variant="outline"
            onClick={() => setReloadKey((current) => current + 1)}
          >
            {t("retry")}
          </Button>
        </div>
      </div>
    )
  }

  return (
    <div className="w-full px-4 py-6 sm:px-6 lg:px-8">
      <PageBackHeader
        to="/channels/website"
        title={mode === "create" ? t("create.title") : channel?.name}
      />
      {mode === "edit" && channel ? (
        <WebsiteChannelEditTabs channel={channel} onChannelChange={setChannel} />
      ) : (
        <WebsiteChannelForm />
      )}
    </div>
  )
}
