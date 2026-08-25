/** 知识库模块布局，在窄侧栏中展示知识库和两级分组。 */
import { useCallback, useEffect, useRef, useState } from "react"
import {
  CircleHelpIcon,
  CloudIcon,
  FileTextIcon,
  FolderIcon,
  LoaderCircleIcon,
  MoreHorizontalIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, Outlet, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  deleteKnowledgeBase,
  deleteKnowledgeGroup,
  isApiError,
  KnowledgeBaseCategory,
  listKnowledgeBases,
  type KnowledgeBaseData,
  type KnowledgeGroupData,
} from "@/api"
import { PagePaneNav, PageSplit } from "@/components/page-split"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { KnowledgeBaseProvider } from "@/features/knowledge-base/knowledge-base-context"
import {
  KnowledgeGroupDialog,
  type KnowledgeGroupDialogState,
} from "@/features/knowledge-base/knowledge-group-dialog"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

type DeleteGroupTarget = {
  knowledgeBase: KnowledgeBaseData
  group: KnowledgeGroupData
}

/** 显示知识库资源树和当前基础管理页面。 */
export function KnowledgeBaseLayout() {
  const { t } = useTranslation("knowledgeBase")
  const location = useLocation()
  const navigate = useNavigate()
  const navigateRef = useRef(navigate)
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseData[]>([])
  const [groupDialog, setGroupDialog] =
    useState<KnowledgeGroupDialogState | null>(null)
  const [deletingKnowledgeBase, setDeletingKnowledgeBase] =
    useState<KnowledgeBaseData | null>(null)
  const [deletingGroup, setDeletingGroup] =
    useState<DeleteGroupTarget | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const indexActive =
    location.pathname === "/knowledge-bases" ||
    location.pathname === "/knowledge-bases/"

  useEffect(() => {
    navigateRef.current = navigate
  }, [navigate])

  /** 读取当前企业的知识库资源树。 */
  const load = useCallback(async () => {
    const version = ++loadVersion.current
    setLoading(true)
    setLoadError(false)
    try {
      const output = await listKnowledgeBases()
      if (version !== loadVersion.current) return
      setKnowledgeBases(output.knowledgeBases)
    } catch (error) {
      if (version !== loadVersion.current) return
      if (recoverSession(error, navigateRef.current)) return
      console.warn("知识库列表加载失败", error)
      setLoadError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

  /** 把创建、保存或分组结果同步到窄侧栏。 */
  const upsertKnowledgeBase = useCallback((knowledgeBase: KnowledgeBaseData) => {
    setKnowledgeBases((current) => {
      const next = current.filter((item) => item.id !== knowledgeBase.id)
      next.push(knowledgeBase)
      return next.sort((left, right) =>
        left.name.localeCompare(right.name, undefined, { sensitivity: "base" }),
      )
    })
  }, [])

  /** 删除当前选中的知识库。 */
  async function confirmDeleteKnowledgeBase() {
    if (!deletingKnowledgeBase || deleting) return
    const target = deletingKnowledgeBase
    setDeleting(true)
    try {
      await deleteKnowledgeBase(target.id)
      if (!mounted.current) return
      setKnowledgeBases((current) =>
        current.filter((item) => item.id !== target.id),
      )
      setDeletingKnowledgeBase(null)
      if (location.pathname.startsWith(`/knowledge-bases/${target.id}`)) {
        navigate("/knowledge-bases")
      }
      toast.success(t("delete.success"))
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) return
      console.warn("知识库删除失败", {
        knowledge_base_id: target.id,
        error,
      })
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("delete.error"))
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  /** 删除空分组并刷新当前知识库树。 */
  async function confirmDeleteGroup() {
    if (!deletingGroup || deleting) return
    const target = deletingGroup
    setDeleting(true)
    try {
      const knowledgeBase = await deleteKnowledgeGroup(
        target.knowledgeBase.id,
        target.group.id,
      )
      if (!mounted.current) return
      upsertKnowledgeBase(knowledgeBase)
      setDeletingGroup(null)
      toast.success(t("group.deleteSuccess"))
    } catch (error) {
      if (!mounted.current) return
      if (recoverSession(error, navigate)) return
      console.warn("知识库分组删除失败", {
        knowledge_base_id: target.knowledgeBase.id,
        group_id: target.group.id,
        error,
      })
      toast.error(
        isApiError(error) ? apiErrorMessage(error) : t("group.deleteError"),
      )
    } finally {
      if (mounted.current) setDeleting(false)
    }
  }

  return (
    <>
      <PageSplit
        paneWidth="md"
        paneVariant="nav"
        paneOnNarrow={indexActive ? "fill" : "hide"}
        paneClassName="md:w-68"
        mainClassName={cn(indexActive && "hidden md:flex")}
        pane={
          <PagePaneNav
            label={t("navigation")}
            title={t("title")}
            action={
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="outline" size="xs">
                    {t("sidebar.create")}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48">
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
                  <DropdownMenuItem asChild>
                    <Link to="/knowledge-bases/new?source=dify">
                      <CloudIcon />
                      {t("sidebar.createDify")}
                    </Link>
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            }
          >
            {loading ? (
              <div className="flex h-20 items-center justify-center gap-2 text-sm text-muted-foreground">
                <LoaderCircleIcon className="size-4 animate-spin" />
                {t("loading")}
              </div>
            ) : loadError ? (
              <div className="flex flex-col items-center px-2 py-6 text-center">
                <p className="text-sm text-muted-foreground">
                  {t("sidebar.loadError")}
                </p>
                <Button
                  className="mt-3"
                  variant="outline"
                  size="sm"
                  onClick={() => void load()}
                >
                  {t("retry")}
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
                    setDeletingGroup({ knowledgeBase, group })
                  }
                  onDeleteKnowledgeBase={() =>
                    setDeletingKnowledgeBase(knowledgeBase)
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

      <AlertDialog
        open={deletingKnowledgeBase !== null}
        onOpenChange={(open) =>
          !open && !deleting && setDeletingKnowledgeBase(null)
        }
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {deletingKnowledgeBase
                ? t("delete.title", { name: deletingKnowledgeBase.name })
                : null}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("delete.description")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("delete.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDeleteKnowledgeBase()}
            >
              {deleting ? t("delete.deleting") : t("delete.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog
        open={deletingGroup !== null}
        onOpenChange={(open) => !open && !deleting && setDeletingGroup(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("group.deleteTitle")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("group.deleteDescription")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("group.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={deleting}
              onClick={() => void confirmDeleteGroup()}
            >
              {deleting ? t("group.deleting") : t("group.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
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
  const { t } = useTranslation("knowledgeBase")
  const path = `/knowledge-bases/${knowledgeBase.id}`
  const active = currentPath === path
  const isQA =
    knowledgeBase.category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA
  const isExternal = knowledgeBase.integrationConnectionId !== ""
  const categoryLabel = isQA
    ? t("category.qaShort")
    : t("category.standardShort")
  const regularGroups = knowledgeBase.groups.filter((group) => !group.isDefault)

  return (
    <section className="mb-2">
      <div
        className={cn(
          "flex items-center rounded-md transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
          active && "bg-sidebar-accent/60 font-medium",
        )}
      >
        <Link
          to={path}
          className="flex h-9 min-w-0 flex-1 items-center gap-2 px-2.5 text-sm"
          title={knowledgeBase.name}
        >
          {isQA ? (
            <CircleHelpIcon />
          ) : (
            <FileTextIcon />
          )}
          <span className="truncate">{knowledgeBase.name}</span>
          <span className="ml-auto shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-normal text-muted-foreground">
            {isExternal
              ? t("sidebar.externalCategory", {
                  source: t("source.dify"),
                  category: categoryLabel,
                })
              : categoryLabel}
          </span>
        </Link>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon-xs"
              className="mr-1"
              aria-label={t("sidebar.more", { name: knowledgeBase.name })}
              title={t("sidebar.more", { name: knowledgeBase.name })}
            >
              <MoreHorizontalIcon />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {!isExternal ? (
              <DropdownMenuItem onSelect={() => onCreateGroup()}>
                {t("sidebar.addGroup")}
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem asChild>
              <Link to={path}>{t("sidebar.edit")}</Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem destructive onSelect={onDeleteKnowledgeBase}>
              {t("sidebar.delete")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <div className="mt-1 ml-3 border-l pl-2">
        <div className="flex h-8 items-center gap-2 px-2 text-xs text-muted-foreground">
          <FolderIcon className="size-3.5 shrink-0" />
          <span className="truncate">{t("group.default")}</span>
        </div>
        {regularGroups.map((group) => (
          <div key={group.id}>
            <KnowledgeGroupTreeRow
              group={group}
              onAddChild={() => onCreateGroup(group.id)}
              onEdit={() => onEditGroup(group)}
              onDelete={() => onDeleteGroup(group)}
            />
            {group.children.map((child) => (
              <div key={child.id} className="ml-4">
                <KnowledgeGroupTreeRow
                  group={child}
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

/** 渲染知识库分组行及低频操作。 */
function KnowledgeGroupTreeRow({
  group,
  onAddChild,
  onEdit,
  onDelete,
}: {
  group: KnowledgeGroupData
  onAddChild?: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation("knowledgeBase")
  return (
    <div className="group/tree flex h-8 items-center rounded-md px-2 text-xs text-muted-foreground hover:bg-sidebar-accent hover:text-sidebar-accent-foreground">
      <FolderIcon className="size-3.5 shrink-0" />
      <span className="ml-2 min-w-0 flex-1 truncate">{group.name}</span>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="ghost"
            size="icon-xs"
            className="-mr-1 opacity-0 group-hover/tree:opacity-100 data-[state=open]:opacity-100"
            aria-label={t("group.more", { name: group.name })}
          >
            <MoreHorizontalIcon />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {onAddChild ? (
            <DropdownMenuItem onSelect={onAddChild}>
              {t("group.addChild")}
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem onSelect={onEdit}>{t("group.edit")}</DropdownMenuItem>
          <DropdownMenuItem destructive onSelect={onDelete}>
            {t("group.delete")}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )
}
