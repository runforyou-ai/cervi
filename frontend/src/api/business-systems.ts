/** 业务系统调用。 */
import {
  CreateBusinessSystem,
  DeleteBusinessSystem,
  GetBusinessSystem,
  ListBusinessSystems,
  UpdateBusinessSystem,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type { BusinessSystemList } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type BusinessSystemListData = NonNullArrays<BusinessSystemList>

/** 读取当前企业的业务系统列表。 */
export const listBusinessSystems = bind(ListBusinessSystems)

/** 读取业务系统详情。 */
export const getBusinessSystem = bind(GetBusinessSystem)

/** 创建业务系统。 */
export const createBusinessSystem = bind(CreateBusinessSystem)

/** 修改业务系统。 */
export const updateBusinessSystem = bind(UpdateBusinessSystem)

/** 删除业务系统。 */
export const deleteBusinessSystem = bind(DeleteBusinessSystem)
