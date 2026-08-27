/** 设备通知权限与未读提醒调用。 */
import {
  CheckNotificationPermission,
  RequestNotificationPermission,
  SendMessageNotification,
  UpdateUnreadIndicator,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { bind } from "@/api/client"

/** 读取当前设备的通知权限状态。 */
export const checkNotificationPermission = bind(CheckNotificationPermission)

/** 申请当前设备的通知权限。 */
export const requestNotificationPermission = bind(
  RequestNotificationPermission,
)

/** 投递一条桌面新消息通知。 */
export const sendNativeMessageNotification = bind(SendMessageNotification)

/** 同步当前设备的未读数和托盘提醒状态。 */
export const updateUnreadIndicator = bind(UpdateUnreadIndicator)
