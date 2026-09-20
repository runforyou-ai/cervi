/** 定义工作台页面路由和多标签分组。 */
import type { ReactElement } from "react"
import { matchRoutes, useRoutes, type Location, type RouteObject } from "react-router"

import { MessageChannelFormPage } from "@/features/channels/message-channel-form-page"
import { MessageChannelListPage } from "@/features/channels/message-channel-list-page"
import { AgentFormPage } from "@/features/contacts/agents/agent-form-page"
import { ContactsPage } from "@/features/contacts/contacts-page"
import { InboxRoute } from "@/features/inbox/inbox-route"
import { MCPServerFormPage } from "@/features/integrations/mcp-servers/mcp-server-form-page"
import { MCPServerListPage } from "@/features/integrations/mcp-servers/mcp-server-list-page"
import { ModelProviderFormPage } from "@/features/integrations/model-services/model-provider-form-page"
import { ModelProviderListPage } from "@/features/integrations/model-services/model-provider-list-page"
import { KnowledgeDocumentListPage } from "@/features/knowledge-base/knowledge-document-list-page"
import { KnowledgeDocumentFormPage } from "@/features/knowledge-base/knowledge-document-form-page"
import { KnowledgeDocumentPage } from "@/features/knowledge-base/knowledge-document-page"
import { KnowledgeQAListPage } from "@/features/knowledge-base/knowledge-qa-list-page"
import { KnowledgeQAFormPage } from "@/features/knowledge-base/knowledge-qa-form-page"
import { KnowledgeBaseFormPage } from "@/features/knowledge-base/knowledge-base-form-page"
import { KnowledgeBaseIndexPage } from "@/features/knowledge-base/knowledge-base-index-page"
import { KnowledgeBaseLayout } from "@/features/knowledge-base/knowledge-base-layout"
import { RoleFormPage } from "@/features/roles/role-form-page"
import { SettingsPage } from "@/features/settings/settings-page"

/** 需要公共外壳的路由前缀，同一前缀下的页面渲染在对应布局内。 */
const workspaceRouteLayouts = [
  { prefix: "/knowledge-bases", element: <KnowledgeBaseLayout /> },
]

/**
 * 工作台路由清单，标签解析与页面渲染共用同一份定义。
 * tabPath 把同一功能模块的页面归入同一个标签。
 */
const workspaceRouteDefinitions = [
  { path: "/inbox", titleKey: "tabs.routes.inbox", element: <InboxRoute /> },
  {
    path: "/settings/profile",
    titleKey: "tabs.routes.profile",
    element: <SettingsPage section="profile" />,
  },
  {
    path: "/settings/security",
    titleKey: "tabs.routes.security",
    element: <SettingsPage section="security" />,
  },
  {
    path: "/settings/preferences",
    titleKey: "tabs.routes.preferences",
    element: <SettingsPage section="preferences" />,
  },
  {
    path: "/settings/devices",
    titleKey: "tabs.routes.devices",
    element: <SettingsPage section="devices" />,
  },
  {
    path: "/settings/general",
    titleKey: "tabs.routes.general",
    element: <SettingsPage section="general" />,
  },
  {
    path: "/settings/roles/new",
    tabPath: "/settings/roles",
    titleKey: "tabs.routes.roles",
    element: (
      <SettingsPage section="roles">
        <RoleFormPage mode="create" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/roles/:roleId",
    tabPath: "/settings/roles",
    titleKey: "tabs.routes.roles",
    element: (
      <SettingsPage section="roles">
        <RoleFormPage mode="detail" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/roles",
    titleKey: "tabs.routes.roles",
    element: <SettingsPage section="roles" />,
  },
  {
    path: "/contacts/employees",
    titleKey: "tabs.routes.employees",
    element: <ContactsPage scope="employees" />,
  },
  {
    path: "/contacts/ai-employees",
    titleKey: "tabs.routes.aiEmployees",
    element: <ContactsPage scope="agents" />,
  },
  {
    path: "/contacts/ai-employees/new",
    tabPath: "/contacts/ai-employees",
    titleKey: "tabs.routes.aiEmployees",
    element: (
      <ContactsPage scope="agents">
        <AgentFormPage mode="create" />
      </ContactsPage>
    ),
  },
  {
    path: "/contacts/ai-employees/:agentId",
    tabPath: "/contacts/ai-employees",
    titleKey: "tabs.routes.aiEmployees",
    element: (
      <ContactsPage scope="agents">
        <AgentFormPage mode="edit" />
      </ContactsPage>
    ),
  },
  {
    path: "/contacts/teams/:teamId",
    titleKey: "tabs.routes.team",
    element: <ContactsPage scope="team" />,
  },
  {
    path: "/contacts/external",
    titleKey: "tabs.routes.externalContacts",
    element: <ContactsPage scope="external" />,
  },
  {
    path: "/knowledge-bases/new",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeBaseFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeBaseFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa/new",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeQAFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa/:entryId/edit",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeQAFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeQAListPage />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents/new",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeDocumentFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents/:documentId/edit",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeDocumentFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents/:documentId",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeDocumentPage />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeDocumentListPage />,
  },
  {
    path: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
    element: <KnowledgeBaseIndexPage />,
  },
  {
    path: "/settings/channels/new",
    tabPath: "/settings/channels",
    titleKey: "tabs.routes.channels",
    element: (
      <SettingsPage section="channels">
        <MessageChannelFormPage mode="create" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/channels/:channelType/:channelId",
    tabPath: "/settings/channels",
    titleKey: "tabs.routes.channels",
    element: (
      <SettingsPage section="channels">
        <MessageChannelFormPage mode="edit" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/channels",
    titleKey: "tabs.routes.channels",
    element: (
      <SettingsPage section="channels">
        <MessageChannelListPage />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/mcp-servers/new",
    tabPath: "/settings/mcp-servers",
    titleKey: "tabs.routes.mcpServers",
    element: (
      <SettingsPage section="mcpServers">
        <MCPServerFormPage mode="create" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/mcp-servers/:mcpServerId",
    tabPath: "/settings/mcp-servers",
    titleKey: "tabs.routes.mcpServers",
    element: (
      <SettingsPage section="mcpServers">
        <MCPServerFormPage mode="edit" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/mcp-servers",
    titleKey: "tabs.routes.mcpServers",
    element: (
      <SettingsPage section="mcpServers">
        <MCPServerListPage />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/chat/new",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="create" returnSection="chat" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/chat/:providerId",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="edit" returnSection="chat" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/chat",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderListPage section="chat" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/embedding/new",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="create" returnSection="embedding" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/embedding/:providerId",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="edit" returnSection="embedding" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/embedding",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderListPage section="embedding" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/rerank/new",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="create" returnSection="rerank" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/rerank/:providerId",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="edit" returnSection="rerank" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/rerank",
    tabPath: "/settings/model-services",
    titleKey: "tabs.routes.modelServices",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderListPage section="rerank" />
      </SettingsPage>
    ),
  },
] as const satisfies readonly {
  path: string
  tabPath?: string
  titleKey: string
  element: ReactElement
}[]

type WorkspaceRouteDefinition = (typeof workspaceRouteDefinitions)[number]

/** 返回路由所属的外壳，无匹配前缀时在顶层渲染。 */
function layoutOf(path: string) {
  return workspaceRouteLayouts.find(
    (layout) => path === layout.prefix || path.startsWith(`${layout.prefix}/`),
  )
}

/**
 * 由清单生成的路由树，标签解析与页面渲染共用。
 * 匹配按 react-router 的路径评分决定，与清单顺序无关。
 */
const workspaceRouteObjects: RouteObject[] = [
  ...workspaceRouteDefinitions
    .filter((definition) => !layoutOf(definition.path))
    .map((definition) => ({
      path: definition.path,
      element: definition.element,
      handle: definition,
    })),
  ...workspaceRouteLayouts.map((layout) => ({
    path: layout.prefix,
    element: layout.element,
    children: workspaceRouteDefinitions
      .filter((definition) => layoutOf(definition.path) === layout)
      .map((definition) => {
        const relative = definition.path.slice(layout.prefix.length + 1)
        return {
          ...(relative ? { path: relative } : { index: true as const }),
          element: definition.element,
          handle: definition,
        }
      }),
  })),
]

const workspaceRedirects: Readonly<Record<string, string>> = {
  "/settings": "/settings/profile",
  "/contacts": "/contacts/employees",
  "/settings/model-services": "/settings/model-services/chat",
}

export type WorkspaceTabTitleKey =
  (typeof workspaceRouteDefinitions)[number]["titleKey"]

export type ResolvedWorkspaceTab = {
  id: string
  href: string
  titleKey: WorkspaceTabTitleKey
}

export type ResolvedWorkspaceLocation = {
  canonicalHref: string
  tab: ResolvedWorkspaceTab | null
}

export const defaultWorkspaceTab = {
  id: "/inbox",
  href: "/inbox",
  titleKey: "tabs.routes.inbox",
} satisfies ResolvedWorkspaceTab

/** 把当前地址解析为规范标签；别名地址给出跳转目标，未知地址回到消息页。 */
export function resolveWorkspaceLocation(
  location: Pick<Location, "pathname" | "search" | "hash">,
): ResolvedWorkspaceLocation {
  // 去掉非根路径的尾部斜杠。
  const pathname =
    location.pathname === "/"
      ? location.pathname
      : location.pathname.replace(/\/+$/, "") || "/"
  const redirectedPathname = workspaceRedirects[pathname]
  if (redirectedPathname) {
    return {
      canonicalHref: `${redirectedPathname}${location.search}${location.hash}`,
      tab: null,
    }
  }

  const matches = matchRoutes(workspaceRouteObjects, pathname)
  const definition = matches?.[matches.length - 1].route.handle as
    | WorkspaceRouteDefinition
    | undefined
  if (!definition) {
    return { canonicalHref: defaultWorkspaceTab.href, tab: null }
  }

  const href = `${pathname}${location.search}${location.hash}`
  return {
    canonicalHref: href,
    tab: {
      id: "tabPath" in definition ? definition.tabPath : pathname,
      href,
      titleKey: definition.titleKey,
    },
  }
}

/** 按指定地址渲染一份工作台页面树。 */
export function WorkspacePageRoutes({ location }: { location: string }) {
  return useRoutes(workspaceRouteObjects, location)
}
