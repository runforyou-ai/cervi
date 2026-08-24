/** 绑定应用服务方法，并归一化可空切片。 */
import {
  ActivateWebsiteChannel,
  AddTeamMembers,
  ChangePassword,
  CheckNotificationPermission,
  CompleteFileUpload,
  CreateAgent,
  CreateContact,
  CreateFileUpload,
  CreateAIProvider,
  CreateRole,
  CreateTeam,
  CreateUser,
  CreateWebsiteChannel,
  DeactivateAgent,
  DeactivateUser,
  DeactivateWebsiteChannel,
  DeleteContact,
  DeleteAIProvider,
  DeleteRole,
  DeleteTeam,
  GetContact,
  GetAIProvider,
  GetAgent,
  GetRole,
  GetS3Setting,
  GetUser,
  GetWebsiteChannel,
  ListChannels,
  ListAgents,
  ListAgentModelOptions,
  ListMemberOptions,
  ListAIProviders,
  ListAvailableAIModels,
  ListContacts,
  ListRoles,
  ListTeams,
  ListTeamMemberCandidates,
  ListTeamMembers,
  ListUsers,
  ListWebsiteChannels,
  LoadIdentity,
  LoadInbox,
  ReactivateUser,
  ReactivateAgent,
  RemoveTeamMembers,
  RequestNotificationPermission,
  RestoreContact,
  SaveS3Setting,
  SelectProfileImage,
  SendMessageNotification,
  TestS3Setting,
  UpdateContact,
  UpdateAIProvider,
  UpdateAgent,
  UpdateAgentCapability,
  UpdateAgentWorkStatus,
  UpdateTeam,
  UpdateUser,
  UpdateUserRoles,
  UpdateOrganization,
  UpdateProfile,
  UpdateRole,
  UpdateUnreadIndicator,
  UpdateUserPreferences,
  UpdateUserWorkStatus,
  UpdateWebsiteChannel,
  UpdateWebsiteChannelAccess,
  UpdateWebsiteChannelChatInterface,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  AIModelInputModality,
  AIModelType,
  AIProviderBrand,
  ContactSort,
  StorageProvider,
  type AIProvider,
  type AIProviderInput,
  type AIProviderList,
  type AIProviderModel,
  type AIProviderModelList,
  type AIProviderSummary,
  type AgentList,
  type AgentListItem,
  type AgentListInput,
  type AgentCapabilityInput,
  type Contact,
  type ContactInput,
  type ContactList,
  type ContactListInput,
  type Conversation as GeneratedConversation,
  type CreateAgentInput,
  type CreateUserInput,
  type User,
  type Agent,
  type Inbox,
  type MemberOption,
  type MemberOptionList,
  type MemberOptionListInput,
  type PermissionDefinition,
  type Role,
  type RoleInput,
  type RoleList,
  type TeamListInput,
  type TeamMember,
  type TeamMemberListInput,
  type TeamMemberList,
  type TeamMemberCandidate,
  type TeamMemberCandidateInput,
  type TeamMemberCandidateList,
  type AgentWorkStatusInput,
  type UpdateAgentInput,
  type UpdateUserInput,
  type UserList,
  type UserListInput,
  type WebsiteChannel as GeneratedWebsiteChannel,
  type WebsiteChannelAccess as GeneratedWebsiteChannelAccess,
  type WebsiteChannelAccessInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"

export * from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

export type StorageProviderId = Exclude<StorageProvider, StorageProvider.$zero>

export type AIProviderBrandId = Exclude<AIProviderBrand, AIProviderBrand.$zero>

export type AIModelTypeId = Exclude<AIModelType, AIModelType.$zero>

export type AIModelInputModalityId = Exclude<
  AIModelInputModality,
  AIModelInputModality.$zero
>

export type AIProviderModelData = Omit<
  AIProviderModel,
  "type" | "inputModalities"
> & {
  type: AIModelTypeId
  inputModalities: AIModelInputModalityId[]
}

export type AIProviderData = Omit<AIProvider, "brand" | "models"> & {
  brand: AIProviderBrandId
  models: AIProviderModelData[]
}

export type AIProviderSummaryData = Omit<
  AIProviderSummary,
  "brand" | "modelTypes"
> & {
  brand: AIProviderBrandId
  modelTypes: AIModelTypeId[]
}

export type AIProviderListData = Omit<AIProviderList, "providers"> & {
  providers: AIProviderSummaryData[]
}

export type ContactDetail = Omit<Contact, "methods" | "channelIdentities"> & {
  methods: NonNullable<Contact["methods"]>
  channelIdentities: NonNullable<Contact["channelIdentities"]>
}

export type ContactListResponse = Omit<ContactList, "contacts"> & {
  contacts: NonNullable<ContactList["contacts"]>
}

export type ContactListQuery = Omit<Partial<ContactListInput>, "deleted">

export type UserListQuery = Partial<UserListInput>

export type AgentListQuery = Partial<AgentListInput>

export type TeamListQuery = Partial<TeamListInput>

export type TeamMemberCandidateQuery = Partial<TeamMemberCandidateInput>

export type TeamMemberListQuery = Partial<TeamMemberListInput>

export type MemberOptionListQuery = Partial<MemberOptionListInput>

export type MemberOptionListData = Omit<MemberOptionList, "members"> & {
  members: MemberOption[]
}

export type AgentData = Omit<Agent, "teams"> & {
  teams: NonNullable<Agent["teams"]>
}

export type AgentListItemData = Omit<AgentListItem, "teams"> & {
  teams: NonNullable<AgentListItem["teams"]>
}

export type AgentListData = Omit<AgentList, "agents"> & {
  agents: AgentListItemData[]
}

export type TeamMemberListData = Omit<
  TeamMemberList,
  "members"
> & {
  members: TeamMember[]
}

export type TeamMemberCandidateListData = Omit<
  TeamMemberCandidateList,
  "members"
> & {
  members: TeamMemberCandidate[]
}

export type UserData = Omit<User, "teams"> & {
  teams: NonNullable<User["teams"]>
}

export type UserListResponse = Omit<UserList, "users"> & {
  users: UserData[]
}

export type ConversationData = Omit<GeneratedConversation, "messages"> & {
  messages: NonNullable<GeneratedConversation["messages"]>
}

export type InboxData = Omit<Inbox, "conversations"> & {
  conversations: ConversationData[]
}

export type Conversation = ConversationData

export type RoleData = Omit<Role, "permissions"> & {
  permissions: NonNullable<Role["permissions"]>
}

export type RoleListData = Omit<RoleList, "roles" | "permissions"> & {
  roles: RoleData[]
  permissions: PermissionDefinition[]
}

export type WebsiteChannelAccessData = Omit<
  GeneratedWebsiteChannelAccess,
  "allowedHosts"
> & {
  allowedHosts: string[]
}

export type WebsiteChannelData = Omit<GeneratedWebsiteChannel, "access"> & {
  access: WebsiteChannelAccessData
}

/** 读取当前登录身份。 */
export const loadIdentity = bind(LoadIdentity)

const getWebsiteChannelBound = bind(GetWebsiteChannel)
/** 读取网站渠道详情。 */
export function getWebsiteChannel(channelId: string) {
  return getWebsiteChannelBound(channelId).then(normalizeWebsiteChannel)
}
/** 创建网站渠道。 */
export const createWebsiteChannel = bind(CreateWebsiteChannel)
/** 修改网站渠道基础信息。 */
export const updateWebsiteChannel = bind(UpdateWebsiteChannel)
/** 修改网站渠道聊天界面。 */
export const updateWebsiteChannelChatInterface = bind(
  UpdateWebsiteChannelChatInterface,
)
const updateWebsiteChannelAccessBound = bind(UpdateWebsiteChannelAccess)
/** 修改网站渠道允许使用的网站。 */
export function updateWebsiteChannelAccess(
  channelId: string,
  input: WebsiteChannelAccessInput,
) {
  return updateWebsiteChannelAccessBound(channelId, input).then(
    normalizeWebsiteChannelAccess,
  )
}
/** 停用网站渠道。 */
export const deactivateWebsiteChannel = bind(DeactivateWebsiteChannel)
/** 启用网站渠道。 */
export const activateWebsiteChannel = bind(ActivateWebsiteChannel)
/** 将联系人移入回收站。 */
export const deleteContact = bind(DeleteContact)
/** 创建企业团队。 */
export const createTeam = bind(CreateTeam)
/** 修改企业团队。 */
export const updateTeam = bind(UpdateTeam)
/** 删除企业团队。 */
export const deleteTeam = bind(DeleteTeam)
/** 将企业身份批量加入团队。 */
export const addTeamMembers = bind(AddTeamMembers)
/** 将企业身份批量移出团队。 */
export const removeTeamMembers = bind(RemoveTeamMembers)
/** 读取对象存储设置。 */
export const getS3Setting = bind(GetS3Setting)
/** 修改当前企业名称。 */
export const updateOrganization = bind(UpdateOrganization)
/** 保存对象存储设置。 */
export const saveS3Setting = bind(SaveS3Setting)
/** 测试对象存储连接。 */
export const testS3Setting = bind(TestS3Setting)
/** 修改当前用户的头像、姓名和邮箱。 */
export const updateProfile = bind(UpdateProfile)
/** 创建文件上传请求。 */
export const createFileUpload = bind(CreateFileUpload)
/** 核验并完成文件上传。 */
export const completeFileUpload = bind(CompleteFileUpload)
/** 使用原生文件对话框选择用户头像图片。 */
export const selectProfileImage = bind(SelectProfileImage)
/** 修改当前用户的登录密码。 */
export const changePassword = bind(ChangePassword)
/** 修改当前用户的语言、时区和账号级通知设置。 */
export const updateUserPreferences = bind(UpdateUserPreferences)
/** 读取当前设备的通知权限状态。 */
export const checkNotificationPermission = bind(CheckNotificationPermission)
/** 申请当前设备的通知权限。 */
export const requestNotificationPermission = bind(
  RequestNotificationPermission,
)
/** 投递一条桌面新消息通知。 */
export const sendNativeMessageNotification = bind(SendMessageNotification)
/** 同步当前设备的未读数和托盘提醒状态。 */
export const updateUnreadIndicator = bind(UpdateUnreadIndicator)
/** 修改当前用户主动设置的工作状态。 */
export const updateUserWorkStatus = bind(UpdateUserWorkStatus)
/** 在一个事务中批量调整企业成员角色。 */
export const updateUserRoles = bind(UpdateUserRoles)
/** 删除自定义角色。 */
export const deleteRole = bind(DeleteRole)

const listAIProvidersBound = bind(ListAIProviders)
const getAIProviderBound = bind(GetAIProvider)
const listAvailableAIModelsBound = bind(ListAvailableAIModels)
const createAIProviderBound = bind(CreateAIProvider)
const updateAIProviderBound = bind(UpdateAIProvider)

/** 读取当前企业的模型服务供应商列表。 */
export function listAIProviders() {
  return listAIProvidersBound().then(
    (output): AIProviderListData => ({
      ...output,
      providers: asList(output.providers).map(normalizeAIProviderSummary),
    }),
  )
}

/** 读取模型服务供应商详情。 */
export function getAIProvider(providerId: string) {
  return getAIProviderBound(providerId).then(normalizeAIProvider)
}

/** 读取指定品牌的预设模型目录。 */
export function listAvailableAIModels(brand: AIProviderBrand) {
  return listAvailableAIModelsBound(brand).then((output: AIProviderModelList) =>
    asList(output.models).map(normalizeAIProviderModel),
  )
}

/** 创建模型服务供应商。 */
export function createAIProvider(input: AIProviderInput) {
  return createAIProviderBound(input).then(normalizeAIProvider)
}

/** 修改模型服务供应商。 */
export function updateAIProvider(providerId: string, input: AIProviderInput) {
  return updateAIProviderBound(providerId, input).then(normalizeAIProvider)
}

/** 删除模型服务供应商。 */
export const deleteAIProvider = bind(DeleteAIProvider)

const listChannelsBound = bind(ListChannels)
const listWebsiteChannelsBound = bind(ListWebsiteChannels)
const listUsersBound = bind(ListUsers)
const listAgentsBound = bind(ListAgents)
const listAgentModelOptionsBound = bind(ListAgentModelOptions)
const getAgentBound = bind(GetAgent)
const updateAgentBound = bind(UpdateAgent)
const updateAgentCapabilityBound = bind(UpdateAgentCapability)
const updateAgentWorkStatusBound = bind(UpdateAgentWorkStatus)
const deactivateAgentBound = bind(DeactivateAgent)
const reactivateAgentBound = bind(ReactivateAgent)
const listTeamsBound = bind(ListTeams)
const listMemberOptionsBound = bind(ListMemberOptions)
const listTeamMemberCandidatesBound = bind(ListTeamMemberCandidates)
const listTeamMembersBound = bind(ListTeamMembers)
const getUserBound = bind(GetUser)
const createAgentBound = bind(CreateAgent)
const createUserBound = bind(CreateUser)
const updateUserBound = bind(UpdateUser)
const deactivateUserBound = bind(DeactivateUser)
const reactivateUserBound = bind(ReactivateUser)
const listContactsBound = bind(ListContacts)
const getContactBound = bind(GetContact)
const createContactBound = bind(CreateContact)
const updateContactBound = bind(UpdateContact)
const restoreContactBound = bind(RestoreContact)
const loadInboxBound = bind(LoadInbox)
const listRolesBound = bind(ListRoles)
const getRoleBound = bind(GetRole)
const createRoleBound = bind(CreateRole)
const updateRoleBound = bind(UpdateRole)

/** 把可空切片转换为空数组。 */
function asList<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}

/** 归一化网站渠道允许使用的网站。 */
function normalizeWebsiteChannelAccess(
  access: GeneratedWebsiteChannelAccess,
): WebsiteChannelAccessData {
  return { ...access, allowedHosts: asList(access.allowedHosts) }
}

/** 归一化网站渠道详情。 */
function normalizeWebsiteChannel(
  channel: GeneratedWebsiteChannel,
): WebsiteChannelData {
  return { ...channel, access: normalizeWebsiteChannelAccess(channel.access) }
}

/** 归一化模型服务供应商详情。 */
function normalizeAIProvider(provider: AIProvider): AIProviderData {
  return {
    ...provider,
    brand: provider.brand as AIProviderBrandId,
    models: asList(provider.models).map(normalizeAIProviderModel),
  }
}

/** 归一化模型服务供应商列表项。 */
function normalizeAIProviderSummary(
  provider: AIProviderSummary,
): AIProviderSummaryData {
  return {
    ...provider,
    brand: provider.brand as AIProviderBrandId,
    modelTypes: asList(provider.modelTypes) as AIModelTypeId[],
  }
}

/** 归一化模型目录项。 */
function normalizeAIProviderModel(model: AIProviderModel): AIProviderModelData {
  return {
    ...model,
    type: model.type as AIModelTypeId,
    inputModalities: asList(
      model.inputModalities,
    ) as AIModelInputModalityId[],
  }
}

/** 读取尚未加入团队的企业成员。 */
export function listTeamMemberCandidates(
  teamId: string,
  query: TeamMemberCandidateQuery = {},
  signal?: AbortSignal,
): Promise<TeamMemberCandidateListData> {
  return listTeamMemberCandidatesBound(
    teamId,
    {
      query: query.query ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({
    ...output,
    members: asList(output.members),
  }))
}

/** 归一化企业成员所属团队。 */
function normalizeUser(user: User): UserData {
  return { ...user, teams: asList(user.teams) }
}

/** 归一化 AI 员工所属团队。 */
function normalizeAgent(agent: Agent): AgentData {
  return { ...agent, teams: asList(agent.teams) }
}

/** 归一化 AI 员工目录项所属团队。 */
function normalizeAgentListItem(agent: AgentListItem): AgentListItemData {
  return { ...agent, teams: asList(agent.teams) }
}

/** 读取企业成员详情。 */
export function getUser(userId: string, signal?: AbortSignal) {
  return getUserBound(userId, signal).then(normalizeUser)
}

/** 创建企业成员账号。 */
export function createUser(input: CreateUserInput) {
  return createUserBound(input).then(normalizeUser)
}

/** 创建企业 AI 员工。 */
export function createAgent(input: CreateAgentInput) {
  return createAgentBound(input).then(normalizeAgent)
}

/** 读取企业 AI 员工可使用的对话模型。 */
export function listAgentModelOptions() {
  return listAgentModelOptionsBound().then((output) => asList(output.models))
}

/** 读取企业 AI 员工详情。 */
export function getAgent(agentId: string, signal?: AbortSignal) {
  return getAgentBound(agentId, signal).then(normalizeAgent)
}

/** 修改企业 AI 员工。 */
export function updateAgent(agentId: string, input: UpdateAgentInput) {
  return updateAgentBound(agentId, input).then(normalizeAgent)
}

/** 修改企业 AI 员工的能力配置。 */
export function updateAgentCapability(
  agentId: string,
  input: AgentCapabilityInput,
) {
  return updateAgentCapabilityBound(agentId, input).then(normalizeAgent)
}

/** 修改企业 AI 员工的工作状态。 */
export function updateAgentWorkStatus(
  agentId: string,
  input: AgentWorkStatusInput,
) {
  return updateAgentWorkStatusBound(agentId, input).then(normalizeAgent)
}

/** 禁用企业 AI 员工账号。 */
export function deactivateAgent(agentId: string) {
  return deactivateAgentBound(agentId).then(normalizeAgent)
}

/** 将企业 AI 员工恢复为正常状态。 */
export function reactivateAgent(agentId: string) {
  return reactivateAgentBound(agentId).then(normalizeAgent)
}

/** 修改企业成员资料、角色和所属团队。 */
export function updateUser(userId: string, input: UpdateUserInput) {
  return updateUserBound(userId, input).then(normalizeUser)
}

/** 禁用企业成员账号。 */
export function deactivateUser(userId: string) {
  return deactivateUserBound(userId).then(normalizeUser)
}

/** 将企业成员账号恢复为正常状态。 */
export function reactivateUser(userId: string) {
  return reactivateUserBound(userId).then(normalizeUser)
}

/** 读取当前企业的渠道选择项。 */
export function listChannels(signal?: AbortSignal) {
  return listChannelsBound(signal).then((list) => asList(list.channels))
}

/** 读取网站渠道列表。 */
export function listWebsiteChannels() {
  return listWebsiteChannelsBound().then((list) => asList(list.channels))
}

/** 读取联系人详情。 */
export function getContact(contactId: string, signal?: AbortSignal) {
  return getContactBound(contactId, signal).then(normalizeContact)
}

/** 创建联系人。 */
export function createContact(input: ContactInput) {
  return createContactBound(input).then(normalizeContact)
}

/** 修改联系人。 */
export function updateContact(contactId: string, input: ContactInput) {
  return updateContactBound(contactId, input).then(normalizeContact)
}

/** 恢复联系人。 */
export function restoreContact(contactId: string) {
  return restoreContactBound(contactId).then(normalizeContact)
}

/** 读取联系人列表。 */
export function listContacts(query: ContactListQuery, signal?: AbortSignal) {
  return listContactsByDeleted(query, false, signal)
}

/** 读取已删除的联系人列表。 */
export function listDeletedContacts(
  query: ContactListQuery,
  signal?: AbortSignal,
) {
  return listContactsByDeleted(query, true, signal)
}

/** 读取企业成员列表。 */
export function listUsers(query: UserListQuery, signal?: AbortSignal) {
  return listUsersBound(
    {
      query: query.query ?? "",
      status: query.status ?? null,
      roleId: query.roleId ?? "",
      teamId: query.teamId ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({
    ...output,
    users: asList(output.users).map(normalizeUser),
  }))
}

/** 读取企业 AI 员工目录。 */
export function listAgents(query: AgentListQuery, signal?: AbortSignal) {
  return listAgentsBound(
    {
      query: query.query ?? "",
      status: query.status ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({
    ...output,
    agents: asList(output.agents).map(normalizeAgentListItem),
  }))
}

/** 读取团队成员列表。 */
export function listTeamMembers(
  teamId: string,
  query: TeamMemberListQuery,
  signal?: AbortSignal,
) {
  return listTeamMembersBound(
    teamId,
    {
      query: query.query ?? "",
      workStatus: query.workStatus ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({
    ...output,
    members: asList(output.members),
  }))
}

/** 读取企业团队列表。 */
export function listTeams(query: TeamListQuery = {}, signal?: AbortSignal) {
  return listTeamsBound(
    {
      query: query.query ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, teams: asList(output.teams) }))
}

/** 读取可分配的企业成员和 AI 员工。 */
export function listMemberOptions(
  query: MemberOptionListQuery = {},
  signal?: AbortSignal,
): Promise<MemberOptionListData> {
  return listMemberOptionsBound(
    {
      query: query.query ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, members: asList(output.members) }))
}

/** 读取统一收件箱。 */
export async function loadInbox(): Promise<InboxData> {
  const inbox = await loadInboxBound()
  return {
    ...inbox,
    conversations: asList(inbox.conversations).map((conversation) => ({
      ...conversation,
      messages: asList(conversation.messages),
    })),
  }
}

/** 读取角色、数量上限和权限目录。 */
export function listRoles(signal?: AbortSignal): Promise<RoleListData> {
  return listRolesBound(signal).then((output) => ({
    ...output,
    roles: asList(output.roles).map(normalizeRole),
    permissions: asList(output.permissions),
  }))
}

/** 读取角色详情。 */
export function getRole(roleId: string, signal?: AbortSignal) {
  return getRoleBound(roleId, signal).then(normalizeRole)
}

/** 创建自定义角色。 */
export function createRole(input: RoleInput) {
  return createRoleBound(input).then(normalizeRole)
}

/** 修改角色信息和权限。 */
export function updateRole(roleId: string, input: RoleInput) {
  return updateRoleBound(roleId, input).then(normalizeRole)
}

/** 按是否回收站读取联系人列表。 */
async function listContactsByDeleted(
  query: ContactListQuery,
  deleted: boolean,
  signal?: AbortSignal,
) {
  const output = await listContactsBound(
    {
      query: query.query ?? "",
      stage: query.stage ?? null,
      channelId: query.channelId ?? "",
      methodType: query.methodType ?? null,
      sort: query.sort ?? ContactSort.ContactSortCreatedAtDescending,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
      deleted,
    },
    signal,
  )
  return {
    ...output,
    contacts: asList(output.contacts),
  } satisfies ContactListResponse
}

/** 把联系人详情中的可空切片转换为空数组。 */
function normalizeContact(contact: Contact): ContactDetail {
  return {
    ...contact,
    methods: asList(contact.methods),
    channelIdentities: asList(contact.channelIdentities),
  }
}

/** 把角色中的可空权限切片转换为空数组。 */
function normalizeRole(role: Role): RoleData {
  return { ...role, permissions: asList(role.permissions) }
}
