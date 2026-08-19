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
} from "../../bindings/github.com/runforyou-ai/cervi/internal/domain/models"
import type {
  Contact,
  ContactInput,
  ContactList,
  ContactListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { call } from "@/api/client"

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
        sort: query.sort ?? ContactSort.$zero,
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
