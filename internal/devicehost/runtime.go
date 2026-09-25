//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/runforyou-ai/cervi/internal/appservice"
	"github.com/runforyou-ai/cervi/internal/integration/agentruntime"
	"github.com/runforyou-ai/cervi/internal/integration/knowledgeretrieval"
	"github.com/runforyou-ai/cervi/internal/integration/localworkspace"
	"github.com/runforyou-ai/cervi/internal/integration/websearch"
)

const (
	// defaultRunTimeout 是服务端未给出有效运行总时限时使用的时限。
	defaultRunTimeout = 30 * time.Minute
	// deviceModelAPIKey 是模型组件要求的非空凭据占位值，模型请求的认证由传输层写入登录令牌。
	deviceModelAPIKey = "cervi-device"
)

// errRunSuppressed 表示服务端判定运行已失效，本机停止执行。
var errRunSuppressed = errors.New("device run suppressed")

// runAgent 按领取时固定的有效配置在本机执行运行时，模型请求经企业服务端模型代理，成功时回报结果。
func (w *Worker) runAgent(runCtx context.Context, meta appservice.RequestMeta, runID string, claim appservice.DeviceRunClaim, local *activeRun) (agentruntime.RunResult, error) {
	var assignment agentruntime.Assignment
	if err := json.Unmarshal(claim.Assignment, &assignment); err != nil {
		return agentruntime.RunResult{}, fmt.Errorf("decode device run assignment: %w", err)
	}
	baseURL, transport, err := w.client.DeviceModelEndpoint(runCtx, meta, runID)
	if err != nil {
		return agentruntime.RunResult{}, fmt.Errorf("resolve device model endpoint: %w", err)
	}
	// 运行总时限以服务端下发的为准，无效时按默认时限执行。
	timeout := time.Duration(claim.RunTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	ctx, cancel := context.WithTimeout(runCtx, timeout)
	defer cancel()
	request := agentruntime.RunRequest{
		RunID:       runID,
		Assignment:  assignment,
		Credentials: agentruntime.ModelCredentials{APIKey: deviceModelAPIKey, BaseURL: baseURL, Transport: transport},
		ReadAttachment: func(ctx context.Context, messageID string) ([]byte, error) {
			return w.client.ReadDeviceRunAttachment(ctx, meta, runID, messageID)
		},
		StreamID: local.streamID,
		Attempt:  1,
		OnStream: func(delta agentruntime.StreamDelta) {
			// 运行 context 已取消时丢弃增量。
			if ctx.Err() == nil {
				local.stream.Publish(delta)
			}
		},
	}
	// 本机工具以会话的默认文件夹为相对路径起点；默认文件夹创建失败只影响文件工具，不影响回复。
	if err := os.MkdirAll(local.folder, 0o755); err != nil {
		slog.Warn("创建会话默认文件夹失败", "agent_run_id", runID, "error", err)
	}
	// 本机命令、本地 MCP 服务的试启动与加载使用同一套运行环境。
	environment := w.toolchain.Environment()
	request.Workspace = localworkspace.New(local.folder, environment)
	// 运行环境为命令前置了工具链目录时，命令工具与本地 MCP 工具的说明写明托管运行环境的用法。
	request.ManagedToolchain = len(environment.PathPrefix) > 0
	request.LocalMCP = &localMCPManager{store: w.localMCP, environment: environment, dir: local.folder}
	request.MCPConnections = localMCPConnections(w.localMCP, environment, local.folder)
	// 有效配置包含知识检索时经企业服务端检索运行绑定的知识库。
	if slices.Contains(assignment.Tools, agentruntime.KnowledgeToolName) {
		request.KnowledgeSearch = remoteKnowledgeSearch(w.client, meta, runID)
	}
	// 联网搜索经企业服务端调用企业配置的搜索服务，网页在本机读取。
	if slices.Contains(assignment.Tools, agentruntime.WebSearchToolName) {
		request.WebSearch = remoteWebSearch(w.client, meta, runID)
	}
	if slices.Contains(assignment.Tools, agentruntime.WebFetchToolName) {
		request.WebFetch = w.pages.Read
	}
	result, err := w.runtime.Run(ctx, request, &remoteInputFeed{client: w.client, meta: meta, runID: runID})
	if err != nil {
		return result, err
	}
	input := appservice.DeviceRunResultInput{Content: result.Content, EndSeq: result.EndSeq}
	if input.Decision, err = json.Marshal(result.Decision); err != nil {
		return result, fmt.Errorf("encode device run decision: %w", err)
	}
	if input.Usage, input.Blocks, err = encodeProcess(result); err != nil {
		return result, err
	}
	completeCtx, cancelComplete := context.WithTimeout(runCtx, workRequestTimeout)
	defer cancelComplete()
	if err := w.client.CompleteDeviceRun(completeCtx, meta, runID, input); err != nil {
		return result, fmt.Errorf("complete device run: %w", err)
	}
	return result, nil
}

// encodeProcess 编码运行已产生的用量与过程内容块。
func encodeProcess(result agentruntime.RunResult) (json.RawMessage, json.RawMessage, error) {
	usage, err := json.Marshal(result.Usage)
	if err != nil {
		return nil, nil, fmt.Errorf("encode device run usage: %w", err)
	}
	blocks, err := json.Marshal(result.Blocks)
	if err != nil {
		return nil, nil, fmt.Errorf("encode device run blocks: %w", err)
	}
	return usage, blocks, nil
}

// remoteKnowledgeSearch 返回经企业服务端检索运行绑定知识库的检索函数。
func remoteKnowledgeSearch(client appservice.DeviceRunBackend, meta appservice.RequestMeta, runID string) agentruntime.KnowledgeSearch {
	return func(ctx context.Context, request knowledgeretrieval.Request) (knowledgeretrieval.Result, error) {
		encoded, err := json.Marshal(request)
		if err != nil {
			return knowledgeretrieval.Result{}, fmt.Errorf("encode knowledge search request: %w", err)
		}
		output, err := client.SearchDeviceRunKnowledge(ctx, meta, runID, appservice.DeviceRunKnowledgeSearchInput{Request: encoded})
		if err != nil {
			return knowledgeretrieval.Result{}, err
		}
		var result knowledgeretrieval.Result
		if err := json.Unmarshal(output.Result, &result); err != nil {
			return knowledgeretrieval.Result{}, fmt.Errorf("decode knowledge search result: %w", err)
		}
		return result, nil
	}
}

// remoteWebSearch 返回经企业服务端调用企业搜索服务的搜索函数。
func remoteWebSearch(client appservice.DeviceRunBackend, meta appservice.RequestMeta, runID string) agentruntime.WebSearch {
	return func(ctx context.Context, request websearch.Request) (websearch.Result, error) {
		encoded, err := json.Marshal(request)
		if err != nil {
			return websearch.Result{}, fmt.Errorf("encode web search request: %w", err)
		}
		output, err := client.SearchDeviceRunWeb(ctx, meta, runID, appservice.DeviceRunWebSearchInput{Request: encoded})
		if err != nil {
			return websearch.Result{}, err
		}
		var result websearch.Result
		if err := json.Unmarshal(output.Result, &result); err != nil {
			return websearch.Result{}, fmt.Errorf("decode web search result: %w", err)
		}
		return result, nil
	}
}

// remoteInputFeed 经企业服务端读取与认领设备运行的输入。
type remoteInputFeed struct {
	client appservice.DeviceRunBackend
	meta   appservice.RequestMeta
	runID  string
}

// Peek 返回指定序号之后尚未认领的连续输入信号。
func (f *remoteInputFeed) Peek(ctx context.Context, afterSeq int64) ([]agentruntime.Trigger, error) {
	signals, err := f.client.PeekDeviceRunInputs(ctx, f.meta, f.runID, appservice.DeviceRunInputPeekInput{AfterSeq: int(afterSeq)})
	if err != nil {
		return nil, fmt.Errorf("peek device run inputs: %w", err)
	}
	triggers := make([]agentruntime.Trigger, 0, len(signals.Seqs))
	for _, seq := range signals.Seqs {
		triggers = append(triggers, agentruntime.Trigger{Seq: seq})
	}
	return triggers, nil
}

// Claim 认领截至指定序号的输入并返回截至该边界的上下文消息，运行已失效时返回 errRunSuppressed。
func (f *remoteInputFeed) Claim(ctx context.Context, throughSeq int64) (agentruntime.ClaimedInput, error) {
	claimed, err := f.client.ClaimDeviceRunInputs(ctx, f.meta, f.runID, appservice.DeviceRunInputClaimInput{ThroughSeq: throughSeq})
	if err != nil {
		return agentruntime.ClaimedInput{}, fmt.Errorf("claim device run inputs: %w", err)
	}
	if claimed.Suppressed {
		return agentruntime.ClaimedInput{}, errRunSuppressed
	}
	input := agentruntime.ClaimedInput{EndSeq: claimed.EndSeq}
	if err := json.Unmarshal(claimed.Messages, &input.Messages); err != nil {
		return agentruntime.ClaimedInput{}, fmt.Errorf("decode claimed messages: %w", err)
	}
	return input, nil
}
