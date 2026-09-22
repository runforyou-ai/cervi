/** 知识库模块布局，在窄侧栏中展示知识库和两级分组。 */
import { useCallback, useEffect, useRef, useState } from "react"
import {
  CircleHelpIcon,
  FileTextIcon,
  FolderIcon,
  PlusIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, Outlet, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteKnowledgeBase,
  deleteKnowledgeGroup,
  isApiError,
  KnowledgeBaseCategory,
  listKnowledgeBaseAgents,
  listKnowledgeBases,
  type KnowledgeBaseAgentListData,
  type KnowledgeBaseData,
  type KnowledgeGroupData,
  sessionPath,
  UserStatus,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { resourceStatus } from "@/components/resource-content"
import { PagePaneNav, PageSplit } from "@/components/page-split"
import { StatusBadge } from "@/components/status-badge"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { Button } from "@/components/ui/button"
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { KnowledgeBaseProvider } from "@/features/knowledge-base/knowledge-base-context"
import {
  KnowledgeGroupDialog,
  type KnowledgeGroupDialogState,
} from "@/features/knowledge-base/knowledge-group-dialog"
import { resourceKeys } from "@/hooks/resource-keys"
import { useConfirmedAction } from "@/hooks/use-confirmed-action"
import { useResource, useResourceInvalidator, useResourceReader } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { cn } from "@/lib/utils"

type DeleteKnowledgeBaseTarget = {
  knowledgeBase: KnowledgeBaseData
  agents: KnowledgeBaseAgentListData["agents"]
}

type DeleteGroupTarget = {
  knowledgeBase: KnowledgeBaseData
  group: KnowledgeGroupData
}

/** 显示知识库资源树和管理页面。 */
export function KnowledgeBaseLayout() {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const location = useLocation()
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [groupDialog, setGroupDialog] =
    useState<KnowledgeGroupDialogState | null>(null)
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

  /** 把创建、保存或分组结果同步到窄侧栏。 */
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

  const groupDeletion = useConfirmedAction<DeleteGroupTarget>({
    action: (target) =>
      deleteKnowledgeGroup(target.knowledgeBase.id, target.group.id),
    invalidateKeys: (target) => [
      resourceKeys.knowledgeBases(),
      resourceKeys.knowledgeBase(target.knowledgeBase.id),
    ],
    logLabel: "知识库分组删除",
    successMessage: () => t("group.deleteSuccess"),
    errorMessage: () => t("group.deleteError"),
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
                <KnowledgeBaseTree
                  key={knowledgeBase.id}
                  knowledgeBase={knowledgeBase}
                  currentPath={location.pathname}
                  onCreateGroup={(parentId) =>
                    setGroupDialog({ knowledgeBase, parentId })
                  }
                  onEditGroup={(group) =>
                    setGroupDialog({ knowledgeBase, group })
                  }
                  onDeleteGroup={(group) =>
                    groupDeletion.select({ knowledgeBase, group })
                  }
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

      <KnowledgeGroupDialog
        state={groupDialog}
        onOpenChange={(open) => !open && setGroupDialog(null)}
        onSaved={upsertKnowledgeBase}
      />

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

      <ConfirmationDialog
        {...groupDeletion.dialog}
        title={t("group.deleteTitle")}
        description={t("group.deleteDescription")}
        pendingLabel={t("common:actions.deleting")}
      />
    </>
  )
}

/** 渲染一个知识库及其分组管理入口。 */
function KnowledgeBaseTree({
  knowledgeBase,
  currentPath,
  onCreateGroup,
  onEditGroup,
  onDeleteGroup,
  onDeleteKnowledgeBase,
}: {
  knowledgeBase: KnowledgeBaseData
  currentPath: string
  onCreateGroup: (parentId?: string) => void
  onEditGroup: (group: KnowledgeGroupData) => void
  onDeleteGroup: (group: KnowledgeGroupData) => void
  onDeleteKnowledgeBase: () => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  const path = `/knowledge-bases/${knowledgeBase.id}`
  const isQA =
    knowledgeBase.category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  const active = currentPath === path
  const categoryLabel = isQA
    ? t("category.qaShort")
    : t("category.standardShort")
  const defaultGroup = knowledgeBase.groups.find((group) => group.isDefault)
  const regularGroups = knowledgeBase.groups.filter((group) => !group.isDefault)

  return (
    <section className="mb-2">
      {/* 知识库操作通过右键菜单完成，菜单打开期间保持该行高亮。 */}
      <ContextMenu>
        <ContextMenuTrigger asChild>
          <div
            className={cn(
              "flex items-center rounded-md transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[state=open]:bg-sidebar-accent",
              active && "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
            )}
          >
            <Link
              to={path}
              className="flex h-8 min-w-0 flex-1 items-center gap-2 px-2.5 text-sm"
              title={knowledgeBase.name}
            >
              {isQA ? <CircleHelpIcon /> : <FileTextIcon />}
              <span className="truncate">{knowledgeBase.name}</span>
              <StatusBadge variant="muted" className="ml-auto shrink-0 font-normal">
                {categoryLabel}
              </StatusBadge>
            </Link>
          </div>
        </ContextMenuTrigger>
        <ContextMenuContent>
          <ContextMenuItem onSelect={() => onCreateGroup()}>
            {t("sidebar.addGroup")}
          </ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem destructive onSelect={onDeleteKnowledgeBase}>
            {t("common:actions.delete")}
          </ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>

      <div className="mt-1 ml-3 border-l pl-2">
        {defaultGroup ? (
          <Link
            to={`${path}/groups/${defaultGroup.id}/${isQA ? "qa" : "documents"}`}
            className={cn(
              "flex h-8 items-center gap-2 rounded-md px-2 text-sm text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
              currentPath.startsWith(`${path}/groups/${defaultGroup.id}/${isQA ? "qa" : "documents"}`) &&
                "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
            )}
          >
            <FolderIcon className="size-3.5 shrink-0" />
            <span className="truncate">{t("group.default")}</span>
          </Link>
        ) : (
          <div className="flex h-8 items-center gap-2 px-2 text-xs text-muted-foreground">
            <FolderIcon className="size-3.5 shrink-0" />
            <span className="truncate">{t("group.default")}</span>
          </div>
        )}
        {regularGroups.map((group) => (
          <div key={group.id}>
            <KnowledgeGroupTreeRow
              group={group}
              contentPath={
                isQA
                  ? `${path}/groups/${group.id}/qa`
                  : `${path}/groups/${group.id}/documents`
              }
              currentPath={currentPath}
              onAddChild={() => onCreateGroup(group.id)}
              onEdit={() => onEditGroup(group)}
              onDelete={() => onDeleteGroup(group)}
            />
            {group.children.map((child) => (
              <div key={child.id} className="ml-4">
                <KnowledgeGroupTreeRow
                  group={child}
                  contentPath={
                    isQA
                      ? `${path}/groups/${child.id}/qa`
                      : `${path}/groups/${child.id}/documents`
                  }
                  currentPath={currentPath}
                  onEdit={() => onEditGroup(child)}
                  onDelete={() => onDeleteGroup(child)}
                />
              </div>
            ))}
          </div>
        ))}
      </div>
    </section>
  )
}

/** 渲染知识库分组行，右键打开分组操作菜单。 */
function KnowledgeGroupTreeRow({
  group,
  contentPath,
  currentPath,
  onAddChild,
  onEdit,
  onDelete,
}: {
  group: KnowledgeGroupData
  contentPath?: string
  currentPath: string
  onAddChild?: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation(["knowledgeBase", "common"])
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <div
          className={cn(
            "flex h-8 items-center rounded-md px-2 text-sm text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground data-[state=open]:bg-sidebar-accent",
            contentPath &&
              currentPath.startsWith(contentPath) &&
              "bg-sidebar-accent font-medium text-sidebar-accent-foreground",
          )}
        >
          <FolderIcon className="size-3.5 shrink-0" />
          {contentPath ? (
            <Link
              to={contentPath}
              className="ml-2 min-w-0 flex-1 truncate py-2"
            >
              {group.name}
            </Link>
          ) : (
            <span className="ml-2 min-w-0 flex-1 truncate">{group.name}</span>
          )}
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        {onAddChild ? (
          <ContextMenuItem onSelect={onAddChild}>
            {t("group.addChild")}
          </ContextMenuItem>
        ) : null}
        <ContextMenuItem onSelect={onEdit}>{t("common:actions.edit")}</ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem destructive onSelect={onDelete}>
          {t("common:actions.delete")}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  )
}
