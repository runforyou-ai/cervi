/** 企业设置与个人账号设置调用。 */
import {
  ChangePassword,
  SelectImage,
  UpdateOrganization,
  UpdateProfile,
  UpdateUserPreferences,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { bind } from "@/api/client"

/** 修改当前企业通用设置。 */
export const updateOrganization = bind(UpdateOrganization)

/** 修改当前用户的头像、姓名和邮箱。 */
export const updateProfile = bind(UpdateProfile)

/** 使用原生文件对话框选择图片。 */
export const selectImage = bind(SelectImage)

/** 修改当前用户的登录密码。 */
export const changePassword = bind(ChangePassword)

/** 修改当前用户偏好设置。 */
export const updateUserPreferences = bind(UpdateUserPreferences)
