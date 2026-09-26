/** 设置中企业成员的独立新建页和编辑页。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router"

import { getUser, isNotFoundApiError, listRoles, listTeams } from "@/api"
import { PageBackButton } from "@/components/page-back-button"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ResourceContent } from "@/components/resource-content"
import { useWorkspace } from "@/contexts/workspace-context"
import { useContactInvalidator } from "@/features/contacts/use-contact-invalidator"
import { MemberAssistantsSection } from "@/features/contacts/assistants/member-assistants-section"
import { MemberForm } from "@/features/settings/members/member-form"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

const listPath = "/settings/members"

/** 新建企业成员，或边改边存已有成员；返回时回到来源列表的筛选和页码。 */
export function MemberFormPage({ mode }: { mode: "create" | "edit" }) {
  const { t } = useTranslation("contacts")
  const { t: tSettings } = useTranslation("settings")
  const { userId = "" } = useParams()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { identity } = useWorkspace()
  const invalidate = useResourceInvalidator()
  const invalidateContact = useContactInvalidator()
  // 只接受成员列表作为返回地址。
  const returnParameter = searchParams.get("returnTo") ?? ""
  const returnTo =
    returnParameter.split("?")[0] === listPath ? returnParameter : listPath

  const roles = useResource(resourceKeys.roles(), () => listRoles())
  const teams = useResource(resourceKeys.teams({ pageSize: 100 }), () =>
    listTeams({ pageSize: 100 }),
  )
  const detail = useResource(resourceKeys.user(userId), () => getUser(userId), {
    enabled: mode === "edit",
  })
  const user = detail.data

  // 成员不存在时返回来源列表。
  useEffect(() => {
    if (mode !== "edit" || !isNotFoundApiError(detail.error)) return
    console.warn("企业成员不存在", { user_id: userId })
    navigate(returnTo, { replace: true })
  }, [detail.error, mode, navigate, returnTo, userId])

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={
          mode === "create"
            ? t("members.create")
            : (user?.displayName ?? t("members.editTitle"))
        }
        description={t(
          mode === "create" ? "members.createDescription" : "members.editDescription",
        )}
      >
        {mode === "edit" ? <PageBackButton to={returnTo} /> : null}
      </PageHeader>
      <PageContent variant="form">
        <ResourceContent
          resources={mode === "edit" ? [roles, teams, detail] : [roles, teams]}
          errorMessage={tSettings("members.detailLoadError")}
        >
          {mode === "create" ? (
            <MemberForm
              roles={roles.data?.roles ?? []}
              teams={teams.data?.teams ?? []}
              onCancel={() => navigate(returnTo)}
              onSaved={() => {
                void invalidateContact("user")
                navigate(returnTo)
              }}
            />
          ) : user ? (
            <div className="flex flex-col gap-9">
              <MemberForm
                key={user.id}
                user={user}
                roles={roles.data?.roles ?? []}
                teams={teams.data?.teams ?? []}
                onSaved={(saved) => {
                  void invalidateContact("user", saved.id)
                  if (saved.id === identity.user.id) {
                    void invalidate(resourceKeys.identity())
                  }
                }}
                onNotFound={() => {
                  void invalidateContact("user")
                  navigate(returnTo, { replace: true })
                }}
              />
              <MemberAssistantsSection userId={user.id} />
            </div>
          ) : null}
        </ResourceContent>
      </PageContent>
    </div>
  )
}
