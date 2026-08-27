/** 外部联系人调用与归一化。 */
import {
  CreateContact,
  DeleteContact,
  GetContact,
  ListContacts,
  RestoreContact,
  UpdateContact,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  ContactSort,
  type Contact,
  type ContactInput,
  type ContactList,
  type ContactListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type ContactDetail = Omit<Contact, "methods" | "channelIdentities"> & {
  methods: NonNullable<Contact["methods"]>
  channelIdentities: NonNullable<Contact["channelIdentities"]>
}

export type ContactListResponse = Omit<ContactList, "contacts"> & {
  contacts: NonNullable<ContactList["contacts"]>
}

export type ContactListQuery = Omit<Partial<ContactListInput>, "deleted">

const listContactsBound = bind(ListContacts)
const getContactBound = bind(GetContact)
const createContactBound = bind(CreateContact)
const updateContactBound = bind(UpdateContact)
const restoreContactBound = bind(RestoreContact)

/** 将联系人移入回收站。 */
export const deleteContact = bind(DeleteContact)

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
