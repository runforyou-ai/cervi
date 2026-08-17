import { request } from "@/api/client"
import type { PageInfo } from "@/api/users"

export type ContactStage = "visitor" | "lead" | "customer"
export type ContactMethodType = "email" | "phone"

export type ContactMethod = {
  id: string
  organizationId: string
  contactId: string
  type: ContactMethodType
  value: string
  label: string | null
  isPrimary: boolean
  createdAt: string
  updatedAt: string
}

export type ContactChannelIdentity = {
  id: string
  channelId: string
  channelType: string
  channelName: string
  externalId: string
  displayName: string | null
  lastSeenAt: string | null
}

export type ContactRecord = {
  id: string
  organizationId: string
  createdByUserId: string
  sourceChannelId?: string | null
  displayName: string | null
  stage: ContactStage
  notes: string | null
  createdAt: string
  updatedAt: string
  deletedAt: string | null
}

export type ContactSummary = {
  id: string
  displayName: string | null
  stage: ContactStage
  notes: string | null
  primaryEmail: string | null
  primaryPhone: string | null
  sourceChannelName: string | null
  channelCount: number
  createdAt: string
  updatedAt: string
  deletedAt: string | null
}

export type ContactDetail = {
  contact: ContactRecord
  sourceChannel?: {
    id: string
    type: string
    name: string
  } | null
  methods: ContactMethod[]
  channelIdentities: ContactChannelIdentity[]
}

export type ContactMethodInput = {
  type: ContactMethodType
  value: string
  label?: string
  isPrimary: boolean
}

export type ContactInput = {
  displayName: string
  channelId: string
  stage: ContactStage
  notes: string
  methods: ContactMethodInput[]
}

export type ContactListQuery = {
  q?: string
  stage?: ContactStage | ""
  channelId?: string
  methodType?: ContactMethodType | ""
  sort?: "updatedAt.desc" | "createdAt.desc" | "displayName.asc"
  page?: number
  pageSize?: number
}

export type ContactListResponse = {
  contacts: ContactSummary[]
  page: PageInfo
}

function queryString(query: ContactListQuery) {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value))
    }
  }
  return search.toString()
}

export function listContacts(query: ContactListQuery, signal?: AbortSignal) {
  return request<ContactListResponse>(`/contacts?${queryString(query)}`, {
    signal,
  })
}

export function listDeletedContacts(
  query: ContactListQuery,
  signal?: AbortSignal,
) {
  return request<ContactListResponse>(
    `/contacts/trash?${queryString(query)}`,
    { signal },
  )
}

export function getContact(contactId: string, signal?: AbortSignal) {
  return request<ContactDetail>(`/contacts/${contactId}`, { signal })
}

export function createContact(input: ContactInput) {
  return request<ContactDetail>("/contacts", {
    method: "POST",
    body: JSON.stringify(input),
  })
}

export function updateContact(contactId: string, input: ContactInput) {
  return request<ContactDetail>(`/contacts/${contactId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  })
}

export function deleteContact(contactId: string) {
  return request<void>(`/contacts/${contactId}`, { method: "DELETE" })
}

export function restoreContact(contactId: string) {
  return request<ContactDetail>(`/contacts/${contactId}/restore`, {
    method: "POST",
  })
}
