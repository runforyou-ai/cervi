//go:build server

package agentruntime

import (
	"context"
	"fmt"
	"sync"
	"time"
	"uuid"

	"github.com/cloudwego/eino/adk"
	"github.com/runforyou-ai/cervi/internal/domain"
)

// Block 定义按模型返回顺序排列的完整中间内容。
type Block struct {
	ID          string                   `json:"id"`
	Position    int64                    `json:"position"`
	ModelCallID string                   `json:"modelCallId"`
	Kind        domain.AgentRunBlockKind `json:"kind"`
	Payload     BlockPayload             `json:"payload"`
}

// BlockPayload 保存文本或工具调用内容，不截断原始参数和结果。
type BlockPayload struct {
	Text     string    `json:"text,omitempty"`
	ToolCall *ToolCall `json:"toolCall,omitempty"`
}

// ToolCall 保存一次模型指定的工具调用，包括可供模型修正的失败。
type ToolCall struct {
	CallID      string                     `json:"callId"`
	Name        string                     `json:"name"`
	Arguments   string                     `json:"arguments"`
	Result      *string                    `json:"result"`
	Error       *string                    `json:"error"`
	Status      domain.AgentToolCallStatus `json:"status"`
	StartedAt   *time.Time                 `json:"startedAt"`
	CompletedAt *time.Time                 `json:"completedAt"`
}

// Progress 定义一次执行尝试的临时快照，序号只在该流内递增。
type Progress struct {
	RunID            string  `json:"runId"`
	StreamID         string  `json:"streamId"`
	Attempt          int     `json:"attempt"`
	Sequence         int64   `json:"sequence"`
	Blocks           []Block `json:"blocks"`
	CandidateContent string  `json:"candidateContent"`
}

type processRecorder struct {
	adk.BaseChatModelAgentMiddleware
	mu            sync.Mutex
	progress      Progress
	toolPositions map[string]int
	onProgress    func(Progress)
}

// newProcessRecorder 为每次执行创建独立的内容缓冲。
func newProcessRecorder(request RunRequest) *processRecorder {
	streamID := request.StreamID
	if streamID == "" {
		streamID = uuid.NewV7().String()
	}
	return &processRecorder{progress: Progress{RunID: request.RunID, StreamID: streamID, Attempt: request.Attempt}, toolPositions: make(map[string]int), onProgress: request.OnProgress}
}

// AfterModelRewriteState 在工具执行前固定同次模型输出的内容块顺序。
func (r *processRecorder) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, _ *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	message := state.Messages[len(state.Messages)-1]
	modelCallID := uuid.NewV7().String()
	r.progress.CandidateContent = ""
	if message.ReasoningContent != "" {
		r.appendBlock(modelCallID, domain.AgentRunBlockThinking, BlockPayload{Text: message.ReasoningContent})
	}
	if len(message.ToolCalls) == 0 {
		r.progress.CandidateContent = message.Content
	} else {
		if message.Content != "" {
			r.appendBlock(modelCallID, domain.AgentRunBlockContent, BlockPayload{Text: message.Content})
		}
		for _, call := range message.ToolCalls {
			r.toolPositions[call.ID] = len(r.progress.Blocks)
			r.appendBlock(modelCallID, domain.AgentRunBlockToolCall, BlockPayload{ToolCall: &ToolCall{
				CallID: call.ID, Name: call.Function.Name, Arguments: call.Function.Arguments, Status: domain.AgentToolCallQueued,
			}})
		}
	}
	r.publishLocked()
	return ctx, state, nil
}

// appendBlock 为语义块分配稳定编号和位置，调用方持有缓冲锁。
func (r *processRecorder) appendBlock(modelCallID string, kind domain.AgentRunBlockKind, payload BlockPayload) {
	r.progress.Blocks = append(r.progress.Blocks, Block{ID: uuid.NewV7().String(), Position: int64(len(r.progress.Blocks) + 1), ModelCallID: modelCallID, Kind: kind, Payload: payload})
}

// updateTool 更新原位置上的工具状态，允许并行工具乱序完成。
func (r *processRecorder) updateTool(callID string, update func(*ToolCall)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	position, ok := r.toolPositions[callID]
	if !ok {
		return fmt.Errorf("agent tool call %q has no model output", callID)
	}
	update(r.progress.Blocks[position].Payload.ToolCall)
	r.publishLocked()
	return nil
}

// resetCandidate 在安全点补入新消息时丢弃候选正文和被跳过的工具。
func (r *processRecorder) resetCandidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress.CandidateContent = ""
	// AfterChatModel 安全点可跳过整批未启动工具，已返回的思考和说明文本仍保留。
	for len(r.progress.Blocks) > 0 {
		last := r.progress.Blocks[len(r.progress.Blocks)-1]
		if last.Payload.ToolCall == nil || last.Payload.ToolCall.Status != domain.AgentToolCallQueued {
			break
		}
		delete(r.toolPositions, last.Payload.ToolCall.CallID)
		r.progress.Blocks = r.progress.Blocks[:len(r.progress.Blocks)-1]
	}
	r.publishLocked()
}

// publishLocked 串行发布独立快照，回调只接收本次更新的数据副本。
func (r *processRecorder) publishLocked() {
	r.progress.Sequence++
	if r.onProgress != nil {
		r.onProgress(r.progress.Clone())
	}
}

// Clone 复制临时快照，使订阅者不能修改执行中的缓冲。
func (p Progress) Clone() Progress {
	blocks := make([]Block, len(p.Blocks))
	for i, block := range p.Blocks {
		blocks[i] = block
		if block.Payload.ToolCall != nil {
			call := *block.Payload.ToolCall
			if call.Result != nil {
				value := *call.Result
				call.Result = &value
			}
			if call.Error != nil {
				value := *call.Error
				call.Error = &value
			}
			if call.StartedAt != nil {
				value := *call.StartedAt
				call.StartedAt = &value
			}
			if call.CompletedAt != nil {
				value := *call.CompletedAt
				call.CompletedAt = &value
			}
			blocks[i].Payload.ToolCall = &call
		}
	}
	p.Blocks = blocks
	return p
}

// blocks 返回成功时应持久化的中间内容，不包含最终正文。
func (r *processRecorder) blocks() []Block {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.progress.Clone().Blocks
}
