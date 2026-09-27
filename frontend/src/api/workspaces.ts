/** 登录账号、工作区列表与创建调用。 */
import {
  CreateWorkspace,
  ListWorkspaces,
  LoadAccount,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import { bind } from "@/api/client"

/** 读取当前登录账号。 */
export const loadAccount = bind(LoadAccount)

/** 读取当前账号作为有效成员可进入的工作区。 */
export const listWorkspaces = bind(ListWorkspaces)

/** 创建工作区，当前账号成为首位管理员。 */
export const createWorkspace = bind(CreateWorkspace)
