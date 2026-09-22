/** 知识库模块布局，在窄侧栏中展示知识库列表。 */
import { useCallback, useEffect, useRef } from "react"
import {
  CircleHelpIcon,
  FileTextIcon,
  PlusIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, Outlet, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteKnowledgeBase,
  isApiError,
  KnowledgeBaseCategory,
  listKnowledgeBaseAgents,
  listKnowledgeBases,
  type KnowledgeBaseAgentListData,
  type KnowledgeBaseData,
  sessionPath,
  UserStatus,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { resourceStatus } from "@/components/resource-content"
import { PagePaneNav, PageSplit } from "@/components/page-split"
import { RowActionsMenu } from "@/components/row-actions-menu"
import { StatusBadge } from "@/components/status-badge"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { KnowledgeBaseProvider } from "@/features/knowledge-base/knowledge-base-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator, useResourceReader } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { cn } from "@/lib/utils"

type DeleteKnowledgeBaseTarget = {
  knowledgeBase: KnowledgeBaseData
  agents: KnowledgeBaseAgentListData["agents"]
}

/** 列表行尾「⋯」浮在行右端，出现时盖住行尾内容，名称宽度保持不变。 */
const rowMoreButtonClass =
  "absolute top-1/2 right-1 -translate-y-1/2 bg-sidebar-accent"

/** 显示知识库列表和管理页面。 */
export function KnowledgeBaseLayout() {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const location = useLocation()
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const readResource = useResourceReader()
  const mounted = useRef(true)
  const indexActive =
    location.pathname === "/knowledge-bases" ||
    location.pathname === "/knowledge-bases/"
  const resource = useResource(resourceKeys.knowledgeBases(), () => listKnowledgeBases())
  const { data, refresh } = resource
  const { status } = resourceStatus(resource)
  const knowledgeBases = data?.knowledgeBases ?? []

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])

  /** 把知识库创建和保存结果同步到窄侧栏。 */
  const upsertKnowledgeBase = useCallback(
    (knowledgeBase: KnowledgeBaseData) => {
      void refresh()
      void invalidate(resourceKeys.knowledgeBase(knowledgeBase.id))
    },
    [refresh, invalidate],
  )

  const knowledgeBaseDeletion = useConfirmedAction<DeleteKnowledgeBaseTarget>({
    action: (target) => deleteKnowledgeBase(target.knowledgeBase.id),
    invalidateKeys: () => [resourceKeys.knowledgeBases()],
    logLabel: "知识库删除",
    successMessage: () => t("delete.success"),
    errorMessage: () => t("delete.error"),
    onSuccess: (target) => {
      if (location.pathname.startsWith(`/knowledge-bases/${target.knowledgeBase.id}`))
        navigate("/knowledge-bases")
    },
  })

  const deletingAgents = knowledgeBaseDeletion.item?.agents ?? []

  /** 读取最新的 AI 员工绑定后打开知识库删除确认。 */
  async function requestDeleteKnowledgeBase(knowledgeBase: KnowledgeBaseData) {
    try {
      const result = await readResource(
        resourceKeys.knowledgeBaseAgents(knowledgeBase.id),
        () => listKnowledgeBaseAgents(knowledgeBase.id),
      )
      if (!mounted.current) return
      knowledgeBaseDeletion.select({ knowledgeBase, agents: result.agents })
    } catch (error) {
      if (!mounted.current) return
      if (isApiError(error) && sessionPath(error.state)) return
      console.warn("知识库 AI 员工读取失败", {
        knowledge_base_id: knowledgeBase.id,
        error,
      })
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("agents.loadError"),
      )
    }
  }

  return (
    <>
      <PageSplit
        paneVariant="nav"
        paneOnNarrow={indexActive ? "fill" : "hide"}
        mainClassName={cn(indexActive && "hidden md:flex")}
        pane={
          <PagePaneNav
            label={t("navigation")}
            title={t("title")}
            action={
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    className="shrink-0 text-muted-foreground"
                    aria-label={t("common:actions.new")}
                    title={t("common:actions.new")}
                  >
                    <PlusIcon />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent side="right" align="start" className="w-48">
                  <DropdownMenuItem asChild>
                    <Link to="/knowledge-bases/new?category=standard">
                      <FileTextIcon />
                      {t("sidebar.createStandard")}
                    </Link>
                  </DropdownMenuItem>
                  <DropdownMenuItem asChild>
                    <Link to="/knowledge-bases/new?category=qa">
                      <CircleHelpIcon />
                      {t("sidebar.createQA")}
                    </Link>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            }
          >
            {status === "loading" ? (
              <LoadingIndicator className="h-20 justify-center">
                {t("common:status.loading")}
              </LoadingIndicator>
            ) : status === "error" ? (
              <div className="flex flex-col items-center px-2 py-6 text-center">
                <p className="text-sm text-muted-foreground">
                  {t("sidebar.loadError")}
                </p>
                <Button
                  className="mt-3"
                  variant="outline"
                  size="sm"
                  onClick={() => void refresh()}
                >
                  {t("common:actions.retry")}
                </Button>
              </div>
            ) : knowledgeBases.length === 0 ? (
              <p className="px-2 py-6 text-center text-sm text-muted-foreground">
                {t("sidebar.empty")}
              </p>
            ) : (
              knowledgeBases.map((knowledgeBase) => (
                <KnowledgeBaseRow
                  key={knowledgeBase.id}
                  knowledgeBase={knowledgeBase}
                  currentPath={location.pathname}
                  onDeleteKnowledgeBase={() =>
                    void requestDeleteKnowledgeBase(knowledgeBase)
                  }
                />
              ))
            )}
          </PagePaneNav>
        }
      >
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <KnowledgeBaseProvider
            upsertKnowledgeBase={upsertKnowledgeBase}
          >
            <Outlet />
          </KnowledgeBaseProvider>
        </div>
      </PageSplit>

      <ConfirmationDialog
        {...knowledgeBaseDeletion.dialog}
        title={
          knowledgeBaseDeletion.item
            ? t("delete.title", { name: knowledgeBaseDeletion.item.knowledgeBase.name })
            : ""
        }
        description={
          deletingAgents.length > 0
            ? t("delete.agentsDescription", {
                count: deletingAgents.length,
                names: deletingAgents
                  .map((agent) =>
                    agent.status === UserStatus.UserStatusActive
                      ? agent.displayName
                      : t("agents.inactive", { name: agent.displayName }),
                  )
                  .join(t("delete.agentSeparator")),
              })
            : t("delete.description")
        }
        pendingLabel={t("common:actions.deleting")}
      />
    </>
  )
}

/** 展示知识库内容入口和编辑、删除菜单。 */
function KnowledgeBaseRow({
  knowledgeBase,
  currentPath,
  onDeleteKnowledgeBase,
}: {
  knowledgeBase: KnowledgeBaseData
  currentPath: string
  onDeleteKnowledgeBase: () => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const navigate = useNavigate()
  const path = `/knowledge-bases/${knowledgeBase.id}`
  const isQA =
    knowledgeBase.category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  const active = currentPath === path || currentPath.startsWith(`${path}/`)
  const categoryLabel = isQA
    ? t("category.qaShort")
    : t("category.standardShort")

  return (
    <RowActionsMenu
      buttonSize="icon-xs"
      buttonClassName={rowMoreButtonClass}
      actions={[
        {
          key: "edit",
          label: t("common:actions.edit"),
          onSelect: () => navigate(path),
        },
        {
          key: "delete",
          label: t("common:actions.delete"),
          destructive: true,
          separatorBefore: true,
          onSelect: onDeleteKnowledgeBase,
        },
      ]}
    >
      {({ moreButton, menuOpen }) => (
        <div
          className={cn(
            "group/row relative flex items-center rounded-md transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
            (active || menuOpen) && "bg-sidebar-accent",
            active && "font-medium text-sidebar-accent-foreground",
          )}
        >
          <Link
            to={`${path}/${isQA ? "qa" : "documents"}`}
            className="flex h-8 min-w-0 flex-1 items-center gap-2 px-2.5 text-sm"
            title={knowledgeBase.name}
          >
            {isQA ? <CircleHelpIcon /> : <FileTextIcon />}
            <span className="truncate">{knowledgeBase.name}</span>
            <StatusBadge variant="muted" className="ml-auto shrink-0 font-normal">
              {categoryLabel}
            </StatusBadge>
          </Link>
          {moreButton}
        </div>
      )}
    </RowActionsMenu>
  )
}
