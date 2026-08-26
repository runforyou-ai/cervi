/** 原生端外部页面调用。 */
import { OpenExternalPage } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { bind } from "@/api/client"

/** 在原生端应用内新窗口打开外部页面。 */
export const openExternalPage = bind(OpenExternalPage)
