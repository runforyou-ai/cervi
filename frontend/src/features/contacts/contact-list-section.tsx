/** 通讯录各分类列表共用的页头、工具栏和分页列表骨架。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import type { PageInfo } from "@/api"
import { ListToolbar } from "@/components/list-toolbar"
import { PageHeader } from "@/components/page-header"
import type { ResourceState } from "@/components/resource-content"
import { ResourceListLayout } from "@/components/resource-list"
import { ContactScopeMobileSelect } from "@/features/contacts/contact-scope-mobile-select"
import type { ContactScope } from "@/features/contacts/contact-scope"

/** 渲染带窄屏范围切换的页头、筛选工具栏和分页列表，翻页时关闭已打开的详情。 */
export function ContactListSection({
  title,
  description,
  scope,
  headerActions,
  toolbar,
  list,
  page,
  setParameters,
  children,
}: {
  title: string
  description: string
  scope: ContactScope
  headerActions?: ReactNode
  toolbar: ReactNode
  list: ResourceState
  page: PageInfo
  setParameters: (changes: Record<string, string | null>) => void
  children: ReactNode
}) {
  const { t } = useTranslation("contacts")

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader
        title={title}
        description={description}
        beforeTitle={<ContactScopeMobileSelect scope={scope} />}
      >
        {headerActions}
      </PageHeader>

      <ListToolbar>{toolbar}</ListToolbar>

      <ResourceListLayout
        resources={list}
        errorMessage={t("list.loadError")}
        page={page}
        onPageChange={(number) =>
          setParameters({ page: String(number), selected: null })
        }
      >
        {children}
      </ResourceListLayout>
    </section>
  )
}
