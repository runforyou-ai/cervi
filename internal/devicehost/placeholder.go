//go:build !server && !ios && !android

package devicehost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/runforyou-ai/cervi/internal/appservice"
)

// runPlaceholder 以固定回复执行一次设备运行：认领当前全部输入，回复本机工作区名称与收到的上下文消息数量。
func runPlaceholder(ctx context.Context, client appservice.DeviceRunBackend, meta appservice.RequestMeta, runID, workspacePath string) error {
	signals, err := client.PeekDeviceRunInputs(ctx, meta, runID, appservice.DeviceRunInputPeekInput{})
	if err != nil {
		return fmt.Errorf("peek device run inputs: %w", err)
	}
	if len(signals.Seqs) == 0 {
		return errors.New("device run has no input")
	}
	claimed, err := client.ClaimDeviceRunInputs(ctx, meta, runID, appservice.DeviceRunInputClaimInput{ThroughSeq: signals.Seqs[len(signals.Seqs)-1]})
	if err != nil {
		return fmt.Errorf("claim device run inputs: %w", err)
	}
	if claimed.Suppressed {
		return nil
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(claimed.Messages, &messages); err != nil {
		return fmt.Errorf("decode claimed messages: %w", err)
	}
	return client.CompleteDeviceRun(ctx, meta, runID, appservice.DeviceRunResultInput{
		Content: fmt.Sprintf("已在本机工作区 %s 收到 %d 条上下文消息。", filepath.Base(workspacePath), len(messages)),
		EndSeq:  claimed.EndSeq,
	})
}
