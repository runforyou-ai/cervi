/** Web 应用入口和路由。 */
import { SharedAppRoutes } from "@/apps/shared-app-routes"

/** 渲染 Web 端路由。 */
export default function WebApp() {
  return <SharedAppRoutes platform="web" />
}
