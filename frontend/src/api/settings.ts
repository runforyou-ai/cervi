/** 企业设置与个人账号设置调用。 */
import {
  ChangePassword,
  GetS3Setting,
  SaveS3Setting,
  SelectImage,
  TestS3Setting,
  UpdateOrganization,
  UpdateProfile,
  UpdateUserPreferences,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { StorageProvider } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"

export type StorageProviderId = Exclude<StorageProvider, StorageProvider.$zero>

/** 读取对象存储设置。 */
export const getS3Setting = bind(GetS3Setting)

/** 修改当前企业通用设置。 */
export const updateOrganization = bind(UpdateOrganization)

/** 保存对象存储设置。 */
export const saveS3Setting = bind(SaveS3Setting)

/** 测试对象存储连接。 */
export const testS3Setting = bind(TestS3Setting)

/** 修改当前用户的头像、姓名和邮箱。 */
export const updateProfile = bind(UpdateProfile)

/** 使用原生文件对话框选择图片。 */
export const selectImage = bind(SelectImage)

/** 修改当前用户的登录密码。 */
export const changePassword = bind(ChangePassword)

/** 修改当前用户偏好设置。 */
export const updateUserPreferences = bind(UpdateUserPreferences)
