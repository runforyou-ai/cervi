/** 企业设置与个人账号设置调用。 */
import {
  ChangePassword,
  GetBusinessHours,
  SelectImage,
  UpdateBusinessHours,
  UpdateOrganization,
  UpdateProfile,
  UpdateUserPreferences,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { BusinessHours } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type BusinessHoursData = NonNullArrays<BusinessHours>

/** 修改当前企业通用设置。 */
export const updateOrganization = bind(UpdateOrganization)

/** 读取当前企业的客服工作时间。 */
export const getBusinessHours = bind(GetBusinessHours)

/** 修改当前企业的客服工作时间。 */
export const updateBusinessHours = bind(UpdateBusinessHours)

/** 修改当前用户的头像、姓名和邮箱。 */
export const updateProfile = bind(UpdateProfile)

/** 使用原生文件对话框选择图片。 */
export const selectImage = bind(SelectImage)

/** 修改当前用户的登录密码。 */
export const changePassword = bind(ChangePassword)

/** 修改当前用户偏好设置。 */
export const updateUserPreferences = bind(UpdateUserPreferences)
