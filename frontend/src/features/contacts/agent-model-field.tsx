/** AI 员工对话模型选择字段。 */
import { useEffect, useMemo, useState } from "react"
import {
  Controller,
  type Control,
  type FieldPathByValue,
  type FieldValues,
} from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router"

import { listAgentModelOptions, type AgentModelOption } from "@/api"
import {
  Field,
  FieldDescription,
  FieldLabel,
} from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { agentModelSelection } from "@/features/contacts/agent-model-selection"
import { recoverSession } from "@/lib/session-navigation"

type LoadState = "loading" | "ready" | "error"

/** 按供应商分组模型。 */
function groupModels(models: AgentModelOption[]) {
  const groups = new Map<
    string,
    { providerName: string; models: AgentModelOption[] }
  >()
  for (const model of models) {
    const group = groups.get(model.providerId)
    if (group) {
      group.models.push(model)
    } else {
      groups.set(model.providerId, {
        providerName: model.providerName,
        models: [model],
      })
    }
  }
  return [...groups.entries()]
}

/** 渲染 AI 员工对话模型选择字段。 */
export function AgentModelField<TValues extends FieldValues>({
  control,
  name,
  disabled = false,
}: {
  control: Control<TValues>
  name: FieldPathByValue<TValues, string>
  disabled?: boolean
}) {
  const { t } = useTranslation("contacts")
  const navigate = useNavigate()
  const [models, setModels] = useState<AgentModelOption[]>([])
  const [loadState, setLoadState] = useState<LoadState>("loading")
  const groups = useMemo(() => groupModels(models), [models])

  useEffect(() => {
    let active = true
    void listAgentModelOptions()
      .then((output) => {
        if (!active) return
        setModels(output)
        setLoadState("ready")
      })
      .catch((error: unknown) => {
        if (!active) return
        if (recoverSession(error, navigate)) return
        console.warn("读取 AI 员工对话模型失败", { error })
        setModels([])
        setLoadState("error")
      })
    return () => {
      active = false
    }
  }, [navigate])

  return (
    <Controller
      name={name}
      control={control}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <FieldLabel htmlFor={`${name}-select`} required>
            {t("agents.capability.model")}
          </FieldLabel>
          <NativeSelect
            {...field}
            id={`${name}-select`}
            required
            disabled={disabled || loadState === "loading"}
            aria-invalid={fieldState.invalid}
          >
            <option value="">
              {loadState === "loading"
                ? t("agents.capability.modelLoading")
                : t("agents.capability.modelSelect")}
            </option>
            {groups.map(([providerId, group]) => (
              <optgroup key={providerId} label={group.providerName}>
                {group.models.map((model) => (
                  <option
                    key={model.modelIdentifier}
                    value={agentModelSelection(
                      model.providerId,
                      model.modelIdentifier,
                    )}
                  >
                    {model.modelName}
                  </option>
                ))}
              </optgroup>
            ))}
          </NativeSelect>
          {loadState === "error" ? (
            <FieldDescription>
              {t("agents.capability.modelLoadError")}
            </FieldDescription>
          ) : loadState === "ready" && models.length === 0 ? (
            <FieldDescription>
              {t("agents.capability.noModels")}{" "}
              <Link to="/integrations/model-services/chat">
                {t("agents.capability.configureModels")}
              </Link>
            </FieldDescription>
          ) : null}
        </Field>
      )}
    />
  )
}
