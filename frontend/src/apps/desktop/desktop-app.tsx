/** 桌面端应用入口和路由。 */
import { SharedAppRoutes } from "@/apps/shared-app-routes"

/** 渲染桌面端路由。 */
export default function DesktopApp() {
  return <SharedAppRoutes platform="desktop" />
}
