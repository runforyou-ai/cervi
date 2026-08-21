//go:build server

package filestore

import (
	"context"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const signedRequestLifetime = 15 * time.Minute

// S3Config 定义访问 S3 兼容对象存储所需配置。
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

// SignedRequest 定义客户端直传或直读对象存储所需请求。
type SignedRequest struct {
	Method  string
	URL     string
	Headers map[string]string
}

// ObjectInfo 返回对象存储中的文件元数据。
type ObjectInfo struct {
	ByteSize    int64
	ContentType string
	ETag        string
}

// PresignPut 创建对象直传请求。
func PresignPut(ctx context.Context, config S3Config, key, contentType string) (SignedRequest, error) {
	presigned, err := s3.NewPresignClient(newS3Client(config)).PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.Bucket), Key: aws.String(key), ContentType: aws.String(contentType),
	}, func(options *s3.PresignOptions) {
		options.Expires = signedRequestLifetime
	})
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{
		Method:  http.MethodPut,
		URL:     presigned.URL,
		Headers: flattenHeaders(presigned.SignedHeader),
	}, nil
}

// PresignGet 创建对象直读请求。
func PresignGet(ctx context.Context, config S3Config, key, contentType, fileName string) (SignedRequest, error) {
	presigned, err := s3.NewPresignClient(newS3Client(config)).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(config.Bucket), Key: aws.String(key), ResponseContentType: aws.String(contentType),
		ResponseContentDisposition: aws.String(contentDisposition(contentType, fileName)),
	}, func(options *s3.PresignOptions) {
		options.Expires = signedRequestLifetime
	})
	if err != nil {
		return SignedRequest{}, err
	}
	return SignedRequest{Method: http.MethodGet, URL: presigned.URL, Headers: flattenHeaders(presigned.SignedHeader)}, nil
}

// Stat 读取对象元数据。
func Stat(ctx context.Context, config S3Config, key string) (ObjectInfo, error) {
	output, err := newS3Client(config).HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(config.Bucket), Key: aws.String(key)})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{ByteSize: aws.ToInt64(output.ContentLength), ContentType: aws.ToString(output.ContentType), ETag: aws.ToString(output.ETag)}, nil
}

// newS3Client 创建 S3 兼容客户端。
func newS3Client(config S3Config) *s3.Client {
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(config.Endpoint), Region: config.Region,
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, "")),
		UsePathStyle: config.ForcePathStyle,
	})
}

// flattenHeaders 将签名请求头转换成前端契约。
func flattenHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, len(headers))
	for name, values := range headers {
		if http.CanonicalHeaderKey(name) == "Host" {
			continue
		}
		if len(values) > 0 {
			result[name] = values[0]
		}
	}
	return result
}

// contentDisposition 返回浏览器展示文件时使用的响应方式。
func contentDisposition(contentType, fileName string) string {
	disposition := "attachment"
	if strings.HasPrefix(contentType, "image/") || contentType == "application/pdf" {
		disposition = "inline"
	}
	return mime.FormatMediaType(disposition, map[string]string{"filename": fileName})
}
