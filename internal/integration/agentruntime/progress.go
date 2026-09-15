//go:build server

package agentruntime

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"sync"
	"time"
	"uuid"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
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

// BlockPayload 完整保存文本、工具调用参数和结果。
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

// streamView 返回内容块在运行流中的展示形态，工具调用不含参数和结果。
func (b Block) streamView() *StreamBlock {
	view := &StreamBlock{ID: b.ID, Position: b.Position, ModelCallID: b.ModelCallID, Kind: b.Kind, Text: b.Payload.Text}
	if call := b.Payload.ToolCall; call != nil {
		view.ToolCall = &StreamToolCall{CallID: call.CallID, Name: call.Name, Status: call.Status, StartedAt: call.StartedAt, CompletedAt: call.CompletedAt}
	}
	return view
}

// processRecorder 在内存中维护一次执行尝试的完整过程，并把展示变化交给运行流发布。
type processRecorder struct {
	adk.TypedBaseChatModelAgentMiddleware[*schema.AgenticMessage]
	mu            sync.Mutex
	process       []Block
	candidate     string
	toolPositions map[string]int
	call          *modelCallStream
	publisher     *streamPublisher
}

// modelCallStream 记录一次尚未定稿的模型调用中流式分片序号与内容块位置的对应关系。
type modelCallStream struct {
	id             string
	start          int
	indices        []int
	positions      map[int]int
	nextOrdinal    int
	candidateParts []candidatePart
	hasTools       bool
}

// candidatePart 定义候选正文中来自同一分片序号的一段文本。
type candidatePart struct {
	index int
	text  string
}

// newProcessRecorder 为每次执行创建独立的内容缓冲和运行流发布器。
func newProcessRecorder(request RunRequest) *processRecorder {
	streamID := request.StreamID
	if streamID == "" {
		streamID = uuid.NewV7().String()
	}
	return &processRecorder{
		toolPositions: make(map[string]int),
		publisher:     &streamPublisher{header: StreamDelta{RunID: request.RunID, StreamID: streamID, Attempt: request.Attempt}, sink: request.OnStream},
	}
}

// streamingModel 在模型流式分片被读取时交给过程记录器。
type streamingModel struct {
	model.BaseModel[*schema.AgenticMessage]
	recorder *processRecorder
}

// WrapModel 为每次模型调用接入流式分片记录。
func (r *processRecorder) WrapModel(_ context.Context, m model.BaseModel[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (model.BaseModel[*schema.AgenticMessage], error) {
	return &streamingModel{BaseModel: m, recorder: r}, nil
}

// Stream 开始记录一次模型调用，分片经过时同步累积并登记增量。
func (m *streamingModel) Stream(ctx context.Context, input []*schema.AgenticMessage, opts ...model.Option) (*schema.StreamReader[*schema.AgenticMessage], error) {
	reader, err := m.BaseModel.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	m.recorder.mu.Lock()
	m.recorder.beginCallLocked()
	m.recorder.mu.Unlock()
	return schema.StreamReaderWithConvert(reader, func(chunk *schema.AgenticMessage) (*schema.AgenticMessage, error) {
		if chunk != nil {
			m.recorder.receive(chunk)
		}
		return chunk, nil
	}), nil
}

// beginCallLocked 开始一次模型调用并清空上一调用的候选正文，调用方持有缓冲锁。
func (r *processRecorder) beginCallLocked() *modelCallStream {
	r.call = &modelCallStream{id: uuid.NewV7().String(), start: len(r.process), positions: make(map[int]int)}
	if r.candidate != "" {
		r.candidate = ""
		r.publisher.add(StreamOperation{Kind: StreamOperationClearCandidate})
	}
	return r.call
}

// receive 按分片序号累积思考、正文和工具调用，没有工具调用前的正文作为候选回复。
func (r *processRecorder) receive(chunk *schema.AgenticMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	call := r.call
	if call == nil {
		return
	}
	for _, block := range chunk.ContentBlocks {
		if block == nil {
			continue
		}
		// 流式分片使用模型给出的序号，整块输出按出现顺序编号。
		index := call.nextOrdinal
		if block.StreamingMeta != nil {
			index = block.StreamingMeta.Index
		} else {
			call.nextOrdinal++
		}
		if !slices.Contains(call.indices, index) {
			call.indices = append(call.indices, index)
		}
		position, exists := call.positions[index]
		switch block.Type {
		case schema.ContentBlockTypeReasoning:
			if block.Reasoning.Text == "" {
				continue
			}
			if exists {
				r.appendTextLocked(position, block.Reasoning.Text)
			} else {
				r.addBlockLocked(index, domain.AgentRunBlockThinking, BlockPayload{Text: block.Reasoning.Text})
			}
		case schema.ContentBlockTypeAssistantGenText:
			text := block.AssistantGenText.Text
			switch {
			case text == "":
			case !call.hasTools:
				if last := len(call.candidateParts) - 1; last >= 0 && call.candidateParts[last].index == index {
					call.candidateParts[last].text += text
				} else {
					call.candidateParts = append(call.candidateParts, candidatePart{index: index, text: text})
				}
				r.candidate += text
				r.publisher.add(StreamOperation{Kind: StreamOperationAppendCandidate, Text: text})
			case exists:
				r.appendTextLocked(position, text)
			default:
				r.addBlockLocked(index, domain.AgentRunBlockContent, BlockPayload{Text: text})
			}
		case schema.ContentBlockTypeFunctionToolCall:
			// 首个工具调用出现后，已输出的候选正文按分片序号转为调用前的说明块。
			if !call.hasTools {
				call.hasTools = true
				if r.candidate != "" {
					r.candidate = ""
					r.publisher.add(StreamOperation{Kind: StreamOperationClearCandidate})
					for _, part := range call.candidateParts {
						r.addBlockLocked(part.index, domain.AgentRunBlockContent, BlockPayload{Text: part.text})
					}
				}
			}
			chunkCall := block.FunctionToolCall
			if !exists {
				r.addBlockLocked(index, domain.AgentRunBlockToolCall, BlockPayload{ToolCall: &ToolCall{
					CallID: chunkCall.CallID, Name: chunkCall.Name, Arguments: chunkCall.Arguments, Status: domain.AgentToolCallQueued,
				}})
				continue
			}
			recorded := r.process[position].Payload.ToolCall
			recorded.Arguments += chunkCall.Arguments
			// 调用编号和工具名称在后续分片中才出现时补齐并发布。
			if (recorded.CallID == "" && chunkCall.CallID != "") || (recorded.Name == "" && chunkCall.Name != "") {
				recorded.CallID = cmp.Or(recorded.CallID, chunkCall.CallID)
				recorded.Name = cmp.Or(recorded.Name, chunkCall.Name)
				r.publisher.add(StreamOperation{Kind: StreamOperationUpsertBlock, Block: r.process[position].streamView()})
			}
		}
	}
}

// addBlockLocked 为当前模型调用追加内容块并登记分片序号，调用方持有缓冲锁。
func (r *processRecorder) addBlockLocked(index int, kind domain.AgentRunBlockKind, payload BlockPayload) {
	position := len(r.process)
	r.process = append(r.process, Block{ID: uuid.NewV7().String(), Position: int64(position + 1), ModelCallID: r.call.id, Kind: kind, Payload: payload})
	r.call.positions[index] = position
	r.publisher.add(StreamOperation{Kind: StreamOperationUpsertBlock, Block: r.process[position].streamView()})
}

// appendTextLocked 向已有文本块追加分片文本，调用方持有缓冲锁。
func (r *processRecorder) appendTextLocked(position int, text string) {
	r.process[position].Payload.Text += text
	r.publisher.add(StreamOperation{Kind: StreamOperationAppendBlockText, BlockID: r.process[position].ID, Text: text})
}

// AfterModelRewriteState 在工具执行前按完整模型输出定稿本次调用的内容块，沿用流式阶段分配的块编号。
func (r *processRecorder) AfterModelRewriteState(ctx context.Context, state *adk.TypedChatModelAgentState[*schema.AgenticMessage], _ *adk.TypedModelContext[*schema.AgenticMessage]) (context.Context, *adk.TypedChatModelAgentState[*schema.AgenticMessage], error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	message := state.Messages[len(state.Messages)-1]
	call := r.call
	if call == nil {
		call = r.beginCallLocked()
	}
	r.call = nil
	// 完整输出的第 i 个内容块由第 i 小的分片序号拼接而成，数量一致时才能沿用流式块编号。
	indices := slices.Sorted(slices.Values(call.indices))
	keyed := len(indices) == len(message.ContentBlocks)
	withCalls := hasToolCalls(message)
	var final []Block
	for i, block := range message.ContentBlocks {
		var kind domain.AgentRunBlockKind
		var payload BlockPayload
		switch {
		case block.Type == schema.ContentBlockTypeReasoning && block.Reasoning.Text != "":
			kind, payload = domain.AgentRunBlockThinking, BlockPayload{Text: block.Reasoning.Text}
		case block.Type == schema.ContentBlockTypeAssistantGenText && withCalls && block.AssistantGenText.Text != "":
			kind, payload = domain.AgentRunBlockContent, BlockPayload{Text: block.AssistantGenText.Text}
		case block.Type == schema.ContentBlockTypeFunctionToolCall:
			modelCall := block.FunctionToolCall
			kind, payload = domain.AgentRunBlockToolCall, BlockPayload{ToolCall: &ToolCall{
				CallID: modelCall.CallID, Name: modelCall.Name, Arguments: modelCall.Arguments, Status: domain.AgentToolCallQueued,
			}}
		default:
			continue
		}
		id := uuid.NewV7().String()
		if keyed {
			if position, ok := call.positions[indices[i]]; ok && r.process[position].Kind == kind {
				id = r.process[position].ID
			}
		}
		final = append(final, Block{ID: id, ModelCallID: call.id, Kind: kind, Payload: payload})
	}
	// 块编号顺序与流式阶段一致时只发布有变化的块，否则整体替换本次调用的块；定稿操作一次登记，同一增量内原子应用。
	var operations []StreamOperation
	streamed := slices.Clone(r.process[call.start:])
	sameOrder := slices.EqualFunc(streamed, final, func(a, b Block) bool { return a.ID == b.ID })
	if !sameOrder && len(streamed) > 0 {
		slog.Warn("Agent 模型调用定稿的内容块与流式阶段不一致，整体替换",
			"agent_run_id", r.publisher.header.RunID, "stream_id", r.publisher.header.StreamID,
			"streamed_blocks", len(streamed), "final_blocks", len(final))
		removed := make([]string, len(streamed))
		for i, block := range streamed {
			removed[i] = block.ID
		}
		operations = append(operations, StreamOperation{Kind: StreamOperationRemoveBlocks, BlockIDs: removed})
	}
	r.process = r.process[:call.start]
	for i, block := range final {
		block.Position = int64(call.start + i + 1)
		if block.Payload.ToolCall != nil {
			r.toolPositions[block.Payload.ToolCall.CallID] = call.start + i
		}
		r.process = append(r.process, block)
		if view := block.streamView(); !sameOrder || !reflect.DeepEqual(streamed[i].streamView(), view) {
			operations = append(operations, StreamOperation{Kind: StreamOperationUpsertBlock, Block: view})
		}
	}
	candidate := ""
	if !withCalls {
		candidate = assistantText(message)
	}
	if candidate != r.candidate {
		r.candidate = candidate
		operations = append(operations, StreamOperation{Kind: StreamOperationClearCandidate})
		if candidate != "" {
			operations = append(operations, StreamOperation{Kind: StreamOperationAppendCandidate, Text: candidate})
		}
	}
	r.publisher.add(operations...)
	return ctx, state, nil
}

// updateTool 更新原位置上的工具状态，允许并行工具乱序完成。
func (r *processRecorder) updateTool(callID string, update func(*ToolCall)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	position, ok := r.toolPositions[callID]
	if !ok {
		return fmt.Errorf("agent tool call %q has no model output", callID)
	}
	update(r.process[position].Payload.ToolCall)
	r.publisher.add(StreamOperation{Kind: StreamOperationUpsertBlock, Block: r.process[position].streamView()})
	return nil
}

// reset 在重新执行本次输入前清空已记录的过程内容。
func (r *processRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.process, r.candidate, r.call = nil, "", nil
	r.toolPositions = make(map[string]int)
	r.publisher.add(StreamOperation{Kind: StreamOperationReset})
}

// resetCandidate 在安全点补入新消息时丢弃候选正文、未完成模型调用的内容和被跳过的工具。
func (r *processRecorder) resetCandidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	var removed []string
	if r.call != nil {
		for _, block := range r.process[r.call.start:] {
			removed = append(removed, block.ID)
		}
		r.process, r.call = r.process[:r.call.start], nil
	}
	// AfterChatModel 安全点可跳过整批未启动工具，已返回的思考和说明文本仍保留。
	for len(r.process) > 0 {
		last := r.process[len(r.process)-1]
		if last.Payload.ToolCall == nil || last.Payload.ToolCall.Status != domain.AgentToolCallQueued {
			break
		}
		delete(r.toolPositions, last.Payload.ToolCall.CallID)
		removed = append(removed, last.ID)
		r.process = r.process[:len(r.process)-1]
	}
	if len(removed) > 0 {
		r.publisher.add(StreamOperation{Kind: StreamOperationRemoveBlocks, BlockIDs: removed})
	}
	if r.candidate != "" {
		r.candidate = ""
		r.publisher.add(StreamOperation{Kind: StreamOperationClearCandidate})
	}
}

// blocks 返回成功时应持久化的中间内容。
func (r *processRecorder) blocks() []Block {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.process)
}
