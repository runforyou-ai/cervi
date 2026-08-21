/** 绑定应用服务方法，并归一化可空切片。 */
import {
  ChangePassword,
  CreateContact,
  CreateWebsiteChannel,
  DeleteContact,
  DeleteWebsiteChannel,
  GetContact,
  GetS3Setting,
  GetUser,
  GetWebsiteChannel,
  ListChannels,
  ListContacts,
  ListUsers,
  ListWebsiteChannels,
  LoadInbox,
  RestoreContact,
  RestoreWebsiteChannel,
  SaveS3Setting,
  TestS3Setting,
  UpdateContact,
  UpdateProfile,
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
/** 保存对象存储设置。 */
export const saveS3Setting = bind(SaveS3Setting)
/** 测试对象存储连接。 */
export const testS3Setting = bind(TestS3Setting)
/** 修改当前用户的姓名和邮箱。 */
export const updateProfile = bind(UpdateProfile)
/** 修改当前用户的登录密码。 */
export const changePassword = bind(ChangePassword)
/** 修改当前用户的语言和时区设置。 */
export const updateUserPreferences = bind(UpdateUserPreferences)
/** 修改当前用户主动设置的工作状态。 */
export const updateUserWorkStatus = bind(UpdateUserWorkStatus)

const listChannelsBound = bind(ListChannels)
const listWebsiteChannelsBound = bind(ListWebsiteChannels)
const listUsersBound = bind(ListUsers)
const listContactsBound = bind(ListContacts)
const getContactBound = bind(GetContact)
const createContactBound = bind(CreateContact)
const updateContactBound = bind(UpdateContact)
const restoreContactBound = bind(RestoreContact)
const loadInboxBound = bind(LoadInbox)

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
