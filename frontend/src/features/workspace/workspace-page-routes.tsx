/** 定义工作台页面路由和多标签分组。 */
import { Route, Routes, matchPath, type Location } from "react-router"

import { MessageChannelFormPage } from "@/features/channels/message-channel-form-page"
import { MessageChannelListPage } from "@/features/channels/message-channel-list-page"
import { ContactsPage } from "@/features/contacts/contacts-page"
import { InboxRoute } from "@/features/inbox/inbox-route"
import { AppsPage } from "@/features/apps/apps-page"
import { IntegrationsLayout } from "@/features/integrations/integrations-layout"
import { BusinessSystemFormPage } from "@/features/integrations/business-systems/business-system-form-page"
import { BusinessSystemListPage } from "@/features/integrations/business-systems/business-system-list-page"
import { ConnectorFormPage } from "@/features/integrations/connectors/connector-form-page"
import { ConnectorListPage } from "@/features/integrations/connectors/connector-list-page"
import { ModelProviderFormPage } from "@/features/integrations/model-services/model-provider-form-page"
import { ModelProviderListPage } from "@/features/integrations/model-services/model-provider-list-page"
import { KnowledgeQAListPage } from "@/features/knowledge-base/knowledge-qa-list-page"
import { KnowledgeQAFormPage } from "@/features/knowledge-base/knowledge-qa-form-page"
import { KnowledgeBaseFormPage } from "@/features/knowledge-base/knowledge-base-form-page"
import { KnowledgeBaseIndexPage } from "@/features/knowledge-base/knowledge-base-index-page"
import { KnowledgeBaseLayout } from "@/features/knowledge-base/knowledge-base-layout"
import { KnowledgeDocumentListPage } from "@/features/knowledge-base/knowledge-document-list-page"
import { KnowledgeDocumentPage } from "@/features/knowledge-base/knowledge-document-page"
import { RoleFormPage } from "@/features/roles/role-form-page"
import {
  PersonalSettingsPage,
  SystemSettingsPage,
} from "@/features/settings/settings-page"

/** 可标签化的工作台路由；tabPath 把同一功能模块的页面归入同一个标签。 */
const workspaceRouteDefinitions = [
  { path: "/inbox", titleKey: "tabs.routes.inbox" },
  { path: "/account/profile", titleKey: "tabs.routes.profile" },
  { path: "/account/security", titleKey: "tabs.routes.security" },
  { path: "/account/preferences", titleKey: "tabs.routes.preferences" },
  {
    path: "/settings/general",
    titleKey: "tabs.routes.general",
  },
  {
    path: "/settings/roles/new",
    tabPath: "/settings/roles",
    titleKey: "tabs.routes.roles",
  },
  {
    path: "/settings/roles/:roleId",
    tabPath: "/settings/roles",
    titleKey: "tabs.routes.roles",
  },
  { path: "/settings/roles", titleKey: "tabs.routes.roles" },
  { path: "/settings/storage", titleKey: "tabs.routes.storage" },
  { path: "/contacts/employees", titleKey: "tabs.routes.employees" },
  { path: "/contacts/ai-employees", titleKey: "tabs.routes.aiEmployees" },
  { path: "/contacts/teams/:teamId", titleKey: "tabs.routes.team" },
  { path: "/contacts/external", titleKey: "tabs.routes.externalContacts" },
  {
    path: "/knowledge-bases/new",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/documents/:documentId",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/documents",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa/new",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa/:entryId/edit",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId/groups/:groupId/qa",
    tabPath: "/knowledge-bases",
    titleKey: "tabs.routes.knowledgeBases",
  },
  { path: "/knowledge-bases", titleKey: "tabs.routes.knowledgeBases" },
  { path: "/apps", titleKey: "tabs.routes.apps" },
  {
    path: "/integrations/channels/new",
    tabPath: "/integrations/channels",
    titleKey: "tabs.routes.channels",
  },
  {
    path: "/integrations/channels/:channelType/:channelId",
    tabPath: "/integrations/channels",
    titleKey: "tabs.routes.channels",
  },
  { path: "/integrations/channels", titleKey: "tabs.routes.channels" },
  {
    path: "/integrations/business-systems/new",
    tabPath: "/integrations/business-systems",
    titleKey: "tabs.routes.businessSystems",
  },
  {
    path: "/integrations/business-systems/:businessSystemId",
    tabPath: "/integrations/business-systems",
    titleKey: "tabs.routes.businessSystems",
  },
  {
    path: "/integrations/business-systems",
    titleKey: "tabs.routes.businessSystems",
  },
  {
    path: "/integrations/model-services/chat/new",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/chat/:providerId",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/chat",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/embedding/new",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/embedding/:providerId",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/embedding",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/rerank/new",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/rerank/:providerId",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/model-services/rerank",
    tabPath: "/integrations/model-services",
    titleKey: "tabs.routes.modelServices",
  },
  {
    path: "/integrations/connectors/new",
    tabPath: "/integrations/connectors",
    titleKey: "tabs.routes.connectors",
  },
  {
    path: "/integrations/connectors/:connectionId",
    tabPath: "/integrations/connectors",
    titleKey: "tabs.routes.connectors",
  },
  {
    path: "/integrations/connectors",
    titleKey: "tabs.routes.connectors",
  },
] as const

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

/** 把当前地址解析为规范标签；未知地址回到消息页。 */
export function resolveWorkspaceLocation(
  location: Pick<Location, "pathname" | "search" | "hash">,
): ResolvedWorkspaceLocation {
  // 规范化工作台路径，避免同一页面生成重复标签。
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

  const definition = workspaceRouteDefinitions.find(({ path }) =>
    matchPath({ path, end: true }, pathname),
  )
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
  return (
    <Routes location={location}>
      <Route path="/inbox" element={<InboxRoute />} />
      <Route
        path="/account/profile"
        element={<PersonalSettingsPage section="profile" />}
      />
      <Route
        path="/account/security"
        element={<PersonalSettingsPage section="security" />}
      />
      <Route
        path="/account/preferences"
        element={<PersonalSettingsPage section="preferences" />}
      />
      <Route
        path="/settings/general"
        element={<SystemSettingsPage section="general" />}
      />
      <Route
        path="/settings/roles"
        element={<SystemSettingsPage section="roles" />}
      />
      <Route
        path="/settings/roles/new"
        element={
          <SystemSettingsPage section="roles">
            <RoleFormPage mode="create" />
          </SystemSettingsPage>
        }
      />
      <Route
        path="/settings/roles/:roleId"
        element={
          <SystemSettingsPage section="roles">
            <RoleFormPage mode="detail" />
          </SystemSettingsPage>
        }
      />
      <Route
        path="/settings/storage"
        element={<SystemSettingsPage section="storage" />}
      />
      <Route
        path="/contacts/employees"
        element={<ContactsPage scope="employees" />}
      />
      <Route
        path="/contacts/ai-employees"
        element={<ContactsPage scope="agents" />}
      />
      <Route
        path="/contacts/teams/:teamId"
        element={<ContactsPage scope="team" />}
      />
      <Route
        path="/contacts/external"
        element={<ContactsPage scope="external" />}
      />
      <Route path="/apps" element={<AppsPage />} />
      <Route path="/knowledge-bases" element={<KnowledgeBaseLayout />}>
        <Route index element={<KnowledgeBaseIndexPage />} />
        <Route
          path=":knowledgeBaseId/groups/:groupId/qa"
          element={<KnowledgeQAListPage />}
        />
        <Route
          path=":knowledgeBaseId/groups/:groupId/qa/new"
          element={<KnowledgeQAFormPage mode="create" />}
        />
        <Route
          path=":knowledgeBaseId/groups/:groupId/qa/:entryId/edit"
          element={<KnowledgeQAFormPage mode="edit" />}
        />
        <Route path="new" element={<KnowledgeBaseFormPage mode="create" />} />
        <Route
          path=":knowledgeBaseId/documents/:documentId"
          element={<KnowledgeDocumentPage />}
        />
        <Route
          path=":knowledgeBaseId/documents"
          element={<KnowledgeDocumentListPage />}
        />
        <Route
          path=":knowledgeBaseId"
          element={<KnowledgeBaseFormPage mode="edit" />}
        />
      </Route>
      <Route path="/integrations" element={<IntegrationsLayout />}>
        <Route path="channels" element={<MessageChannelListPage />} />
        <Route
          path="channels/new"
          element={<MessageChannelFormPage mode="create" />}
        />
        <Route
          path="channels/:channelType/:channelId"
          element={<MessageChannelFormPage mode="edit" />}
        />
        <Route
          path="business-systems"
          element={<BusinessSystemListPage />}
        />
        <Route
          path="business-systems/new"
          element={<BusinessSystemFormPage mode="create" />}
        />
        <Route
          path="business-systems/:businessSystemId"
          element={<BusinessSystemFormPage mode="edit" />}
        />
        <Route
          path="model-services/chat"
          element={<ModelProviderListPage section="chat" />}
        />
        <Route
          path="model-services/chat/new"
          element={
            <ModelProviderFormPage mode="create" returnSection="chat" />
          }
        />
        <Route
          path="model-services/chat/:providerId"
          element={
            <ModelProviderFormPage mode="edit" returnSection="chat" />
          }
        />
        <Route
          path="model-services/embedding"
          element={<ModelProviderListPage section="embedding" />}
        />
        <Route
          path="model-services/embedding/new"
          element={
            <ModelProviderFormPage mode="create" returnSection="embedding" />
          }
        />
        <Route
          path="model-services/embedding/:providerId"
          element={
            <ModelProviderFormPage mode="edit" returnSection="embedding" />
          }
        />
        <Route
          path="model-services/rerank"
          element={<ModelProviderListPage section="rerank" />}
        />
        <Route
          path="model-services/rerank/new"
          element={
            <ModelProviderFormPage mode="create" returnSection="rerank" />
          }
        />
        <Route
          path="model-services/rerank/:providerId"
          element={
            <ModelProviderFormPage mode="edit" returnSection="rerank" />
          }
        />
        <Route path="connectors" element={<ConnectorListPage />} />
        <Route
          path="connectors/new"
          element={<ConnectorFormPage mode="create" />}
        />
        <Route
          path="connectors/:connectionId"
          element={<ConnectorFormPage mode="edit" />}
        />
      </Route>
    </Routes>
  )
}
