//go:build server

package setting

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
)

// TestS3SettingAction 测试 S3 对象存储配置。
type TestS3SettingAction struct {
	runner *connectiontest.Runner
}

// NewTestS3SettingAction 创建 S3 配置测试操作。
func NewTestS3SettingAction(runner *connectiontest.Runner) *TestS3SettingAction {
	return &TestS3SettingAction{runner: runner}
}

// Execute 校验配置并测试目标存储桶是否可访问。
func (a *TestS3SettingAction) Execute(ctx context.Context, input S3Setting) error {
	input, fields := normalizeS3Setting(input)
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

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
	probe := connectiontest.ProbeFunc(func(testCtx context.Context) error {
		_, err := client.HeadBucket(testCtx, &s3.HeadBucketInput{Bucket: aws.String(input.Bucket)})
		if err == nil {
			return nil
		}
		var responseError interface{ HTTPStatusCode() int }
		if errors.As(err, &responseError) {
			return connectiontest.HTTPStatusError(responseError.HTTPStatusCode())
		}
		return connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	})
	return a.runner.Run(ctx, connectiontest.Target{
		Category: connectiontest.CategoryObjectStorage,
		Adapter:  string(input.Provider),
		Location: connectiontest.LocationServer,
	}, probe)
}
