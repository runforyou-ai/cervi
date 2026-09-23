/** 外部联系人调用。 */
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
  type ContactList,
  type ContactListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type ContactDetail = NonNullArrays<Contact>

export type ContactListResponse = NonNullArrays<ContactList>

export type ContactListQuery = Omit<Partial<ContactListInput>, "deleted">

const listContactsBound = bind(ListContacts)

/** 将联系人移入回收站。 */
export const deleteContact = bind(DeleteContact)

/** 读取联系人详情。 */
export const getContact = bind(GetContact)

/** 创建联系人。 */
export const createContact = bind(CreateContact)

/** 修改联系人。 */
export const updateContact = bind(UpdateContact)

/** 恢复联系人。 */
export const restoreContact = bind(RestoreContact)

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
function listContactsByDeleted(
  query: ContactListQuery,
  deleted: boolean,
  signal?: AbortSignal,
) {
  return listContactsBound(
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
}
