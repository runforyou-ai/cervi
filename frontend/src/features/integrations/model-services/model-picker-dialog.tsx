/** 模型服务表单中读取可选模型并批量加入目录的入口和弹窗。 */
import { useEffect, useRef, useState } from "react"
import { useWatch, type UseFormReturn } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  discoverAIProviderModels,
  isApiError,
  listAvailableAIModels,
  type AIProviderBrandId,
  type AIProviderModelData,
} from "@/api"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { aiProviderBrandConfigs } from "@/features/integrations/model-services/model-provider-brands"
import { modelFormValue } from "@/features/integrations/model-services/model-provider-model-values"
import type { AIProviderFormValues } from "@/features/integrations/model-services/model-provider-schema"
import {
  modelInputModalityNameKeys,
  modelTypeNameKeys,
} from "@/features/integrations/model-services/model-service-options"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

/** 读取当前品牌的可选模型，确认后把目录中尚不存在的模型交给表单追加。 */
export function ModelPickerDialog({
  form,
  onAppend,
}: {
  form: UseFormReturn<AIProviderFormValues>
  onAppend: (models: AIProviderFormValues["models"]) => void
}) {
  const { t } = useTranslation(["integrations", "common"])
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const [availableModels, setAvailableModels] = useState<AIProviderModelData[]>([])
  const [draftModelIDs, setDraftModelIDs] = useState<Set<string>>(new Set())
  const [loadingModels, setLoadingModels] = useState(false)
  const mounted = useRef(true)
  const watchedBrand = useWatch({ control: form.control, name: "brand" }) as AIProviderBrandId
  const discoversModels = Boolean(aiProviderBrandConfigs[watchedBrand].discoversModels)

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 读取当前品牌的可选模型并打开选择弹窗。 */
  async function openModelDialog() {
    if (loadingModels) return
    const brand = form.getValues("brand") as AIProviderBrandId
    // 模型目录由服务实例提供时，先校验连接配置再读取实例。
    const discovers = Boolean(aiProviderBrandConfigs[brand].discoversModels)
    if (discovers) {
      const valid = await form.trigger(["brand", "credentialType", "apiKey", "apiUrl"], {
        shouldFocus: true,
      })
      if (!valid || !mounted.current) return
    }
    setLoadingModels(true)
    const { credentialType, apiKey, apiUrl } = form.getValues()
    const requested = `${brand}\n${credentialType}\n${apiKey}\n${apiUrl}`
    try {
      const models = discovers
        ? await discoverAIProviderModels({ brand, credentialType, apiKey, apiUrl })
        : await listAvailableAIModels(brand)
      if (!mounted.current) return
      // 读取期间连接配置变化时结果已过期，不能追加到当前品牌的目录。
      const current = form.getValues()
      if (
        requested !==
        `${current.brand}\n${current.credentialType}\n${current.apiKey}\n${current.apiUrl}`
      ) {
        return
      }
      setAvailableModels(models)
      setDraftModelIDs(new Set(models.map((model) => model.identifier)))
      setOpen(true)
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("可选模型加载失败", { brand, error: requestError })
      toast.error(
        isApiError(requestError)
          ? apiErrorMessage(requestError, ["brand", "credentialType", "apiKey", "apiUrl"])
          : t(
              discovers
                ? "modelServices.models.discoverError"
                : "modelServices.models.loadError",
            ),
      )
    } finally {
      if (mounted.current) setLoadingModels(false)
    }
  }

  /** 切换弹窗中的待选模型。 */
  function toggleDraftModel(identifier: string, checked: boolean) {
    setDraftModelIDs((current) => {
      const next = new Set(current)
      if (checked) next.add(identifier)
      else next.delete(identifier)
      return next
    })
  }

  /** 确认模型选择并追加目录中尚不存在的模型。 */
  function confirmModels() {
    const existingIDs = new Set(
      form.getValues("models").map((model) => model.identifier.trim()),
    )
    const modelsToAppend = availableModels
      .filter(
        (model) =>
          draftModelIDs.has(model.identifier) && !existingIDs.has(model.identifier),
      )
      .map(modelFormValue)
    if (modelsToAppend.length > 0) onAppend(modelsToAppend)
    setOpen(false)
  }

  return (
    <>
      <Button
        type="button"
        variant="link"
        size="sm"
        className="h-auto p-0"
        disabled={loadingModels}
        onClick={() => void openModelDialog()}
      >
        {loadingModels
          ? t("modelServices.models.loading")
          : t(
              discoversModels
                ? "modelServices.models.discover"
                : "modelServices.models.fetch",
            )}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>
              {t(
                discoversModels
                  ? "modelServices.models.discoverDialogTitle"
                  : "modelServices.models.dialogTitle",
              )}
            </DialogTitle>
          </DialogHeader>
          <div className="max-h-[60vh] overflow-auto rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="w-12">
                    <span className="sr-only">{t("modelServices.models.select")}</span>
                  </TableHead>
                  <TableHead>{t("modelServices.models.columns.identifier")}</TableHead>
                  <TableHead>{t("modelServices.models.columns.type")}</TableHead>
                  <TableHead>
                    {t("modelServices.models.columns.inputModalities")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {availableModels.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-24 text-center text-muted-foreground"
                    >
                      {t("modelServices.models.dialogEmpty")}
                    </TableCell>
                  </TableRow>
                ) : null}
                {availableModels.map((model) => (
                  <TableRow key={model.identifier}>
                    <TableCell>
                      <input
                        type="checkbox"
                        className="size-4 accent-primary"
                        checked={draftModelIDs.has(model.identifier)}
                        onChange={(event) =>
                          toggleDraftModel(model.identifier, event.target.checked)
                        }
                        aria-label={t("modelServices.models.toggle", {
                          name: model.name,
                        })}
                      />
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      {model.identifier}
                    </TableCell>
                    <TableCell>{t(modelTypeNameKeys[model.type])}</TableCell>
                    <TableCell>
                      {model.inputModalities
                        .map((modality) => t(modelInputModalityNameKeys[modality]))
                        .join("、")}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <div className="flex items-center justify-between gap-3">
            <Button type="button" variant="link" onClick={() => setDraftModelIDs(new Set())}>
              {t("modelServices.models.clearAll")}
            </Button>
            <div className="flex gap-2">
              <Button type="button" variant="outline" onClick={() => setOpen(false)}>
                {t("common:actions.cancel")}
              </Button>
              <Button type="button" onClick={confirmModels}>
                {t("common:actions.confirm")}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
