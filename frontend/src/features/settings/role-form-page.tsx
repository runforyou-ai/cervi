/** 角色新建和详情页。 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { LoaderCircleIcon } from "lucide-react"
import { Controller, useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router"
import { toast } from "sonner"

import {
  createRole,
  getRole,
  isApiError,
  listRoles,
  PermissionLevel,
  RoleKind,
  updateRole,
  updateUserRoles,
  type PermissionCode,
  type PermissionDefinition,
  type PermissionResource,
  type RoleData,
} from "@/api"
import { FormInputField } from "@/components/form/form-input-field"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import {
  permissionResourceLabel,
  roleDescription,
  roleDisplayName,
} from "@/features/roles/role-labels"
import {
  createRoleSettingsSchema,
  roleNameMaxLength,
  type RoleSettingsFormValues,
} from "@/features/settings/role-settings-schema"
import {
  RoleMemberDialog,
  type RoleMemberChange,
} from "@/features/settings/role-member-dialog"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"

const newRoleID = "new-role"

/** 按权限功能整理查看和管理选项。 */
function permissionRows(definitions: PermissionDefinition[]) {
  const rows = new Map<
    PermissionResource,
    Partial<Record<PermissionLevel, PermissionDefinition>>
  >()
  for (const definition of definitions) {
    const row = rows.get(definition.resource) ?? {}
    row[definition.level] = definition
    rows.set(definition.resource, row)
  }
  return [...rows.entries()]
}

/** 显示角色资料和权限表单。 */
export function RoleFormPage({ mode }: { mode: "create" | "detail" }) {
  const { t } = useTranslation("settings")
  const { t: tCommon } = useTranslation("common")
  const navigate = useNavigate()
  const { roleId = "" } = useParams()
  const [role, setRole] = useState<RoleData | null>(null)
  const [roles, setRoles] = useState<RoleData[]>([])
  const [definitions, setDefinitions] = useState<PermissionDefinition[]>([])
  const [memberDialogOpen, setMemberDialogOpen] = useState(false)
  const [memberChanges, setMemberChanges] = useState<RoleMemberChange[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)
  const loadVersion = useRef(0)
  const mounted = useRef(true)
  const schema = useMemo(
    () =>
      createRoleSettingsSchema({
        nameRequired: t("roles.validation.nameRequired"),
        nameTooLong: t("roles.validation.nameTooLong"),
        descriptionTooLong: t("roles.validation.descriptionTooLong"),
      }),
    [t],
  )
  const form = useForm<RoleSettingsFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: { name: "", description: "", permissions: [] },
  })
  const selected = useWatch({ control: form.control, name: "permissions" })
  const roleName = useWatch({ control: form.control, name: "name" })
  const admin = role?.kind === RoleKind.RoleKindAdmin
  const custom = mode === "create" || role?.kind === RoleKind.RoleKindCustom
  const memberTargetRole = useMemo<RoleData | null>(() => {
    if (role) return role
    if (mode !== "create") return null
    return {
      id: newRoleID,
      kind: RoleKind.RoleKindCustom,
      name: roleName.trim() || t("roles.members.newRole"),
      description: "",
      permissions: selected,
      memberCount: 0,
      createdAt: "",
      updatedAt: "",
    }
  }, [mode, role, roleName, selected, t])
  const memberDialogRoles = useMemo(
    () =>
      memberTargetRole &&
      !roles.some((item) => item.id === memberTargetRole.id)
        ? [...roles, memberTargetRole]
        : roles,
    [memberTargetRole, roles],
  )

  /** 读取权限目录和待编辑角色。 */
  const load = useCallback(async () => {
    const version = ++loadVersion.current
    setLoading(true)
    setLoadError(false)
    try {
      const [catalog, currentRole] = await Promise.all([
        listRoles(),
        mode === "detail" ? getRole(roleId) : Promise.resolve(null),
      ])
      if (version !== loadVersion.current) return
      setRoles(catalog.roles)
      setDefinitions(catalog.permissions)
      setRole(currentRole)
      setMemberChanges([])
      form.reset({
        name: currentRole
          ? currentRole.kind === RoleKind.RoleKindCustom
            ? currentRole.name
            : roleDisplayName(currentRole, tCommon)
          : "",
        description: currentRole
          ? roleDescription(currentRole, t)
          : "",
        permissions: currentRole?.permissions ?? [],
      })
    } catch (requestError) {
      if (version !== loadVersion.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("角色详情加载失败", { role_id: roleId, error: requestError })
      setLoadError(true)
    } finally {
      if (version === loadVersion.current) setLoading(false)
    }
  }, [form, mode, navigate, roleId, t, tCommon])

  useEffect(() => {
    mounted.current = true
    void load()
    return () => {
      mounted.current = false
      loadVersion.current += 1
    }
  }, [load])

  /** 切换权限并维护管理权限对查看权限的依赖。 */
  function togglePermission(
    definition: PermissionDefinition,
    checked: boolean,
  ) {
    const next = new Set(selected)
    if (checked) {
      next.add(definition.code)
      if (definition.level === PermissionLevel.PermissionLevelManage) {
        const view = definitions.find(
          (item) =>
            item.resource === definition.resource &&
            item.level === PermissionLevel.PermissionLevelView,
        )
        if (view) next.add(view.code)
      }
    } else {
      next.delete(definition.code)
      if (definition.level === PermissionLevel.PermissionLevelView) {
        const manage = definitions.find(
          (item) =>
            item.resource === definition.resource &&
            item.level === PermissionLevel.PermissionLevelManage,
        )
        if (manage) next.delete(manage.code)
      }
    }
    form.setValue("permissions", [...next] as PermissionCode[], {
      shouldDirty: true,
      shouldValidate: true,
    })
  }

  /** 保存角色资料、权限和成员配置。 */
  async function save(values: RoleSettingsFormValues) {
    let createdRoleID = ""
    let targetRoleID = roleId
    try {
      if (mode === "create") {
        const created = await createRole(values)
        createdRoleID = created.id
        targetRoleID = created.id
      } else {
        if (!admin) await updateRole(roleId, values)
      }
      if (memberChanges.length > 0) {
        await updateUserRoles({
          changes: memberChanges.map((change) => ({
            userId: change.user.id,
            roleId:
              change.nextRoleID === newRoleID
                ? targetRoleID
                : change.nextRoleID,
          })),
        })
      }
      if (!mounted.current) return
      toast.success(
        mode === "create"
          ? t("roles.form.createSuccess")
          : t("roles.form.updateSuccess"),
      )
      navigate("/settings/roles")
    } catch (requestError) {
      if (!mounted.current) return
      if (recoverSession(requestError, navigate)) return
      console.warn("保存角色失败", {
        role_id: targetRoleID,
        error: requestError,
      })
      if (isApiError(requestError)) {
        toast.error(
          apiErrorMessage(requestError, ["name", "description", "permissions"]),
        )
        if (createdRoleID) navigate(`/settings/roles/${createdRoleID}`)
        return
      }
      toast.error(t("roles.form.saveError"))
      if (createdRoleID) navigate(`/settings/roles/${createdRoleID}`)
    }
  }

  const title =
    mode === "create"
      ? t("roles.form.createTitle")
      : role
        ? roleDisplayName(role, tCommon)
        : t("roles.form.detailTitle")
  const rows = permissionRows(definitions)
  const pendingRoleIDs = useMemo(
    () =>
      Object.fromEntries(
        memberChanges.map((change) => [change.user.id, change.nextRoleID]),
      ),
    [memberChanges],
  )
  const memberCount = memberTargetRole
    ? memberChanges.reduce((count, change) => {
        if (change.previousRoleID === memberTargetRole.id) count -= 1
        if (change.nextRoleID === memberTargetRole.id) count += 1
        return count
      }, memberTargetRole.memberCount)
    : 0

  /** 返回角色列表。 */
  function cancel() {
    navigate("/settings/roles")
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={title} />
      <PageContent>
        {loading ? (
          <div className="flex min-h-48 items-center justify-center gap-2 rounded-lg border text-sm text-muted-foreground">
            <LoaderCircleIcon className="size-4 animate-spin" />
            {t("roles.loading")}
          </div>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">
              {t("roles.form.loadError")}
            </p>
            <Button className="mt-4" variant="outline" onClick={load}>
              {t("roles.retry")}
            </Button>
          </div>
        ) : (
          <form
            className="w-full max-w-3xl space-y-8"
            onSubmit={form.handleSubmit(save)}
            noValidate
          >
            <div className="space-y-5">
              <FieldGroup>
                <FormInputField
                  name="name"
                  control={form.control}
                  label={t("roles.form.name")}
                  autoFocus={custom}
                  disabled={!custom}
                  maxLength={roleNameMaxLength}
                />
                <Controller
                  name="description"
                  control={form.control}
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor="role-description" required={false}>
                        {t("roles.form.description")}
                      </FieldLabel>
                      <Textarea
                        {...field}
                        id="role-description"
                        disabled={!custom}
                        aria-invalid={fieldState.invalid}
                      />
                    </Field>
                  )}
                />
              </FieldGroup>

              {memberTargetRole ? (
                <section className="flex items-center justify-between gap-4 rounded-lg border bg-card p-4">
                  <div>
                    <h3 className="font-medium">{t("roles.members.sectionTitle")}</h3>
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t("roles.members.count", { count: memberCount })}
                    </p>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => setMemberDialogOpen(true)}
                  >
                    {t("roles.members.configure")}
                  </Button>
                </section>
              ) : null}

              <section>
                <div className="mb-3">
                  <h3 className="font-medium">{t("roles.permissions.title")}</h3>
                  {admin ? (
                    <p className="mt-1 text-sm text-muted-foreground">
                      {t("roles.permissions.adminDescription")}
                    </p>
                  ) : null}
                </div>
                <div className="overflow-hidden rounded-lg border bg-card">
                  <Table>
                    <TableHeader>
                      <TableRow className="hover:bg-transparent">
                        <TableHead>{t("roles.permissions.columns.function")}</TableHead>
                        <TableHead className="w-28 text-center">
                          {t("roles.permissions.columns.view")}
                        </TableHead>
                        <TableHead className="w-28 text-center">
                          {t("roles.permissions.columns.manage")}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {rows.map(([resource, levels]) => (
                        <TableRow key={resource}>
                          <TableCell className="font-medium">
                            {permissionResourceLabel(resource, t)}
                          </TableCell>
                          {[
                            PermissionLevel.PermissionLevelView,
                            PermissionLevel.PermissionLevelManage,
                          ].map((level) => {
                            const definition = levels[level]
                            if (!definition) {
                              return (
                                <TableCell key={level} className="text-center text-muted-foreground">
                                  —
                                </TableCell>
                              )
                            }
                            const checked = admin || selected.includes(definition.code)
                            const manage = levels[PermissionLevel.PermissionLevelManage]
                            const viewRequired =
                              level === PermissionLevel.PermissionLevelView &&
                              manage &&
                              selected.includes(manage.code)
                            return (
                              <TableCell key={level} className="text-center">
                                <input
                                  type="checkbox"
                                  className="size-4 accent-primary"
                                  checked={checked}
                                  disabled={admin || Boolean(viewRequired)}
                                  aria-label={t("roles.permissions.toggle", {
                                    resource: permissionResourceLabel(resource, t),
                                    level:
                                      level === PermissionLevel.PermissionLevelView
                                        ? t("roles.permissions.columns.view")
                                        : t("roles.permissions.columns.manage"),
                                  })}
                                  onChange={(event) =>
                                    togglePermission(definition, event.target.checked)
                                  }
                                />
                              </TableCell>
                            )
                          })}
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </section>

            </div>

            <div className="flex items-center gap-2">
              <Button
                type="submit"
                disabled={form.formState.isSubmitting}
              >
                {form.formState.isSubmitting ? (
                  <LoaderCircleIcon className="animate-spin" />
                ) : null}
                {form.formState.isSubmitting
                  ? t("roles.form.saving")
                  : t("roles.form.save")}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={form.formState.isSubmitting}
                onClick={cancel}
              >
                {t("roles.form.cancel")}
              </Button>
            </div>
          </form>
        )}
      </PageContent>
      <RoleMemberDialog
        role={memberDialogOpen ? memberTargetRole : null}
        roles={memberDialogRoles}
        pendingRoleIDs={pendingRoleIDs}
        onOpenChange={setMemberDialogOpen}
        onConfirm={setMemberChanges}
      />
    </div>
  )
}
