export type Organization = {
  id: string
  name: string
}

export type User = {
  id: string
  organizationId: string
  email: string
  displayName: string
  role: string
  status: string
}

export type Principal = {
  organization: Organization
  user: User
}
