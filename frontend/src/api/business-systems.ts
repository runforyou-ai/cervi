/** 业务系统调用与归一化。 */
import {
  CreateBusinessSystem,
  DeleteBusinessSystem,
  GetBusinessSystem,
  ListBusinessSystems,
  UpdateBusinessSystem,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  BusinessSystem,
  BusinessSystemList,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type BusinessSystemListData = Omit<
  BusinessSystemList,
  "businessSystems"
> & {
  businessSystems: BusinessSystem[]
}

const listBusinessSystemsBound = bind(ListBusinessSystems)

/** 读取当前企业的业务系统列表。 */
export function listBusinessSystems() {
  return listBusinessSystemsBound().then(
    (output): BusinessSystemListData => ({
      ...output,
      businessSystems: asList(output.businessSystems),
    }),
  )
}

/** 读取业务系统详情。 */
export const getBusinessSystem = bind(GetBusinessSystem)

/** 创建业务系统。 */
export const createBusinessSystem = bind(CreateBusinessSystem)

/** 修改业务系统。 */
export const updateBusinessSystem = bind(UpdateBusinessSystem)

/** 删除业务系统。 */
export const deleteBusinessSystem = bind(DeleteBusinessSystem)
