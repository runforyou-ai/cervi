//go:build server

package agentruntime

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/runforyou-ai/cervi/internal/domain"
)

// streamFlushInterval 是运行流增量的合并发布周期。
const streamFlushInterval = 50 * time.Millisecond

var (
	// ErrStreamGap 表示增量序号不连续，接收方需要重新读取快照。
	ErrStreamGap = errors.New("agent run stream sequence gap")
	// ErrStreamMismatch 表示增量不属于当前快照所在的流，接收方需要重新读取快照。
	ErrStreamMismatch = errors.New("agent run stream mismatch")
)

// StreamOperationKind 定义运行流增量中的操作类型。
type StreamOperationKind string

const (
	StreamOperationUpsertBlock     StreamOperationKind = "upsert_block"
	StreamOperationAppendBlockText StreamOperationKind = "append_block_text"
	StreamOperationRemoveBlocks    StreamOperationKind = "remove_blocks"
	StreamOperationAppendCandidate StreamOperationKind = "append_candidate"
	StreamOperationClearCandidate  StreamOperationKind = "clear_candidate"
	StreamOperationReset           StreamOperationKind = "reset"
)

// StreamOperation 定义一条可按顺序应用到运行流快照的变更。
type StreamOperation struct {
	Kind     StreamOperationKind
	Block    *StreamBlock // upsert_block 写入的完整块。
	BlockID  string       // append_block_text 追加文本的块。
	BlockIDs []string     // remove_blocks 移除的块。
	Text     string       // append_block_text 与 append_candidate 追加的文本。
}

// StreamBlock 定义运行流中展示的内容块，工具调用只含名称和状态。
type StreamBlock struct {
	ID          string
	Position    int64
	ModelCallID string
	Kind        domain.AgentRunBlockKind
	Text        string
	ToolCall    *StreamToolCall
}

// StreamToolCall 定义运行流中的工具调用名称、状态和起止时间。
type StreamToolCall struct {
	CallID      string
	Name        string
	Status      domain.AgentToolCallStatus
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// StreamDelta 定义运行流快照从起始序号到终止序号的增量，序号在同一流内从 1 连续递增。
type StreamDelta struct {
	RunID        string
	StreamID     string
	Attempt      int
	BaseSequence int64 // 应用前快照所在的序号。
	Sequence     int64 // 应用后快照所在的序号。
	Operations   []StreamOperation
}

// StreamSnapshot 定义运行流在某个序号上的完整展示状态。
type StreamSnapshot struct {
	RunID            string
	StreamID         string
	Attempt          int
	Sequence         int64
	Blocks           []StreamBlock
	CandidateContent string
}

// Apply 应用起始序号与快照序号一致的增量；终止序号不超过快照序号时视为重复返回 false，起始序号不一致、流不一致或操作无法应用时返回错误。
func (s *StreamSnapshot) Apply(delta StreamDelta) (bool, error) {
	if delta.RunID != s.RunID || delta.StreamID != s.StreamID {
		return false, ErrStreamMismatch
	}
	if delta.Sequence <= s.Sequence {
		return false, nil
	}
	if delta.BaseSequence != s.Sequence {
		return false, ErrStreamGap
	}
	// 在副本上应用全部操作，任一操作失败时快照保持原状。
	blocks, candidate := slices.Clone(s.Blocks), s.CandidateContent
	for _, operation := range delta.Operations {
		switch operation.Kind {
		case StreamOperationUpsertBlock:
			index := slices.IndexFunc(blocks, func(block StreamBlock) bool { return block.ID == operation.Block.ID })
			if index >= 0 {
				blocks[index] = operation.Block.clone()
			} else {
				blocks = append(blocks, operation.Block.clone())
			}
		case StreamOperationAppendBlockText:
			index := slices.IndexFunc(blocks, func(block StreamBlock) bool { return block.ID == operation.BlockID })
			if index < 0 {
				return false, fmt.Errorf("append text to unknown stream block %q", operation.BlockID)
			}
			blocks[index].Text += operation.Text
		case StreamOperationRemoveBlocks:
			for _, id := range operation.BlockIDs {
				index := slices.IndexFunc(blocks, func(block StreamBlock) bool { return block.ID == id })
				if index < 0 {
					return false, fmt.Errorf("remove unknown stream block %q", id)
				}
				blocks = slices.Delete(blocks, index, index+1)
			}
		case StreamOperationAppendCandidate:
			candidate += operation.Text
		case StreamOperationClearCandidate:
			candidate = ""
		case StreamOperationReset:
			blocks, candidate = nil, ""
		default:
			return false, fmt.Errorf("unsupported stream operation %q", operation.Kind)
		}
	}
	s.Blocks, s.CandidateContent, s.Sequence = blocks, candidate, delta.Sequence
	return true, nil
}

// MergeStreamDeltas 把同一流内首尾相接的两条增量合并为一条，不相接时返回 false。
func MergeStreamDeltas(earlier, later StreamDelta) (StreamDelta, bool) {
	if earlier.RunID != later.RunID || earlier.StreamID != later.StreamID || earlier.Sequence != later.BaseSequence {
		return StreamDelta{}, false
	}
	merged := later
	merged.BaseSequence = earlier.BaseSequence
	merged.Operations = mergeStreamOperations(append(slices.Clone(earlier.Operations), later.Operations...))
	return merged, true
}

// Clone 复制快照供独立读取。
func (s StreamSnapshot) Clone() StreamSnapshot {
	s.Blocks = slices.Clone(s.Blocks)
	for i, block := range s.Blocks {
		s.Blocks[i] = block.clone()
	}
	return s
}

// clone 复制内容块及其工具调用和起止时间。
func (b StreamBlock) clone() StreamBlock {
	if b.ToolCall != nil {
		call := *b.ToolCall
		if call.StartedAt != nil {
			startedAt := *call.StartedAt
			call.StartedAt = &startedAt
		}
		if call.CompletedAt != nil {
			completedAt := *call.CompletedAt
			call.CompletedAt = &completedAt
		}
		b.ToolCall = &call
	}
	return b
}

// streamPublisher 按固定周期合并运行流操作，并串行交给接收方。
type streamPublisher struct {
	header  StreamDelta
	sink    func(StreamDelta)
	mu      sync.Mutex
	pending []StreamOperation
	stop    chan struct{}
	done    chan struct{}
}

// start 启动合并发布协程，未配置接收方时不发布。
func (p *streamPublisher) start() {
	if p.sink == nil {
		return
	}
	p.stop, p.done = make(chan struct{}), make(chan struct{})
	go func() {
		defer close(p.done)
		ticker := time.NewTicker(streamFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				p.flush()
			case <-p.stop:
				p.flush()
				return
			}
		}
	}()
}

// close 发布剩余操作并等待发布协程退出。
func (p *streamPublisher) close() {
	if p.stop == nil {
		return
	}
	close(p.stop)
	<-p.done
}

// add 登记待发布的操作，模型执行只写入内存队列。
func (p *streamPublisher) add(operations ...StreamOperation) {
	if p.sink == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.pending = append(p.pending, operations...)
}

// flush 合并本周期的操作并发布为下一个序号的增量。
func (p *streamPublisher) flush() {
	p.mu.Lock()
	if len(p.pending) == 0 {
		p.mu.Unlock()
		return
	}
	delta := p.header
	delta.BaseSequence, delta.Sequence = p.header.Sequence, p.header.Sequence+1
	p.header.Sequence = delta.Sequence
	delta.Operations = mergeStreamOperations(p.pending)
	p.pending = nil
	p.mu.Unlock()
	p.sink(delta)
}

// mergeStreamOperations 合并相邻的同块文本追加、候选正文追加和同块写入，重置之前的操作全部丢弃。
func mergeStreamOperations(operations []StreamOperation) []StreamOperation {
	merged := make([]StreamOperation, 0, len(operations))
	for _, operation := range operations {
		if operation.Kind == StreamOperationReset {
			merged = append(merged[:0], operation)
			continue
		}
		if len(merged) > 0 {
			last := &merged[len(merged)-1]
			switch {
			case operation.Kind == StreamOperationAppendBlockText && last.Kind == StreamOperationAppendBlockText && last.BlockID == operation.BlockID,
				operation.Kind == StreamOperationAppendCandidate && last.Kind == StreamOperationAppendCandidate:
				last.Text += operation.Text
				continue
			case operation.Kind == StreamOperationAppendBlockText && last.Kind == StreamOperationUpsertBlock && last.Block.ID == operation.BlockID:
				block := *last.Block
				block.Text += operation.Text
				last.Block = &block
				continue
			case operation.Kind == StreamOperationUpsertBlock && last.Kind == StreamOperationUpsertBlock && last.Block.ID == operation.Block.ID:
				last.Block = operation.Block
				continue
			}
		}
		merged = append(merged, operation)
	}
	return merged
}
