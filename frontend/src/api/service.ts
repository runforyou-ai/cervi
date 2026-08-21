/** 绑定应用服务方法，并归一化可空切片。 */
import {
  ChangePassword,
  CompleteFileUpload,
  CreateContact,
  CreateFileUpload,
  CreateRole,
  CreateWebsiteChannel,
  DeleteContact,
  DeleteRole,
  DeleteWebsiteChannel,
  GetContact,
  GetRole,
  GetS3Setting,
  GetUser,
  GetWebsiteChannel,
  ListChannels,
  ListContacts,
  ListRoles,
  ListUsers,
  ListWebsiteChannels,
  LoadInbox,
  RestoreContact,
  RestoreWebsiteChannel,
  SaveS3Setting,
  SelectProfileImage,
  TestS3Setting,
  UpdateContact,
  UpdateOrganization,
  UpdateProfile,
  UpdateRole,
  UpdateUserPreferences,
  UpdateUserWorkStatus,
  UpdateWebsiteChannel,
  UpdateWebsiteChannelChatInterface,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  ContactSort,
  StorageProvider,
  type Contact,
  type ContactInput,
  type ContactList,
  type ContactListInput,
  type Conversation as GeneratedConversation,
  type Inbox,
  type PermissionDefinition,
  type Role,
  type RoleInput,
  type UserListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"

export * from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

export type StorageProviderId = Exclude<
  StorageProvider,
  StorageProvider.$zero
>

export type ContactDetail = Omit<Contact, "methods" | "channelIdentities"> & {
  methods: NonNullable<Contact["methods"]>
  channelIdentities: NonNullable<Contact["channelIdentities"]>
}

export type ContactListResponse = Omit<ContactList, "contacts"> & {
  contacts: NonNullable<ContactList["contacts"]>
}

export type ContactListQuery = Omit<Partial<ContactListInput>, "deleted">

export type UserListQuery = Partial<UserListInput>

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

export type RoleListData = {
  roles: RoleData[]
  permissions: PermissionDefinition[]
}

/** 读取网站渠道详情。 */
export const getWebsiteChannel = bind(GetWebsiteChannel)
/** 创建网站渠道。 */
export const createWebsiteChannel = bind(CreateWebsiteChannel)
/** 修改网站渠道。 */
export const updateWebsiteChannel = bind(UpdateWebsiteChannel)
/** 修改网站渠道聊天界面。 */
export const updateWebsiteChannelChatInterface = bind(
  UpdateWebsiteChannelChatInterface,
)
/** 将网站渠道移入回收站。 */
export const deleteWebsiteChannel = bind(DeleteWebsiteChannel)
/** 恢复网站渠道。 */
export const restoreWebsiteChannel = bind(RestoreWebsiteChannel)
/** 将联系人移入回收站。 */
export const deleteContact = bind(DeleteContact)
/** 读取企业成员详情。 */
export const getUser = bind(GetUser)
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
/** 修改当前用户的语言和时区设置。 */
export const updateUserPreferences = bind(UpdateUserPreferences)
/** 修改当前用户主动设置的工作状态。 */
export const updateUserWorkStatus = bind(UpdateUserWorkStatus)
/** 删除自定义角色。 */
export const deleteRole = bind(DeleteRole)

const listChannelsBound = bind(ListChannels)
const listWebsiteChannelsBound = bind(ListWebsiteChannels)
const listUsersBound = bind(ListUsers)
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

/** 读取当前企业的渠道选择项。 */
export function listChannels(signal?: AbortSignal) {
  return listChannelsBound(signal).then((list) => asList(list.channels))
}

/** 读取网站渠道列表。 */
export function listWebsiteChannels() {
  return listWebsiteChannelsBound(false).then((list) => asList(list.channels))
}

/** 读取已删除的网站渠道列表。 */
export function listDeletedWebsiteChannels() {
  return listWebsiteChannelsBound(true).then((list) => asList(list.channels))
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
      role: query.role ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, users: asList(output.users) }))
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

/** 读取角色和预定义权限目录。 */
export function listRoles(signal?: AbortSignal): Promise<RoleListData> {
  return listRolesBound(signal).then((output) => ({
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
