/** 未选择知识库时的默认内容页。 */
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { PageContent } from "@/components/page-content"
import { Button } from "@/components/ui/button"

/** 提示用户从窄边栏选择或新建知识库。 */
export function KnowledgeBaseIndexPage() {
  const { t } = useTranslation("knowledgeBase")

  return (
    <PageContent className="flex items-center justify-center">
      <div className="flex max-w-sm flex-col items-center text-center">
        <p className="text-sm text-muted-foreground">
          {t("selection.empty")}
        </p>
        <div className="mt-4 flex gap-2">
          <Button size="sm" asChild>
            <Link to="/knowledge-bases/new?category=standard">
              {t("selection.createDocument")}
            </Link>
          </Button>
          <Button size="sm" variant="outline" asChild>
            <Link to="/knowledge-bases/new?category=qa">
              {t("selection.createQA")}
            </Link>
          </Button>
        </div>
      </div>
    </PageContent>
  )
}
