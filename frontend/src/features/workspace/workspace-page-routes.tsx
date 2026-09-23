/** 定义工作台页面路由。 */
import type { ReactElement } from "react"
import { matchRoutes, useRoutes, type Location, type RouteObject } from "react-router"

import { AgentFormPage } from "@/features/agents/agent-form-page"
import { AgentListPage } from "@/features/agents/agent-list-page"
import { AIPerformancePage } from "@/features/agents/ai-performance-page"
import {
  AgentsModuleLayout,
  agentsModulePaths,
} from "@/features/agents/agents-module-layout"
import { MessageChannelFormPage } from "@/features/channels/message-channel-form-page"
import { MessageChannelListPage } from "@/features/channels/message-channel-list-page"
import { AssistantFormPage } from "@/features/contacts/assistants/assistant-form-page"
import { ContactsPage } from "@/features/contacts/contacts-page"
import { ChatRoute } from "@/features/inbox/chat-route"
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
import { KnowledgeBaseListPage } from "@/features/knowledge-base/knowledge-base-list-page"
import { RoleFormPage } from "@/features/roles/role-form-page"
import { MemberFormPage } from "@/features/settings/members/member-form-page"
import { SettingsPage } from "@/features/settings/settings-page"

/** 需要公共外壳的路由前缀，同一前缀下的页面渲染在对应布局内。 */
const workspaceRouteLayouts = agentsModulePaths.map((prefix) => ({
  prefix,
  element: <AgentsModuleLayout />,
}))

/** 工作台路由清单，地址解析与页面渲染共用同一份定义。 */
const workspaceRouteDefinitions = [
  { path: "/inbox", element: <InboxRoute /> },
  { path: "/chats", element: <ChatRoute /> },
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
    path: "/settings/notifications",
    element: <SettingsPage section="notifications" />,
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
    path: "/settings/customer-service",
    element: <SettingsPage section="customerService" />,
  },
  {
    path: "/settings/members/new",
    element: (
      <SettingsPage section="members">
        <MemberFormPage mode="create" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/members/:userId",
    element: (
      <SettingsPage section="members">
        <MemberFormPage mode="detail" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/members",
    element: <SettingsPage section="members" />,
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
    path: "/contacts/teams",
    element: <ContactsPage scope="team" />,
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
    path: "/contacts/assistants",
    element: <ContactsPage scope="assistants" />,
  },
  {
    path: "/contacts/assistants/new",
    element: (
      <ContactsPage scope="assistants">
        <AssistantFormPage mode="create" />
      </ContactsPage>
    ),
  },
  {
    path: "/contacts/assistants/:assistantId",
    element: (
      <ContactsPage scope="assistants">
        <AssistantFormPage mode="edit" />
      </ContactsPage>
    ),
  },
  {
    path: "/ai-performance",
    element: <AIPerformancePage />,
  },
  {
    path: "/ai-employees",
    element: <AgentListPage />,
  },
  {
    path: "/ai-employees/new",
    element: <AgentFormPage mode="create" />,
  },
  {
    path: "/ai-employees/:agentId",
    element: <AgentFormPage mode="edit" />,
  },
  {
    path: "/channels",
    element: <MessageChannelListPage />,
  },
  {
    path: "/channels/:channelType/new",
    element: <MessageChannelFormPage mode="create" />,
  },
  {
    path: "/channels/:channelType/:channelId",
    element: <MessageChannelFormPage mode="edit" />,
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
    path: "/knowledge-bases/:knowledgeBaseId/qa/new",
    element: <KnowledgeQAFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/qa/:entryId/edit",
    element: <KnowledgeQAFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/qa",
    element: <KnowledgeQAListPage />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/documents/new",
    element: <KnowledgeDocumentFormPage mode="create" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/documents/:documentId/edit",
    element: <KnowledgeDocumentFormPage mode="edit" />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/documents/:documentId",
    element: <KnowledgeDocumentPage />,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/documents",
    element: <KnowledgeDocumentListPage />,
  },
  {
    path: "/knowledge-bases",
    element: <KnowledgeBaseListPage />,
  },
  {
    path: "/tools",
    element: <MCPServerListPage />,
  },
  {
    path: "/tools/new",
    element: <MCPServerFormPage mode="create" />,
  },
  {
    path: "/tools/:mcpServerId",
    element: <MCPServerFormPage mode="edit" />,
  },
  {
    path: "/settings/model-services/new/:brand",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="create" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services/:providerId",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderFormPage mode="edit" />
      </SettingsPage>
    ),
  },
  {
    path: "/settings/model-services",
    element: (
      <SettingsPage section="modelServices">
        <ModelProviderListPage />
      </SettingsPage>
    ),
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
