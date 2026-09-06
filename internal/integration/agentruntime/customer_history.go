//go:build server

package agentruntime

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

type customerHistoryInput struct {
	Query string `json:"query" jsonschema_description:"需要从以往客服沟通中查找的问题或线索"`
}

// newCustomerHistoryTool 创建查询当前客户会话历史客服周期的工具。
func newCustomerHistoryTool(search CustomerHistorySearch) (tool.InvokableTool, error) {
	return toolutils.InferTool(
		"search_customer_history",
		"默认上下文只包含本轮客服沟通。需要了解以往沟通时，查询同一客户会话中已结束的客服记录。若返回 available=false，表示历史查询暂不可用，不代表没有历史记录，请勿重复调用。",
		func(ctx context.Context, input customerHistoryInput) (CustomerHistoryResult, error) {
			return search(ctx, input.Query)
		},
	)
}
