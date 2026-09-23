/** 消息渠道创建页和按类型扩展的编辑页。 */
import { useEffect, useState } from "react"
import { useQueryClient } from "@tanstack/react-query"
import { useTranslation } from "react-i18next"
import {
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router"

import {
  ChannelType,
  getMessageChannel,
  getTelegramChannel,
  getWebsiteChannel,
  isNotFoundApiError,
  TelegramWebhookStatus,
  type MessageChannelSummary,
  type TelegramChannel,
  type WebsiteChannelChatInterfaceInput,
  type WebsiteChannelData,
} from "@/api"
import { PageBackButton } from "@/components/page-back-button"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ChannelReceptionSettingsForm } from "@/features/channels/channel-reception-settings-form"
import { MessageChannelForm } from "@/features/channels/message-channel-form"
import {
  isMessageChannelType,
  messageChannelTypeDefinition,
} from "@/lib/message-channel-types"
import { TelegramChannelConnectionForm } from "@/features/channels/telegram/telegram-channel-connection-form"
import { TelegramChannelInfoPanel } from "@/features/channels/telegram/telegram-channel-info-panel"
import { WebsiteChannelChatInterfaceForm } from "@/features/channels/website/website-channel-chat-interface-form"
import {
  WebsiteChannelUsagePanel,
  type WebsiteChannelAccessTab,
} from "@/features/channels/website/website-channel-usage-panel"
import { WebsiteChatPreview } from "@/features/channels/website/website-chat-preview"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

type EditTab = "basic" | "reception" | "chat-interface" | "usage" | "connection"
type EditableChannel =
  | MessageChannelSummary
  | WebsiteChannelData
  | TelegramChannel

/** 各编辑页签的标题文案键。 */
const editTabLabelKeys = {
  basic: "tabs.basic",
  reception: "tabs.reception",
  "chat-interface": "tabs.chatInterface",
  usage: "tabs.usage",
  connection: "tabs.connection",
} as const satisfies Record<EditTab, string>

/** 按渠道类型给出编辑页签和详情读取方式，未列出的类型只有通用页签。 */
const channelEditConfigs: Partial<
  Record<
    ChannelType,
    {
      tabs: readonly EditTab[]
      load: (id: string, signal: AbortSignal) => Promise<EditableChannel>
    }
  >
> = {
  [ChannelType.ChannelTypeWebsite]: {
    tabs: ["basic", "reception", "chat-interface", "usage"],
    load: getWebsiteChannel,
  },
  [ChannelType.ChannelTypeTelegram]: {
    tabs: ["basic", "reception", "connection"],
    load: getTelegramChannel,
  },
}

/** 返回渠道类型的编辑配置。 */
function channelEditConfig(type: string) {
  return (
    channelEditConfigs[type as ChannelType] ?? {
      tabs: ["basic", "reception"] as const,
      load: getMessageChannel,
    }
  )
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

/** 判断详情是否包含 Telegram 渠道扩展。 */
function isTelegramChannel(
  channel: EditableChannel,
): channel is TelegramChannel {
  return (
    channel.type === ChannelType.ChannelTypeTelegram &&
    "connection" in channel
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
  onConnectionSavingChange,
}: {
  channel: EditableChannel
  onChannelChange: (channel: EditableChannel) => void
  onConnectionSavingChange: (saving: boolean) => void
}) {
  const { t } = useTranslation("channels")
  const [searchParams, setSearchParams] = useSearchParams()
  const websiteChannel = isWebsiteChannelData(channel) ? channel : null
  const telegramChannel = isTelegramChannel(channel) ? channel : null
  const { tabs } = channelEditConfig(channel.type)
  const requestedTab = searchParams.get("tab")
  const requestedAccess = searchParams.get("access")
  const tabValid = tabs.some((tab) => tab === requestedTab)
  const activeTab = tabValid ? (requestedTab as EditTab) : "basic"
  const activeAccess: WebsiteChannelAccessTab =
    requestedAccess === "link" ? "link" : "embed"
  const [previewValue, setPreviewValue] =
    useState<WebsiteChannelChatInterfaceInput | null>(() =>
      websiteChannel ? savedPreviewValue(websiteChannel) : null,
    )

  useEffect(() => {
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
    searchParams,
    setSearchParams,
    tabValid,
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
    onChannelChange({ ...channel, ...updated })
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
      {telegramChannel ? (
        <TabsContent
          value="connection"
          forceMount
          className="data-[state=inactive]:hidden"
        >
          <TelegramChannelConnectionForm
            channel={telegramChannel}
            onUpdated={onChannelChange}
            onSavingChange={onConnectionSavingChange}
          />
        </TabsContent>
      ) : null}
    </div>
  )

  // 右侧面板：Telegram 显示接入信息，网站渠道显示聊天界面实时预览。
  const aside = telegramChannel ? (
    <TelegramChannelInfoPanel channel={telegramChannel} />
  ) : websiteChannel && previewValue ? (
    <WebsiteChatPreview value={previewValue} />
  ) : null

  return (
    <Tabs value={activeTab} onValueChange={setTab}>
      <TabsList>
        {tabs.map((tab) => (
          <TabsTrigger key={tab} value={tab}>
            {t(editTabLabelKeys[tab])}
          </TabsTrigger>
        ))}
      </TabsList>
      {aside ? (
        <div className="mt-6 grid gap-8 xl:grid-cols-[minmax(0,1fr)_360px]">
          {content}
          {aside}
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
  const { t } = useTranslation(["channels", "common"])
  const navigate = useNavigate()
  const { channelId = "", channelType = "" } = useParams()
  const [pageSearchParams] = useSearchParams()
  // 来源列表的状态筛选，返回时带回。
  const listStatus = pageSearchParams.get("status")
  const [telegramConnectionSaving, setTelegramConnectionSaving] =
    useState(false)
  const editable = mode === "edit" && isMessageChannelType(channelType)
  const detailKey = resourceKeys.messageChannel(channelType, channelId)
  const detail = useResource<EditableChannel>(
    detailKey,
    (signal) => channelEditConfig(channelType).load(channelId, signal),
    {
      enabled: editable,
      // 等待 Telegram 回调时低频刷新已保存的 Webhook 状态。
      refetchInterval: (data) =>
        data &&
        isTelegramChannel(data) &&
        data.connection.webhookStatus ===
          TelegramWebhookStatus.TelegramWebhookStatusWaiting &&
        !telegramConnectionSaving
          ? 8_000
          : false,
    },
  )
  // 地址中的类型与详情一致时才展示，类型不一致时等待纠正地址。
  const channel =
    detail.data && detail.data.type === channelType ? detail.data : null
  const queryClient = useQueryClient()
  const invalidateResource = useResourceInvalidator()

  /** 拦截无效的渠道类型参数，回到渠道列表。 */
  useEffect(() => {
    if (!isMessageChannelType(channelType)) {
      navigate("/channels", { replace: true })
    }
  }, [channelType, navigate])

  /** 详情类型与地址不一致时校正地址。 */
  const loadedChannel = detail.data
  useEffect(() => {
    if (!loadedChannel || loadedChannel.type === channelType) return
    navigate(`/channels/${loadedChannel.type}/${loadedChannel.id}`, {
      replace: true,
    })
  }, [channelType, loadedChannel, navigate])

  /** 渠道不存在时回到渠道列表。 */
  useEffect(() => {
    if (detail.error && isNotFoundApiError(detail.error)) {
      console.warn("消息渠道不存在", {
        channel_id: channelId,
        channel_type: channelType,
      })
      navigate("/channels", { replace: true })
    }
  }, [channelId, channelType, detail.error, navigate])

  /** 子表单保存后写入详情缓存并重新读取。 */
  function handleChannelChange(next: EditableChannel) {
    // 各页签按渠道详情提交完整记录，保存结果立即写入缓存，其他页签随即使用最新值。
    queryClient.setQueryData(detailKey, next)
    void invalidateResource(detailKey)
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
            ? t("create.title", { type: typeLabel })
            : channel && typeLabel
              ? t("edit.namedTitle", { type: typeLabel, name: channel.name })
              : editTitle
        }
        description={t(
          mode === "create" ? "create.description" : "edit.description",
        )}
      >
        {mode === "edit" ? (
          <PageBackButton
            to={`/channels${listStatus === "disabled" ? "?status=disabled" : ""}`}
          />
        ) : null}
      </PageHeader>
      <PageContent variant="form">
        {mode === "edit" ? (
          <ResourceContent
            resources={editable ? detail : []}
            errorMessage={t("form.loadError")}
          >
            {channel ? (
              <MessageChannelEditTabs
                key={channel.id}
                channel={channel}
                onChannelChange={handleChannelChange}
                onConnectionSavingChange={setTelegramConnectionSaving}
              />
            ) : null}
          </ResourceContent>
        ) : isMessageChannelType(channelType) ? (
          <MessageChannelForm type={channelType} />
        ) : null}
      </PageContent>
    </div>
  )
}
