/** 当前列表范围的筛选浮层。 */
import { ListFilterIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ConversationType,
  InboxScope,
  ServiceSessionStatus,
  type InboxChannel,
} from "@/api"
import { Button } from "@/components/ui/button"
import { NativeSelect } from "@/components/ui/native-select"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import {
  inboxKindOptionsForScope,
  toggleInboxKinds,
} from "@/features/inbox/inbox-query"

type InboxFilterValue = {
  channelId: string
  serviceStatus: ServiceSessionStatus
  kinds: ConversationType[]
}

/** 判断当前筛选是否偏离默认值。 */
function inboxFilterApplied(value: InboxFilterValue) {
  return (
    value.channelId !== "" ||
    value.serviceStatus === ServiceSessionStatus.ServiceSessionStatusClosed ||
    value.kinds.length > 0
  )
}

/** 按当前范围提供渠道、服务状态或会话类型筛选。 */
export function InboxFilter({
  scope,
  value,
  channels,
  onChange,
}: {
  scope: InboxScope
  value: InboxFilterValue
  channels: InboxChannel[]
  onChange: (changes: Partial<InboxFilterValue>) => void
}) {
  const { t } = useTranslation("inbox")
  const applied = inboxFilterApplied(value)
  const options = inboxKindOptionsForScope(scope)

  return (
    <Popover>
      {/* 触发按钮宽度固定，已设置条件用角标表示。 */}
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="relative shrink-0 text-muted-foreground"
          aria-label={applied ? t("filterApplied") : t("filterLabel")}
          title={t("filterLabel")}
        >
          <ListFilterIcon className="size-5" />
          {applied ? (
            <span
              aria-hidden="true"
              className="absolute top-1.5 right-1.5 size-1.5 rounded-full bg-primary"
            />
          ) : null}
        </Button>
      </PopoverTrigger>
      {/* 浮层紧贴触发按钮向右展开，改条件时会话列表大部分保持可见。 */}
      <PopoverContent side="right" align="start" className="grid gap-3">
        {scope === InboxScope.InboxScopeCustomer ? (
          <>
            <label className="grid gap-1.5">
              <span className="text-xs text-muted-foreground">
                {t("filterChannel")}
              </span>
              <NativeSelect
                value={value.channelId}
                onChange={(event) =>
                  onChange({ channelId: event.target.value })
                }
              >
                <option value="">{t("filterAllChannels")}</option>
                {channels.map((channel) => (
                  <option key={channel.id} value={channel.id}>
                    {channel.enabled
                      ? channel.name
                      : `${channel.name}（${t("filterChannelDisabled")}）`}
                  </option>
                ))}
              </NativeSelect>
            </label>
            <label className="grid gap-1.5">
              <span className="text-xs text-muted-foreground">
                {t("filterServiceStatus")}
              </span>
              <NativeSelect
                value={value.serviceStatus}
                onChange={(event) =>
                  onChange({
                    serviceStatus: event.target.value as ServiceSessionStatus,
                  })
                }
              >
                <option value={ServiceSessionStatus.ServiceSessionStatusOpen}>
                  {t("filterServiceStatusOpen")}
                </option>
                <option value={ServiceSessionStatus.ServiceSessionStatusClosed}>
                  {t("filterServiceStatusClosed")}
                </option>
              </NativeSelect>
            </label>
          </>
        ) : (
          <fieldset className="grid gap-1.5">
            <legend className="pb-1.5 text-xs text-muted-foreground">
              {t("filterKind")}
            </legend>
            {options.map((option) => (
              <label key={option.kind} className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="size-4 accent-primary"
                  checked={value.kinds.includes(option.kind)}
                  onChange={(event) =>
                    onChange({
                      kinds: toggleInboxKinds(scope, value.kinds, option.kind, event.target.checked),
                    })
                  }
                />
                <span>{t(option.label)}</span>
              </label>
            ))}
          </fieldset>
        )}
        {applied ? (
          <Button
            variant="outline"
            size="sm"
            className="justify-self-start"
            onClick={() =>
              onChange({
                channelId: "",
                serviceStatus: ServiceSessionStatus.ServiceSessionStatusOpen,
                kinds: [],
              })
            }
          >
            {t("filterReset")}
          </Button>
        ) : null}
      </PopoverContent>
    </Popover>
  )
}
