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
  return listChannelsBound(signal).then((list) => asList(list.channels))
}

export function listWebsiteChannels() {
  return listWebsiteChannelsBound(false).then((list) => asList(list.channels))
}

export function listDeletedWebsiteChannels() {
  return listWebsiteChannelsBound(true).then((list) => asList(list.channels))
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
      query: query.query ?? "",
      status: query.status ?? null,
      role: query.role ?? null,
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  ).then((output) => ({ ...output, users: asList(output.users) }))
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

function normalizeContact(contact: Contact): ContactDetail {
  return {
    ...contact,
    methods: asList(contact.methods),
    channelIdentities: asList(contact.channelIdentities),
  }
}
