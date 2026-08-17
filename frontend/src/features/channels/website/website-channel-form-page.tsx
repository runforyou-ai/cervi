import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"

import { getWebsiteChannel, type WebsiteChannel } from "@/api/channels"
import { ApiError } from "@/api/client"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { WebsiteChannelForm } from "@/features/channels/website/website-channel-form"
import { WebsiteChannelChatInterfaceForm } from "@/features/channels/website/website-channel-chat-interface-form"

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
  const activeTab = requestedTab === "chat-interface" ? "chat-interface" : "basic"

  useEffect(() => {
    if (requestedTab === "basic" || requestedTab === "chat-interface") {
      return
    }
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", "basic")
    setSearchParams(nextParams, { replace: true })
  }, [requestedTab, searchParams, setSearchParams])

  return (
    <Tabs
      className="mt-6"
      value={activeTab}
      onValueChange={(value) => {
        const nextParams = new URLSearchParams(searchParams)
        nextParams.set("tab", value)
        setSearchParams(nextParams, { replace: true })
      }}
    >
      <TabsList>
        <TabsTrigger value="basic">{t("tabs.basic")}</TabsTrigger>
        <TabsTrigger value="chat-interface">
          {t("tabs.chatInterface")}
        </TabsTrigger>
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
    </Tabs>
  )
}

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
            navigate("/channels/website", { replace: true })
            return
          }
        }
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
      <Breadcrumb className="mb-6">
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link to="/channels/website">{t("list.title")}</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>
              {mode === "create" ? t("create.title") : channel?.name}
            </BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>
      <h2 className="text-xl font-semibold tracking-tight">
        {mode === "create" ? t("create.title") : channel?.name}
      </h2>
      {mode === "edit" && channel ? (
        <WebsiteChannelEditTabs channel={channel} onChannelChange={setChannel} />
      ) : (
        <WebsiteChannelForm />
      )}
    </div>
  )
}
