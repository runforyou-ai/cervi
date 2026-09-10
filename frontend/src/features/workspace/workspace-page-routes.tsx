/** 定义工作台页面路由和多标签分组。 */
import type { ReactElement } from "react"
import { matchRoutes, useRoutes, type Location, type RouteObject } from "react-router"

import { MessageChannelFormPage } from "@/features/channels/message-channel-form-page"
import { MessageChannelListPage } from "@/features/channels/message-channel-list-page"
import { AgentFormPage } from "@/features/contacts/agents/agent-form-page"
import { ContactsPage } from "@/features/contacts/contacts-page"
import { InboxRoute } from "@/features/inbox/inbox-route"
import { AppsPage } from "@/features/apps/apps-page"
import { IntegrationsLayout } from "@/features/integrations/integrations-layout"
import { BusinessSystemFormPage } from "@/features/integrations/business-systems/business-system-form-page"
import { BusinessSystemListPage } from "@/features/integrations/business-systems/business-system-list-page"
import { MCPServerFormPage } from "@/features/integrations/mcp-servers/mcp-server-form-page"
import { MCPServerListPage } from "@/features/integrations/mcp-servers/mcp-server-list-page"
import { ModelProviderFormPage } from "@/features/integrations/model-services/model-provider-form-page"
import { ModelProviderListPage } from "@/features/integrations/model-services/model-provider-list-page"
import { KnowledgeDocumentListPage } from "@/features/knowledge-base/knowledge-document-list-page"
import { KnowledgeDocumentPage } from "@/features/knowledge-base/knowledge-document-page"
import { KnowledgeQAListPage } from "@/features/knowledge-base/knowledge-qa-list-page"
import { KnowledgeQAFormPage } from "@/features/knowledge-base/knowledge-qa-form-page"
import { KnowledgeBaseFormPage } from "@/features/knowledge-base/knowledge-base-form-page"
import { KnowledgeBaseIndexPage } from "@/features/knowledge-base/knowledge-base-index-page"
import { KnowledgeBaseLayout } from "@/features/knowledge-base/knowledge-base-layout"
import { RoleFormPage } from "@/features/roles/role-form-page"
import {
  PersonalSettingsPage,
  SystemSettingsPage,
} from "@/features/settings/settings-page"

/** 需要公共外壳的路由前缀，同一前缀下的页面渲染在对应布局内。 */
const workspaceRouteLayouts = [
  { prefix: "/knowledge-bases", element: <KnowledgeBaseLayout /> },
  { prefix: "/integrations", element: <IntegrationsLayout /> },
]

/**
 * 工作台路由清单，标签解析与页面渲染共用同一份定义。
 * tabPath 把同一功能模块的页面归入同一个标签。
 */
const workspaceRouteDefinitions = [
  { path: "/inbox", titleKey: "tabs.routes.inbox", element: <InboxRoute /> },
  {
    path: "/account/profile",
    titleKey: "tabs.routes.profile",
    element: <PersonalSettingsPage section="profile" />,
  },
  {
    path: "/account/security",
    titleKey: "tabs.routes.security",
    element: <PersonalSettingsPage section="security" />,
  },
  {
    path: "/account/preferences",
    titleKey: "tabs.routes.preferences",
    element: <PersonalSettingsPage section="preferences" />,
  },
  {
    path: "/settings/general",
    titleKey: "tabs.routes.general",
    element: <SystemSettingsPage section="general" />,
  },
  {
    path: "/settings/roles/new",
    tabPath: "/settings/roles",
    titleKey: "tabs.routes.roles",
    element: (
      <SystemSettingsPage section="roles">
        <RoleFormPage mode="create" />
      </SystemSettingsPage>
    ),
  },
  {
    path: "/settings/roles/:roleId",
    tabPath: "/settings/roles",
    titleKey: "tabs.routes.roles",
    element: (
      <SystemSettingsPage section="roles">
        <RoleFormPage mode="detail" />
      </SystemSettingsPage>
    ),
  },
  {
    path: "/settings/roles",
    titleKey: "tabs.routes.roles",
    element: <SystemSettingsPage section="roles" />,
  },
  {
    path: "/settings/storage",
    titleKey: "tabs.routes.storage",
    element: <SystemSettingsPage section="storage" />,
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
  { path: "/apps", titleKey: "tabs.routes.apps", element: <AppsPage /> },
  {
    path: "/integrations/channels/new",
    tabPath: "/integrations/channels",
    titleKey: "tabs.routes.channels",
    element: <MessageChannelFormPage mode="create" />,
  },
  {
    path: "/integrations/channels/:channelType/:channelId",
    tabPath: "/integrations/channels",
    titleKey: "tabs.routes.channels",
    element: <MessageChannelFormPage mode="edit" />,
  },
  {
    path: "/integrations/channels",
    titleKey: "tabs.routes.channels",
    element: <MessageChannelListPage />,
  },
  {
    path: "/integrations/business-systems/new",
    tabPath: "/integrations/business-systems",
    titleKey: "tabs.routes.businessSystems",
    element: <BusinessSystemFormPage mode="create" />,
  },
  {
    path: "/integrations/business-systems/:businessSystemId",
    tabPath: "/integrations/business-systems",
    titleKey: "tabs.routes.businessSystems",
    element: <BusinessSystemFormPage mode="edit" />,
  },
  {
    path: "/integrations/business-systems",
    titleKey: "tabs.routes.businessSystems",
    element: <BusinessSystemListPage />,
  },
  {
    path: "/integrations/mcp-servers/new",
    tabPath: "/integrations/mcp-servers",
    titleKey: "tabs.routes.mcpServers",
    element: <MCPServerFormPage mode="create" />,
  },
  {
    path: "/integrations/mcp-servers/:mcpServerId",
    tabPath: "/integrations/mcp-servers",
    titleKey: "tabs.routes.mcpServers",
    element: <MCPServerFormPage mode="edit" />,
  },
  {
    path: "/integrations/mcp-servers",
    titleKey: "tabs.routes.mcpServers",
    element: <MCPServerListPage />,
  },
  {
    path: "/integrations/model-services/chat/new",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderFormPage mode="create" returnSection="chat" />,
  },
  {
    path: "/integrations/model-services/chat/:providerId",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderFormPage mode="edit" returnSection="chat" />,
  },
  {
    path: "/integrations/model-services/chat",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderListPage section="chat" />,
  },
  {
    path: "/integrations/model-services/embedding/new",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderFormPage mode="create" returnSection="embedding" />,
  },
  {
    path: "/integrations/model-services/embedding/:providerId",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderFormPage mode="edit" returnSection="embedding" />,
  },
  {
    path: "/integrations/model-services/embedding",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderListPage section="embedding" />,
  },
  {
    path: "/integrations/model-services/rerank/new",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderFormPage mode="create" returnSection="rerank" />,
  },
  {
    path: "/integrations/model-services/rerank/:providerId",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderFormPage mode="edit" returnSection="rerank" />,
  },
  {
    path: "/integrations/model-services/rerank",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
    element: <ModelProviderListPage section="rerank" />,
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
  "/account": "/account/profile",
  "/settings": "/settings/general",
  "/contacts": "/contacts/employees",
  "/integrations": "/integrations/channels",
  "/integrations/model-services": "/integrations/model-services/chat",
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
