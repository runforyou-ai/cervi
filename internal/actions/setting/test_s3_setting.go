//go:build server

package setting

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/runforyou-ai/cervi/internal/integration/connectiontest"
	serverfilecontent "github.com/runforyou-ai/cervi/internal/storage/server/filecontent"
)

const s3ProbeOrigin = "https://cervi.invalid"

// S3TestFailure 标识对象存储探针失败的具体能力。
type S3TestFailure string

const (
	S3TestFailureBucketAccess S3TestFailure = "bucket_access"
	S3TestFailureCORS         S3TestFailure = "cors"
	S3TestFailureUpload       S3TestFailure = "upload"
	S3TestFailurePublicAccess S3TestFailure = "public_access"
	S3TestFailureCleanup      S3TestFailure = "cleanup"
)

// s3TestError 保留对象存储探针的业务失败阶段和底层连接错误。
type s3TestError struct {
	failure S3TestFailure
	err     error
}

func (e *s3TestError) Error() string {
	return e.err.Error()
}

func (e *s3TestError) Unwrap() error {
	return e.err
}

// S3TestFailureOf 返回对象存储探针失败的具体能力。
func S3TestFailureOf(err error) (S3TestFailure, bool) {
	var testError *s3TestError
	if !errors.As(err, &testError) {
		return "", false
	}
	return testError.failure, true
}

func newS3TestError(failure S3TestFailure, err error) error {
	return &s3TestError{failure: failure, err: err}
}

// TestS3SettingAction 测试 S3 对象存储配置。
type TestS3SettingAction struct {
	runner *connectiontest.Runner
}

// NewTestS3SettingAction 创建 S3 配置测试操作。
func NewTestS3SettingAction(runner *connectiontest.Runner) *TestS3SettingAction {
	return &TestS3SettingAction{runner: runner}
}

// Execute 校验配置并测试目标存储桶的上传、跨域和公开读取能力。
func (a *TestS3SettingAction) Execute(ctx context.Context, organizationID string, input S3Setting) error {
	input, fields := normalizeS3Setting(input)
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}

	config := serverfilecontent.S3Config{
		Endpoint: input.Endpoint, Region: input.Region, Bucket: input.Bucket,
		AccessKeyID: input.AccessKeyID, SecretAccessKey: input.SecretAccessKey, ForcePathStyle: input.ForcePathStyle,
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
		if err != nil {
			return newS3TestError(S3TestFailureBucketAccess, s3ProbeError(err))
		}
		return testS3ObjectFlow(testCtx, config, input.PublicBaseURL, organizationID)
	})
	return a.runner.Run(ctx, connectiontest.Target{
		Category: connectiontest.CategoryObjectStorage,
		Adapter:  string(input.Provider),
		Location: connectiontest.LocationServer,
	}, probe)
}

// testS3ObjectFlow 使用正式请求语义验证浏览器直传和公开读取。
func testS3ObjectFlow(ctx context.Context, config serverfilecontent.S3Config, publicBaseURL, organizationID string) (probeErr error) {
	probeID := uuid.NewString()
	storageKey := "organizations/" + organizationID + "/files/" + probeID + ".txt"
	payload := []byte("cervi-object-storage-probe:" + probeID)
	signed, err := serverfilecontent.PresignPut(ctx, config, storageKey, "text/plain; charset=utf-8")
	if err != nil {
		return newS3TestError(S3TestFailureUpload, connectiontest.InvalidConfigError(err))
	}
	publicURL, err := serverfilecontent.PublicURL(publicBaseURL, storageKey)
	if err != nil {
		return newS3TestError(S3TestFailurePublicAccess, connectiontest.InvalidConfigError(err))
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := serverfilecontent.Delete(cleanupCtx, config, storageKey); err != nil && probeErr == nil {
			probeErr = newS3TestError(S3TestFailureCleanup, s3ProbeError(err))
		}
	}()

	httpClient := connectiontest.NewHTTPClient()
	requestedHeaders := probeRequestHeaderNames(signed.Headers)
	if err := testS3PutPreflight(ctx, httpClient, signed.URL, requestedHeaders); err != nil {
		return newS3TestError(S3TestFailureCORS, err)
	}
	if err := putS3Probe(ctx, httpClient, signed, payload); err != nil {
		return err
	}
	if err := getS3PublicProbe(ctx, httpClient, publicURL, payload); err != nil {
		return newS3TestError(S3TestFailurePublicAccess, err)
	}
	return nil
}

// testS3PutPreflight 验证浏览器对正式预签名上传请求的 CORS 预检。
func testS3PutPreflight(ctx context.Context, client connectiontest.HTTPDoer, uploadURL string, requestedHeaders []string) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodOptions, uploadURL, nil)
	if err != nil {
		return connectiontest.InvalidConfigError(err)
	}
	request.Header.Set("Origin", s3ProbeOrigin)
	request.Header.Set("Access-Control-Request-Method", http.MethodPut)
	request.Header.Set("Access-Control-Request-Headers", strings.Join(requestedHeaders, ", "))
	response, err := client.Do(request)
	if err != nil {
		return connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return connectiontest.HTTPStatusError(response.StatusCode)
	}
	if !corsAllowsOrigin(response.Header, s3ProbeOrigin) || !corsAllowsToken(response.Header.Values("Access-Control-Allow-Methods"), http.MethodPut) {
		return s3ProbeProtocolError("object storage CORS does not allow the upload origin or PUT method")
	}
	for _, name := range requestedHeaders {
		if !corsAllowsToken(response.Header.Values("Access-Control-Allow-Headers"), name) {
			return s3ProbeProtocolError("object storage CORS does not allow signed upload header " + name)
		}
	}
	return nil
}

// putS3Probe 执行带 Origin 的正式预签名上传并验证实际 CORS 响应。
func putS3Probe(ctx context.Context, client connectiontest.HTTPDoer, signed serverfilecontent.SignedRequest, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, signed.Method, signed.URL, bytes.NewReader(payload))
	if err != nil {
		return newS3TestError(S3TestFailureUpload, connectiontest.InvalidConfigError(err))
	}
	for name, value := range signed.Headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Origin", s3ProbeOrigin)
	response, err := client.Do(request)
	if err != nil {
		return newS3TestError(S3TestFailureUpload, connectiontest.ClassifyTransportError(connectiontest.StageConnect, err))
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return newS3TestError(S3TestFailureUpload, connectiontest.HTTPStatusError(response.StatusCode))
	}
	if !corsAllowsOrigin(response.Header, s3ProbeOrigin) {
		return newS3TestError(S3TestFailureCORS, s3ProbeProtocolError("object storage upload response does not allow the upload origin"))
	}
	return nil
}

// getS3PublicProbe 匿名读取公开地址并核对探针内容。
func getS3PublicProbe(ctx context.Context, client connectiontest.HTTPDoer, publicURL string, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, publicURL, nil)
	if err != nil {
		return connectiontest.InvalidConfigError(err)
	}
	response, err := client.Do(request)
	if err != nil {
		return connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return connectiontest.HTTPStatusError(response.StatusCode)
	}
	actual, err := io.ReadAll(io.LimitReader(response.Body, int64(len(payload)+1)))
	if err != nil {
		return connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
	}
	if !bytes.Equal(actual, payload) {
		return s3ProbeProtocolError("public object response does not match the uploaded probe")
	}
	return nil
}

// probeRequestHeaderNames 返回浏览器预检需要声明的正式上传请求头。
func probeRequestHeaderNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" && name != "host" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// corsAllowsOrigin 判断响应是否允许指定浏览器来源。
func corsAllowsOrigin(headers http.Header, origin string) bool {
	for _, value := range headers.Values("Access-Control-Allow-Origin") {
		if value == "*" || value == origin {
			return true
		}
	}
	return false
}

// corsAllowsToken 判断逗号分隔的 CORS 响应头是否允许指定值。
func corsAllowsToken(values []string, expected string) bool {
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if token == "*" || strings.EqualFold(token, expected) {
				return true
			}
		}
	}
	return false
}

// s3ProbeError 把 S3 SDK 错误归一为连接测试错误。
func s3ProbeError(err error) error {
	var responseError interface{ HTTPStatusCode() int }
	if errors.As(err, &responseError) {
		return connectiontest.HTTPStatusError(responseError.HTTPStatusCode())
	}
	return connectiontest.ClassifyTransportError(connectiontest.StageConnect, err)
}

// s3ProbeProtocolError 返回对象存储能力不满足浏览器直传契约的错误。
func s3ProbeProtocolError(message string) error {
	return connectiontest.NewError(connectiontest.StageCapability, connectiontest.FailureProtocol, errors.New(message))
}
