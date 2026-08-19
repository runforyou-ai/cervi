import {
  CreateContact,
  DeleteContact,
  GetContact,
  ListContacts,
  RestoreContact,
  UpdateContact,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  ContactMethodType,
  ContactSort,
  ContactStage,
  type Contact,
  type ContactInput,
  type ContactList,
  type ContactListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { call } from "@/api/client"
import { optionalWailsEnum } from "@/lib/wails-enum"

export { ContactMethodType, ContactSort, ContactStage }
export type {
  ContactChannelIdentity,
  ContactInput,
  ContactMethod,
  ContactMethodInput,
  ContactRecord,
  ContactSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

export type ContactDetail = Omit<
  Contact,
  "methods" | "channelIdentities"
> & {
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

/** 从查询参数解析联系人阶段，空值表示不筛选。 */
export function contactStageFromQuery(value: string | null) {
  return optionalWailsEnum(ContactStage, value)
}

/** 从查询参数解析联系方式类型，空值表示不筛选。 */
export function contactMethodTypeFromQuery(value: string | null) {
  return optionalWailsEnum(ContactMethodType, value)
}

/** 从查询参数解析联系人排序，缺省按创建时间倒序。 */
export function contactSortFromQuery(value: string | null) {
  return (
    optionalWailsEnum(ContactSort, value) ??
    ContactSort.ContactSortCreatedAtDescending
  )
}

export async function listContacts(
  query: ContactListQuery,
  signal?: AbortSignal,
) {
  return listContactsByDeleted(query, false, signal)
}

export async function listDeletedContacts(
  query: ContactListQuery,
  signal?: AbortSignal,
) {
  return listContactsByDeleted(query, true, signal)
}

export function getContact(contactId: string, signal?: AbortSignal) {
  return call((meta) => GetContact(meta, contactId), signal).then(normalizeContact)
}

export function createContact(input: ContactInput) {
  return call((meta) => CreateContact(meta, input)).then(normalizeContact)
}

export function updateContact(contactId: string, input: ContactInput) {
  return call((meta) => UpdateContact(meta, contactId, input)).then(normalizeContact)
}

export function deleteContact(contactId: string) {
  return call((meta) => DeleteContact(meta, contactId))
}

export function restoreContact(contactId: string) {
  return call((meta) => RestoreContact(meta, contactId)).then(normalizeContact)
}

async function listContactsByDeleted(
  query: ContactListQuery,
  deleted: boolean,
  signal?: AbortSignal,
) {
  const output = await call(
    (meta) =>
      ListContacts(meta, {
        query: query.q ?? "",
        stage: query.stage ?? ContactStage.$zero,
        channelId: query.channelId ?? "",
        methodType: query.methodType ?? ContactMethodType.$zero,
        sort: query.sort ?? ContactSort.ContactSortCreatedAtDescending,
        page: query.page ?? 1,
        pageSize: query.pageSize ?? 50,
        deleted,
      }),
    signal,
  )
  return {
    ...output,
    contacts: output.contacts ?? [],
  } satisfies ContactListResponse
}

function normalizeContact(contact: Contact): ContactDetail {
  return {
    ...contact,
    methods: contact.methods ?? [],
    channelIdentities: contact.channelIdentities ?? [],
  }
}
