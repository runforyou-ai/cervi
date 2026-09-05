//go:build server

package appservice

import conversationaction "github.com/runforyou-ai/cervi/internal/actions/conversation"

// conversationAgentProcessFromAction 转换消息中有序的完整过程。
func conversationAgentProcessFromAction(process *conversationaction.ConversationAgentProcess) *ConversationAgentProcess {
	if process == nil {
		return nil
	}
	result := &ConversationAgentProcess{ID: process.ID, DurationMilliseconds: process.DurationMilliseconds,
		InputTokens: process.Usage.PromptTokens, OutputTokens: process.Usage.CompletionTokens, Blocks: make([]AgentRunContentBlock, 0, len(process.Blocks))}
	for _, block := range process.Blocks {
		item := AgentRunContentBlock{ID: block.ID, Position: block.Position, Kind: AgentRunBlockKind(block.Kind), Text: block.Payload.Text}
		if call := block.Payload.ToolCall; call != nil {
			item.ToolCall = &AgentToolCall{Name: call.Name, Arguments: call.Arguments, Result: call.Result, Error: call.Error, Status: AgentToolCallStatus(call.Status)}
		}
		result.Blocks = append(result.Blocks, item)
	}
	return result
}
