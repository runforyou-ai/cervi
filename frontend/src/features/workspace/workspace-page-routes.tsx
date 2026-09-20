/** 定义工作台页面路由。 */
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
  { prefix: "/integrations", element: <IntegrationsLayout /> },
]

/** 工作台路由清单，地址解析与页面渲染共用同一份定义。 */
const workspaceRouteDefinitions = [
  { path: "/inbox", element: <InboxRoute /> },
  {
    path: "/settings/profile",
    element: <SettingsPage section="profile" />,
  },
  {
    path: "/settings/security",
    element: <SettingsPage section="security" />,
  },
  {
    path: "/settings/preferences",
    element: <SettingsPage section="preferences" />,
  },
  {
    path: "/settings/devices",
    element: <SettingsPage section="devices" />,
  },
  {
    path: "/settings/general",
    element: <SettingsPage section="general" />,
  },
  {
    path: "/settings/roles/new",
    element: (
      <SettingsPage section="roles">
        <RoleFormPage mode="create" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/roles/:roleId",
    element: (
      <SettingsPage section="roles">
        <RoleFormPage mode="detail" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/roles",
    element: <SettingsPage section="roles" />,
  },
  {
    path: "/contacts/employees",
    element: <ContactsPage scope="employees" />,
  },
  {
    path: "/contacts/ai-employees",
    element: <ContactsPage scope="agents" />,
  },
  {
    path: "/contacts/ai-employees/new",
    element: (
      <ContactsPage scope="agents">
        <AgentFormPage mode="create" />
      </ContactsPage>
    ),
  },
  {
    path: "/contacts/ai-employees/:agentId",
    element: (
      <ContactsPage scope="agents">
        <AgentFormPage mode="edit" />
      </ContactsPage>
    ),
  },
  {
    path: "/contacts/teams/:teamId",
    element: <ContactsPage scope="team" />,
  },
  {
    path: "/contacts/external",
    element: <ContactsPage scope="external" />,
  },
  {
    path: "/knowledge-bases/new",
    element: <KnowledgeBaseFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId",
    element: <KnowledgeBaseFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa/new",
    element: <KnowledgeQAFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa/:entryId/edit",
    element: <KnowledgeQAFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa",
    element: <KnowledgeQAListPage />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents/new",
    element: <KnowledgeDocumentFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents/:documentId/edit",
    element: <KnowledgeDocumentFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents/:documentId",
    element: <KnowledgeDocumentPage />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/documents",
    element: <KnowledgeDocumentListPage />,
  },
  {
    path: "/knowledge-bases",
    element: <KnowledgeBaseIndexPage />,
  },
  { path: "/apps", element: <AppsPage /> },
  {
    path: "/integrations/channels/new",
    element: <MessageChannelFormPage mode="create" />,
  },
  {
    path: "/integrations/channels/:channelType/:channelId",
    element: <MessageChannelFormPage mode="edit" />,
  },
  {
    path: "/integrations/channels",
    element: <MessageChannelListPage />,
  },
  {
    path: "/integrations/business-systems/new",
    element: <BusinessSystemFormPage mode="create" />,
  },
  {
    path: "/integrations/business-systems/:businessSystemId",
    element: <BusinessSystemFormPage mode="edit" />,
  },
  {
    path: "/integrations/business-systems",
    element: <BusinessSystemListPage />,
  },
  {
    path: "/integrations/mcp-servers/new",
    element: <MCPServerFormPage mode="create" />,
  },
  {
    path: "/integrations/mcp-servers/:mcpServerId",
    element: <MCPServerFormPage mode="edit" />,
  },
  {
    path: "/integrations/mcp-servers",
    element: <MCPServerListPage />,
  },
  {
    path: "/integrations/model-services/chat/new",
    element: <ModelProviderFormPage mode="create" returnSection="chat" />,
  },
  {
    path: "/integrations/model-services/chat/:providerId",
    element: <ModelProviderFormPage mode="edit" returnSection="chat" />,
  },
  {
    path: "/integrations/model-services/chat",
    element: <ModelProviderListPage section="chat" />,
  },
  {
    path: "/integrations/model-services/embedding/new",
    element: <ModelProviderFormPage mode="create" returnSection="embedding" />,
  },
  {
    path: "/integrations/model-services/embedding/:providerId",
    element: <ModelProviderFormPage mode="edit" returnSection="embedding" />,
  },
  {
    path: "/integrations/model-services/embedding",
    element: <ModelProviderListPage section="embedding" />,
  },
  {
    path: "/integrations/model-services/rerank/new",
    element: <ModelProviderFormPage mode="create" returnSection="rerank" />,
  },
  {
    path: "/integrations/model-services/rerank/:providerId",
    element: <ModelProviderFormPage mode="edit" returnSection="rerank" />,
  },
  {
    path: "/integrations/model-services/rerank",
    element: <ModelProviderListPage section="rerank" />,
  },
] as const satisfies readonly {
  path: string
  element: ReactElement
}[]

/** 返回路由所属的外壳，无匹配前缀时在顶层渲染。 */
function layoutOf(path: string) {
  return workspaceRouteLayouts.find(
    (layout) => path === layout.prefix || path.startsWith(`${layout.prefix}/`),
  )
}

/**
 * 由清单生成的路由树，地址解析与页面渲染共用。
 * 匹配按 react-router 的路径评分决定，与清单顺序无关。
 */
const workspaceRouteObjects: RouteObject[] = [
  ...workspaceRouteDefinitions
    .filter((definition) => !layoutOf(definition.path))
    .map((definition) => ({
      path: definition.path,
      element: definition.element,
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
        }
      }),
  })),
]

const workspaceRedirects: Readonly<Record<string, string>> = {
  "/settings": "/settings/profile",
  "/contacts": "/contacts/employees",
  "/integrations": "/integrations/channels",
  "/integrations/model-services": "/integrations/model-services/chat",
}

export type ResolvedWorkspaceLocation = {
  canonicalHref: string
  matched: boolean
}

export const defaultWorkspaceHref = "/inbox"

/** 判断地址是否属于设置页。 */
export function isSettingsHref(href: string) {
  return href === "/settings" || href.startsWith("/settings/")
}

/** 把当前地址解析为规范地址；别名地址给出跳转目标，未知地址回到消息页。 */
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
      matched: false,
    }
  }

  if (!matchRoutes(workspaceRouteObjects, pathname)) {
    return { canonicalHref: defaultWorkspaceHref, matched: false }
  }

  return {
    canonicalHref: `${pathname}${location.search}${location.hash}`,
    matched: true,
  }
}

/** 按指定地址渲染一份工作台页面树。 */
export function WorkspacePageRoutes({ location }: { location: string }) {
  return useRoutes(workspaceRouteObjects, location)
}
