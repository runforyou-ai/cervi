/** 定义可作为工作台标签长期挂载的页面路由。 */
import { Route, Routes, matchPath, type Location } from "react-router"

import { MessageChannelFormPage } from "@/features/channels/message-channel-form-page"
import { MessageChannelListPage } from "@/features/channels/message-channel-list-page"
import { ContactsPage } from "@/features/contacts/contacts-page"
import { InboxRoute } from "@/features/inbox/inbox-route"
import { IntegrationsLayout } from "@/features/integrations/integrations-layout"
import { ModelProviderFormPage } from "@/features/integrations/model-services/model-provider-form-page"
import { ModelProviderListPage } from "@/features/integrations/model-services/model-provider-list-page"
import { KnowledgeBaseFormPage } from "@/features/knowledge-base/knowledge-base-form-page"
import { KnowledgeBaseIndexPage } from "@/features/knowledge-base/knowledge-base-index-page"
import { KnowledgeBaseLayout } from "@/features/knowledge-base/knowledge-base-layout"
import { RoleFormPage } from "@/features/settings/role-form-page"
import {
  PersonalSettingsPage,
  SystemSettingsPage,
} from "@/features/settings/settings-page"

const workspaceRouteDefinitions = [
  { path: "/inbox", titleKey: "tabs.routes.inbox" },
  { path: "/account/profile", titleKey: "tabs.routes.profile" },
  { path: "/account/security", titleKey: "tabs.routes.security" },
  { path: "/account/preferences", titleKey: "tabs.routes.preferences" },
  {
    path: "/settings/organization",
    titleKey: "tabs.routes.organization",
  },
  {
    path: "/settings/roles/new",
    titleKey: "tabs.routes.newRole",
    transient: true,
  },
  { path: "/settings/roles/:roleId", titleKey: "tabs.routes.role" },
  { path: "/settings/roles", titleKey: "tabs.routes.roles" },
  { path: "/settings/storage", titleKey: "tabs.routes.storage" },
  { path: "/contacts/employees", titleKey: "tabs.routes.employees" },
  { path: "/contacts/ai-employees", titleKey: "tabs.routes.aiEmployees" },
  { path: "/contacts/teams/:teamId", titleKey: "tabs.routes.team" },
  { path: "/contacts/external", titleKey: "tabs.routes.externalContacts" },
  {
    path: "/knowledge-bases/new",
    titleKey: "tabs.routes.newKnowledgeBase",
    transient: true,
  },
  {
    path: "/knowledge-bases/:knowledgeBaseId",
    titleKey: "tabs.routes.knowledgeBase",
  },
  { path: "/knowledge-bases", titleKey: "tabs.routes.knowledgeBases" },
  {
    path: "/integrations/channels/new",
    titleKey: "tabs.routes.newChannel",
    transient: true,
  },
  {
    path: "/integrations/channels/:channelType/:channelId",
    titleKey: "tabs.routes.channel",
  },
  { path: "/integrations/channels", titleKey: "tabs.routes.channels" },
  {
    path: "/integrations/model-services/chat/new",
    titleKey: "tabs.routes.newChatProvider",
    transient: true,
  },
  {
    path: "/integrations/model-services/chat/:providerId",
    titleKey: "tabs.routes.chatProvider",
  },
  {
    path: "/integrations/model-services/chat",
    titleKey: "tabs.routes.chatProviders",
  },
  {
    path: "/integrations/model-services/embedding/new",
    titleKey: "tabs.routes.newEmbeddingProvider",
    transient: true,
  },
  {
    path: "/integrations/model-services/embedding/:providerId",
    titleKey: "tabs.routes.embeddingProvider",
  },
  {
    path: "/integrations/model-services/embedding",
    titleKey: "tabs.routes.embeddingProviders",
  },
  {
    path: "/integrations/model-services/rerank/new",
    titleKey: "tabs.routes.newRerankProvider",
    transient: true,
  },
  {
    path: "/integrations/model-services/rerank/:providerId",
    titleKey: "tabs.routes.rerankProvider",
  },
  {
    path: "/integrations/model-services/rerank",
    titleKey: "tabs.routes.rerankProviders",
  },
] as const

const workspaceRedirects: Readonly<Record<string, string>> = {
  "/account": "/account/profile",
  "/settings": "/settings/organization",
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
  transient: boolean
}

export type ResolvedWorkspaceLocation = {
  canonicalHref: string
  tab: ResolvedWorkspaceTab | null
}

export const defaultWorkspaceTab = {
  id: "/inbox",
  href: "/inbox",
  titleKey: "tabs.routes.inbox",
  transient: false,
} satisfies ResolvedWorkspaceTab

/** 规范化工作台路径，避免同一页面生成重复标签。 */
function normalizePathname(pathname: string) {
  if (pathname === "/") {
    return pathname
  }
  return pathname.replace(/\/+$/, "") || "/"
}

/** 把当前地址解析为规范标签；未知地址回到消息页。 */
export function resolveWorkspaceLocation(
  location: Pick<Location, "pathname" | "search" | "hash">,
): ResolvedWorkspaceLocation {
  const pathname = normalizePathname(location.pathname)
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
      id: pathname,
      href,
      titleKey: definition.titleKey,
      transient: "transient" in definition && definition.transient,
    },
  }
}

/** 按标签保存的独立地址渲染一份工作台页面树。 */
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
        path="/settings/organization"
        element={<SystemSettingsPage section="organization" />}
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
      <Route path="/knowledge-bases" element={<KnowledgeBaseLayout />}>
        <Route index element={<KnowledgeBaseIndexPage />} />
        <Route
          path="new"
          element={<KnowledgeBaseFormPage mode="create" />}
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
      </Route>
    </Routes>
  )
}
