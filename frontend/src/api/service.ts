import {
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
  UpdateWebsiteChannel,
  UpdateWebsiteChannelChatInterface,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  ContactMethodType,
  ContactSort,
  ContactStage,
  StorageProvider,
  UserRole,
  UserStatus,
  type Contact,
  type ContactInput,
  type ContactList,
  type ContactListInput,
  type Conversation,
  type Inbox,
  type UserListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"

export {
  ChannelType,
  ContactMethodType,
  ContactSort,
  ContactStage,
  Locale,
  MessageAuthor,
  StorageProvider,
  UserRole,
  UserStatus,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
export type {
  ChannelSummary,
  ContactChannelIdentity,
  ContactInput,
  ContactMethod,
  ContactMethodInput,
  ContactRecord,
  ContactSummary,
  DirectoryUser,
  Message,
  Organization,
  PageInfo,
  Principal,
  S3Setting,
  User,
  WebsiteChannel,
  WebsiteChannelChatInterface,
  WebsiteChannelChatInterfaceInput,
  WebsiteChannelInput,
  WebsiteChannelSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

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

export type ContactListQuery = Omit<
  Partial<ContactListInput>,
  "query" | "deleted"
> & {
  q?: string
}

export type UserListQuery = Omit<Partial<UserListInput>, "query"> & {
  q?: string
}

export type ConversationData = Omit<Conversation, "messages"> & {
  messages: NonNullable<Conversation["messages"]>
}

export type InboxData = Omit<Inbox, "conversations"> & {
  conversations: ConversationData[]
}

export type { ConversationData as Conversation }

export const getWebsiteChannel = bind(GetWebsiteChannel)
export const createWebsiteChannel = bind(CreateWebsiteChannel)
export const updateWebsiteChannel = bind(UpdateWebsiteChannel)
export const updateWebsiteChannelChatInterface = bind(
  UpdateWebsiteChannelChatInterface,
)
export const deleteWebsiteChannel = bind(DeleteWebsiteChannel)
export const restoreWebsiteChannel = bind(RestoreWebsiteChannel)
export const deleteContact = bind(DeleteContact)
export const getUser = bind(GetUser)
export const getS3Setting = bind(GetS3Setting)
export const saveS3Setting = bind(SaveS3Setting)
export const testS3Setting = bind(TestS3Setting)

const listChannelsBound = bind(ListChannels)
const listWebsiteChannelsBound = bind(ListWebsiteChannels)
const listUsersBound = bind(ListUsers)
const listContactsBound = bind(ListContacts)
const getContactBound = bind(GetContact)
const createContactBound = bind(CreateContact)
const updateContactBound = bind(UpdateContact)
const restoreContactBound = bind(RestoreContact)
const loadInboxBound = bind(LoadInbox)

function asList<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}

export function listChannels(signal?: AbortSignal) {
  return listChannelsBound(signal).then(asList)
}

export function listWebsiteChannels() {
  return listWebsiteChannelsBound(false).then(asList)
}

export function listDeletedWebsiteChannels() {
  return listWebsiteChannelsBound(true).then(asList)
}

export function getContact(contactId: string, signal?: AbortSignal) {
  return getContactBound(contactId, signal).then(normalizeContact)
}

export function createContact(input: ContactInput) {
  return createContactBound(input).then(normalizeContact)
}

export function updateContact(contactId: string, input: ContactInput) {
  return updateContactBound(contactId, input).then(normalizeContact)
}

export function restoreContact(contactId: string) {
  return restoreContactBound(contactId).then(normalizeContact)
}

export function listContacts(query: ContactListQuery, signal?: AbortSignal) {
  return listContactsByDeleted(query, false, signal)
}

export function listDeletedContacts(
  query: ContactListQuery,
  signal?: AbortSignal,
) {
  return listContactsByDeleted(query, true, signal)
}

export function listUsers(query: UserListQuery, signal?: AbortSignal) {
  return listUsersBound(
    {
      query: query.q ?? "",
      status: query.status ?? UserStatus.$zero,
      role: query.role ?? UserRole.$zero,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, users: output.users ?? [] }))
}

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

async function listContactsByDeleted(
  query: ContactListQuery,
  deleted: boolean,
  signal?: AbortSignal,
) {
  const output = await listContactsBound(
    {
      query: query.q ?? "",
      stage: query.stage ?? ContactStage.$zero,
      channelId: query.channelId ?? "",
      methodType: query.methodType ?? ContactMethodType.$zero,
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

function normalizeContact(contact: Contact): ContactDetail {
  return {
    ...contact,
    methods: asList(contact.methods),
    channelIdentities: asList(contact.channelIdentities),
  }
}
