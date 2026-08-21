//go:build server

// Package filecleanup 定时删除过期且未关联业务数据的文件。
package filecleanup

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	fileaction "github.com/runforyou-ai/cervi/internal/actions/file"
	settingaction "github.com/runforyou-ai/cervi/internal/actions/setting"
	"github.com/runforyou-ai/cervi/internal/domain"
	"github.com/runforyou-ai/cervi/internal/filestore"
	servermodels "github.com/runforyou-ai/cervi/internal/storage/server/models"
	"github.com/uptrace/bun"
)

const (
	defaultInterval  = time.Hour
	defaultBatchSize = 200
)

// Service 运行过期文件清理循环。
type Service struct {
	cleanup      *fileaction.CleanupAction
	getS3Setting *settingaction.GetS3SettingQuery
	local        *filestore.LocalStore
	interval     time.Duration
	batchSize    int
}

// NewService 创建过期文件清理服务。
func NewService(db *bun.DB, local *filestore.LocalStore) *Service {
	return &Service{
		cleanup:      fileaction.NewCleanupAction(db),
		getS3Setting: settingaction.NewGetS3SettingQuery(db),
		local:        local,
		interval:     defaultInterval,
		batchSize:    defaultBatchSize,
	}
}

// Start 立即清理一次，然后按固定周期继续运行。
func (s *Service) Start(ctx context.Context) {
	slog.Info("临时文件清理已启动", "interval", s.interval, "batch_size", s.batchSize)
	go func() {
		s.run(ctx)
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.run(ctx)
			}
		}
	}()
}

// run 清理当前已过期的所有文件批次。
func (s *Service) run(ctx context.Context) {
	scheduledCount := s.scheduleExpired(ctx)
	deletedCount := 0
	failedCount := 0
	s3Configs := make(map[string]filestore.S3Config)
	for {
		now := time.Now().UTC()
		records, err := s.cleanup.ClaimDeleting(ctx, now, now.Add(s.interval), s.batchSize)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("认领待删除文件失败", "error", err)
			}
			return
		}
		if len(records) == 0 {
			break
		}
		for index := range records {
			if err := s.delete(ctx, &records[index], s3Configs); err != nil {
				failedCount++
				if ctx.Err() == nil {
					slog.Warn("清理文件失败", "organization_id", records[index].OrganizationID, "file_id", records[index].ID, "storage_backend", records[index].StorageBackend, "error", err)
				}
				continue
			}
			if err := s.cleanup.DeleteClaimed(ctx, records[index].ID); err != nil {
				failedCount++
				if ctx.Err() == nil {
					slog.Warn("删除文件记录失败", "file_id", records[index].ID, "error", err)
				}
				continue
			}
			deletedCount++
		}
		if len(records) < s.batchSize {
			break
		}
	}
	if scheduledCount > 0 || deletedCount > 0 || failedCount > 0 {
		slog.Info("临时文件清理完成", "scheduled_count", scheduledCount, "deleted_count", deletedCount, "failed_count", failedCount)
	}
}

// scheduleExpired 将过期临时文件分批放入删除队列。
func (s *Service) scheduleExpired(ctx context.Context) int {
	total := 0
	for {
		now := time.Now().UTC()
		count, err := s.cleanup.ScheduleExpired(ctx, now, now.Add(s.interval), s.batchSize)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("安排过期文件清理失败", "error", err)
			}
			return total
		}
		total += int(count)
		if count < int64(s.batchSize) {
			return total
		}
	}
}

// delete 从文件记录指定的存储位置删除内容。
func (s *Service) delete(ctx context.Context, record *servermodels.File, s3Configs map[string]filestore.S3Config) error {
	switch domain.FileStorageBackend(record.StorageBackend) {
	case domain.FileStorageBackendLocal:
		return s.local.Delete(record.StorageKey)
	case domain.FileStorageBackendS3:
		config, exists := s3Configs[record.OrganizationID]
		if !exists {
			setting, err := s.getS3Setting.ExecuteForOrganization(ctx, record.OrganizationID)
			if err != nil {
				return err
			}
			config = filestore.S3Config{
				Endpoint: setting.Endpoint, Region: setting.Region, Bucket: setting.Bucket,
				AccessKeyID: setting.AccessKeyID, SecretAccessKey: setting.SecretAccessKey, ForcePathStyle: setting.ForcePathStyle,
			}
			s3Configs[record.OrganizationID] = config
		}
		return filestore.Delete(ctx, config, record.StorageKey)
	default:
		return fmt.Errorf("invalid file storage backend %q", record.StorageBackend)
	}
}
