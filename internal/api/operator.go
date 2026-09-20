//go:build server

package api

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"uuid"
)

// 运营接口的稳定错误码，供 SaaS 按语义处理。
const (
	operatorCodeInvalidCredential = "invalid_operator_credential"
)

// requestIDHeader 是运营调用方传入的请求关联标识。
const requestIDHeader = "X-Request-Id"

type operatorErrorBody struct {
	Error operatorError `json:"error"`
}

type operatorError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// Deployment 描述本部署的形态与企业域名后缀。
type Deployment struct {
	Mode                string `json:"mode"`
	ManagedDomainSuffix string `json:"managedDomainSuffix"`
}

// OperatorService 是官方托管运营接口的 Gin 适配器，仅在托管部署注册。
type OperatorService struct {
	deployment Deployment
	credential string
	router     *gin.Engine
}

// NewOperatorService 创建运营接口适配器。
func NewOperatorService(deployment Deployment, credential string) *OperatorService {
	service := &OperatorService{deployment: deployment, credential: credential}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery(), service.authenticate)
	// 返回部署形态和企业域名后缀，供 SaaS 确认调用目标。
	router.GET("/deployment", func(c *gin.Context) {
		c.JSON(http.StatusOK, service.deployment)
	})
	service.router = router
	return service
}

// ServeHTTP 将运营请求交给 Gin 路由处理。
func (s *OperatorService) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.router.ServeHTTP(writer, request)
}

// authenticate 校验运营服务凭据，凭据是运营接口的唯一访问控制手段。
func (s *OperatorService) authenticate(c *gin.Context) {
	requestID := strings.TrimSpace(c.GetHeader(requestIDHeader))
	if requestID == "" {
		requestID = uuid.New().String()
	}
	c.Set(requestIDHeader, requestID)
	credential := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(credential)), []byte(s.credential)) != 1 {
		slog.Warn("运营凭据无效", "request_id", requestID, "path", c.Request.URL.Path)
		c.AbortWithStatusJSON(http.StatusUnauthorized, operatorErrorBody{Error: operatorError{
			Code: operatorCodeInvalidCredential, Message: "运营凭据无效。", RequestID: requestID,
		}})
		return
	}
	c.Next()
}
