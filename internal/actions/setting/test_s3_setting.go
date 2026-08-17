//go:build server

package setting

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ErrS3ConnectionFailed 表示 S3 连接测试失败。
var ErrS3ConnectionFailed = errors.New("S3 connection test failed")

// TestS3SettingAction 测试 S3 对象存储配置。
type TestS3SettingAction struct {
	timeout time.Duration
}

// NewTestS3SettingAction 创建 S3 配置测试操作。
func NewTestS3SettingAction() *TestS3SettingAction {
	return &TestS3SettingAction{timeout: 10 * time.Second}
}

// Execute 校验配置并测试目标存储桶是否可访问。
func (a *TestS3SettingAction) Execute(ctx context.Context, input S3Setting) error {
	input, fields := normalizeS3Setting(input)
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	testCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(input.Endpoint),
		Region:       input.Region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			input.AccessKeyID,
			input.SecretAccessKey,
			"",
		)),
		UsePathStyle:     input.ForcePathStyle,
		RetryMaxAttempts: 1,
	})
	_, err := client.HeadBucket(testCtx, &s3.HeadBucketInput{Bucket: aws.String(input.Bucket)})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrS3ConnectionFailed, err)
	}
	return nil
}
