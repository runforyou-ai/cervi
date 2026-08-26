/** 连接器调用与归一化。 */
import {
  CreateIntegrationConnection,
  DeleteIntegrationConnection,
  GetIntegrationConnection,
  ListIntegrationConnections,
  TestIntegrationConnection,
  UpdateIntegrationConnection,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  IntegrationConnectionStatus,
  IntegrationConnectionType,
  type IntegrationConnection,
  type IntegrationConnectionInput,
  type IntegrationConnectionList,
  type IntegrationConnectionSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type IntegrationConnectionTypeId = Exclude<
  IntegrationConnectionType,
  IntegrationConnectionType.$zero
>

export type IntegrationConnectionStatusId = Exclude<
  IntegrationConnectionStatus,
  IntegrationConnectionStatus.$zero
>

export type IntegrationConnectionData = Omit<
  IntegrationConnection,
  "type" | "status"
> & {
  type: IntegrationConnectionTypeId
  status: IntegrationConnectionStatusId
}

export type IntegrationConnectionSummaryData = Omit<
  IntegrationConnectionSummary,
  "type" | "status"
> & {
  type: IntegrationConnectionTypeId
  status: IntegrationConnectionStatusId
}

export type IntegrationConnectionListData = Omit<
  IntegrationConnectionList,
  "connections"
> & {
  connections: IntegrationConnectionSummaryData[]
}

const listIntegrationConnectionsBound = bind(ListIntegrationConnections)
const getIntegrationConnectionBound = bind(GetIntegrationConnection)
const testIntegrationConnectionBound = bind(TestIntegrationConnection)
const createIntegrationConnectionBound = bind(CreateIntegrationConnection)
const updateIntegrationConnectionBound = bind(UpdateIntegrationConnection)

/** 读取当前企业的连接器列表。 */
export function listIntegrationConnections() {
  return listIntegrationConnectionsBound().then(
    (output): IntegrationConnectionListData => ({
      ...output,
      connections: asList(output.connections).map(
        normalizeIntegrationConnectionSummary,
      ),
    }),
  )
}

/** 读取连接器详情。 */
export function getIntegrationConnection(connectionId: string) {
  return getIntegrationConnectionBound(connectionId).then(
    normalizeIntegrationConnection,
  )
}

/** 测试连接器草稿配置。 */
export const testIntegrationConnection = testIntegrationConnectionBound

/** 创建连接器。 */
export function createIntegrationConnection(input: IntegrationConnectionInput) {
  return createIntegrationConnectionBound(input).then(
    normalizeIntegrationConnection,
  )
}

/** 修改连接器。 */
export function updateIntegrationConnection(
  connectionId: string,
  input: IntegrationConnectionInput,
) {
  return updateIntegrationConnectionBound(connectionId, input).then(
    normalizeIntegrationConnection,
  )
}

/** 删除连接器。 */
export const deleteIntegrationConnection = bind(DeleteIntegrationConnection)

/** 归一化连接器详情中的枚举值。 */
function normalizeIntegrationConnection(
  connection: IntegrationConnection,
): IntegrationConnectionData {
  return {
    ...connection,
    type: connection.type as IntegrationConnectionTypeId,
    status: connection.status as IntegrationConnectionStatusId,
  }
}

/** 归一化连接器列表项中的枚举值。 */
function normalizeIntegrationConnectionSummary(
  connection: IntegrationConnectionSummary,
): IntegrationConnectionSummaryData {
  return {
    ...connection,
    type: connection.type as IntegrationConnectionTypeId,
    status: connection.status as IntegrationConnectionStatusId,
  }
}
