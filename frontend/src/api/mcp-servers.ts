/** MCP 服务调用与归一化。 */
import {
  CreateMCPServer,
  DeleteMCPServer,
  GetMCPServer,
  ListMCPServers,
  UpdateMCPServer,
  TestMCPServerConnection,
  TestSavedMCPServerConnection,
  RefreshMCPServerTools,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  MCPServer,
  MCPServerList,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type MCPServerListData = Omit<
  MCPServerList,
  "mcpServers"
> & {
  mcpServers: MCPServer[]
}

const listMCPServersBound = bind(ListMCPServers)

/** 读取当前企业的 MCP 服务列表。 */
export function listMCPServers() {
  return listMCPServersBound().then(
    (output): MCPServerListData => ({
      ...output,
      mcpServers: asList(output.mcpServers).map((server) => ({ ...server, tools: asList(server.tools) })),
    }),
  )
}

/** 读取 MCP 服务详情。 */
export const getMCPServer = bind(GetMCPServer)

/** 创建 MCP 服务。 */
export const createMCPServer = bind(CreateMCPServer)

/** 修改 MCP 服务。 */
export const updateMCPServer = bind(UpdateMCPServer)

/** 删除 MCP 服务。 */
export const deleteMCPServer = bind(DeleteMCPServer)

/** 测试 MCP 草稿连接配置。 */
export const testMCPServerConnection = bind(TestMCPServerConnection)

/** 测试已保存的 MCP 服务连接。 */
export const testSavedMCPServerConnection = bind(TestSavedMCPServerConnection)

/** 提交全部 MCP 服务的工具更新任务。 */
export const refreshMCPServerTools = bind(RefreshMCPServerTools)
