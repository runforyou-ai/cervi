/** 消息渠道创建页和按类型扩展的编辑页。 */
import { useEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Link,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"

import {
  ChannelType,
  getMessageChannel,
  getWebsiteChannel,
  isNotFoundApiError,
  type MessageChannelSummary,
  type WebsiteChannelData,
  type WebsiteChannelChatInterfaceInput,
} from "@/api"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ChannelReceptionSettingsForm } from "@/features/channels/channel-reception-settings-form"
import { MessageChannelForm } from "@/features/channels/message-channel-form"
import {
  isMessageChannelType,
  messageChannelTypeDefinition,
} from "@/features/channels/message-channel-types"
import { WebsiteChannelChatInterfaceForm } from "@/features/channels/website/website-channel-chat-interface-form"
import {
  WebsiteChannelUsagePanel,
  type WebsiteChannelAccessTab,
} from "@/features/channels/website/website-channel-usage-panel"
import { WebsiteChatPreview } from "@/features/channels/website/website-chat-preview"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

const baseEditTabs = ["basic", "reception"] as const
const websiteEditTabs = [...baseEditTabs, "chat-interface", "usage"] as const

type EditTab = (typeof websiteEditTabs)[number]
type EditableChannel = MessageChannelSummary | WebsiteChannelData

/** 判断值是否为当前渠道支持的编辑页签。 */
function isEditTab(
  value: string | null,
  website: boolean,
): value is EditTab {
  const tabs: readonly string[] = website ? websiteEditTabs : baseEditTabs
  return value !== null && tabs.includes(value)
}

/** 判断值是否为渠道访问方式页签。 */
function isAccessTab(value: string | null): value is WebsiteChannelAccessTab {
  return value === "embed" || value === "link"
}

/** 判断详情是否包含网站渠道扩展。 */
function isWebsiteChannelData(
  channel: EditableChannel,
): channel is WebsiteChannelData {
  return (
    channel.type === ChannelType.ChannelTypeWebsite &&
    "chatInterface" in channel &&
    "access" in channel
  )
}

/** 把已保存的聊天界面设置归一化为实时预览值。 */
function savedPreviewValue(
  channel: WebsiteChannelData,
): WebsiteChannelChatInterfaceInput {
  return {
    title: channel.chatInterface.title,
    subtitle: channel.chatInterface.subtitle ?? "",
    greetingMessage: channel.chatInterface.greetingMessage ?? "",
    themeColor: channel.chatInterface.themeColor,
  }
}

/** 渠道编辑页签，与 URL 同步并按类型展示扩展设置。 */
function MessageChannelEditTabs({
  channel,
  onChannelChange,
}: {
  channel: EditableChannel
  onChannelChange: (channel: EditableChannel) => void
}) {
  const { t } = useTranslation("channels")
  const [searchParams, setSearchParams] = useSearchParams()
  const websiteChannel = isWebsiteChannelData(channel) ? channel : null
  const requestedTab = searchParams.get("tab")
  const requestedAccess = searchParams.get("access")
  const activeTab = isEditTab(requestedTab, websiteChannel !== null)
    ? requestedTab
    : "basic"
  const activeAccess: WebsiteChannelAccessTab =
    requestedAccess === "link" ? "link" : "embed"
  const [previewValue, setPreviewValue] =
    useState<WebsiteChannelChatInterfaceInput | null>(() =>
      websiteChannel ? savedPreviewValue(websiteChannel) : null,
    )

  useEffect(() => {
    const tabValid = isEditTab(requestedTab, websiteChannel !== null)
    const accessValid = isAccessTab(requestedAccess)
    if (
      tabValid &&
      (activeTab !== "usage" || accessValid) &&
      (websiteChannel || requestedAccess === null)
    ) {
      return
    }
    const nextParams = new URLSearchParams(searchParams)
    if (!tabValid) {
      nextParams.set("tab", "basic")
    }
    if (websiteChannel && activeTab === "usage" && !accessValid) {
      nextParams.set("access", "embed")
    }
    if (!websiteChannel) {
      nextParams.delete("access")
    }
    setSearchParams(nextParams, { replace: true })
  }, [
    activeTab,
    requestedAccess,
    requestedTab,
    searchParams,
    setSearchParams,
    websiteChannel,
  ])

  /** 切换渠道编辑页签并同步 URL。 */
  function setTab(value: string) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", value)
    if (value === "usage" && !isAccessTab(nextParams.get("access"))) {
      nextParams.set("access", "embed")
    }
    setSearchParams(nextParams, { replace: true })
  }

  /** 切换网站渠道访问方式并同步 URL。 */
  function setAccess(value: WebsiteChannelAccessTab) {
    const nextParams = new URLSearchParams(searchParams)
    nextParams.set("tab", "usage")
    nextParams.set("access", value)
    setSearchParams(nextParams, { replace: true })
  }

  /** 合并通用渠道基础信息更新。 */
  function mergeSummary(updated: MessageChannelSummary) {
    onChannelChange(
      websiteChannel ? { ...websiteChannel, ...updated } : updated,
    )
  }

  const content = (
    <div className="min-w-0">
      <TabsContent
        value="basic"
        forceMount
        className="data-[state=inactive]:hidden"
      >
        <MessageChannelForm channel={channel} onUpdated={mergeSummary} />
      </TabsContent>
      <TabsContent
        value="reception"
        forceMount
        className="data-[state=inactive]:hidden"
      >
        <ChannelReceptionSettingsForm
          channel={channel}
          onUpdated={mergeSummary}
        />
      </TabsContent>
      {websiteChannel ? (
        <>
          <TabsContent
            value="chat-interface"
            forceMount
            className="data-[state=inactive]:hidden"
          >
            <WebsiteChannelChatInterfaceForm
              channel={websiteChannel}
              onPreviewChange={setPreviewValue}
              onUpdated={(chatInterface) =>
                onChannelChange({ ...websiteChannel, chatInterface })
              }
            />
          </TabsContent>
          <TabsContent
            value="usage"
            forceMount
            className="data-[state=inactive]:hidden"
          >
            <WebsiteChannelUsagePanel
              channel={websiteChannel}
              access={activeAccess}
              onAccessChange={setAccess}
              onUpdated={(access) =>
                onChannelChange({ ...websiteChannel, access })
              }
            />
          </TabsContent>
        </>
      ) : null}
    </div>
  )

  return (
    <Tabs
      value={activeTab}
      onValueChange={setTab}
      className={websiteChannel ? "max-w-[1240px]" : "max-w-2xl"}
    >
      <TabsList>
        <TabsTrigger value="basic">{t("tabs.basic")}</TabsTrigger>
        <TabsTrigger value="reception">{t("tabs.reception")}</TabsTrigger>
        {websiteChannel ? (
          <>
            <TabsTrigger value="chat-interface">
              {t("tabs.chatInterface")}
            </TabsTrigger>
            <TabsTrigger value="usage">{t("tabs.usage")}</TabsTrigger>
          </>
        ) : null}
      </TabsList>
      {websiteChannel && previewValue ? (
        <div className="mt-6 grid gap-8 xl:grid-cols-[minmax(0,1fr)_480px]">
          {content}
          <WebsiteChatPreview value={previewValue} />
        </div>
      ) : (
        <div className="mt-6">{content}</div>
      )}
    </Tabs>
  )
}

/** 创建或编辑消息渠道。 */
export function MessageChannelFormPage({
  mode,
}: {
  mode: "create" | "edit"
}) {
  const { t } = useTranslation("channels")
  const navigate = useNavigate()
  const { channelId = "", channelType = "" } = useParams()
  const [channel, setChannel] = useState<EditableChannel | null>(null)
  const editable = mode === "edit" && isMessageChannelType(channelType)
  const {
    data: loadedChannel,
    loading: detailLoading,
    refreshing: detailRefreshing,
    error: detailError,
    refresh,
  } = useResource<EditableChannel>(
    resourceKeys.messageChannel(channelType, channelId),
    () =>
      channelType === ChannelType.ChannelTypeWebsite
        ? getWebsiteChannel(channelId)
        : getMessageChannel(channelId),
    { enabled: editable },
  )

  /** 编辑模式下拦截无效的渠道类型参数。 */
  useEffect(() => {
    if (mode === "edit" && !isMessageChannelType(channelType)) {
      navigate("/integrations/channels", { replace: true })
    }
  }, [channelType, mode, navigate])

  /** 详情就绪后校正地址中的渠道类型并同步编辑状态。 */
  useEffect(() => {
    if (!loadedChannel) {
      return
    }
    if (loadedChannel.type !== channelType) {
      navigate(
        `/integrations/channels/${loadedChannel.type}/${loadedChannel.id}`,
        { replace: true },
      )
      return
    }
    setChannel(loadedChannel)
  }, [channelType, loadedChannel, navigate])

  /** 渠道不存在时回到渠道列表。 */
  useEffect(() => {
    if (detailError && isNotFoundApiError(detailError)) {
      console.warn("消息渠道不存在", {
        channel_id: channelId,
        channel_type: channelType,
      })
      navigate("/integrations/channels", { replace: true })
    }
  }, [channelId, channelType, detailError, navigate])

  const loading =
    mode === "edit" &&
    (detailLoading ||
      (!channel && !detailError) ||
      (Boolean(detailError) && detailRefreshing))
  const invalidateResource = useResourceInvalidator()

  /** 子表单保存后同步本地渠道并失效详情缓存。 */
  function handleChannelChange(next: EditableChannel) {
    setChannel(next)
    void invalidateResource(resourceKeys.messageChannel(channelType, channelId))
  }

  const typeDefinition = channel
    ? messageChannelTypeDefinition(channel.type)
    : isMessageChannelType(channelType)
      ? messageChannelTypeDefinition(channelType)
      : undefined
  const typeLabel = typeDefinition
    ? t(`types.${typeDefinition.translationKey}`)
    : ""
  const editTitle = typeLabel
    ? t("edit.title", { type: typeLabel })
    : t("edit.fallbackTitle")

  return (
    <div className="flex min-h-0 w-full flex-1 flex-col overflow-hidden">
      <PageHeader
        title={
          mode === "create"
            ? t("create.title")
            : channel && typeLabel
              ? t("edit.namedTitle", { type: typeLabel, name: channel.name })
              : editTitle
        }
      >
        <Button variant="ghost" size="sm" asChild>
          <Link to="/integrations/channels">{t("form.back")}</Link>
        </Button>
      </PageHeader>
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("loading")}
          </div>
        ) : mode === "edit" && !channel ? (
          <div className="flex min-h-48 items-center justify-center text-center">
            <div>
              <p className="text-sm text-muted-foreground">
                {t("form.loadError")}
              </p>
              <Button
                className="mt-4"
                variant="outline"
                onClick={() => void refresh()}
              >
                {t("retry")}
              </Button>
            </div>
          </div>
        ) : mode === "edit" && channel ? (
          <MessageChannelEditTabs
            key={channel.id}
            channel={channel}
            onChannelChange={handleChannelChange}
          />
        ) : (
          <MessageChannelForm />
        )}
      </PageContent>
    </div>
  )
}
