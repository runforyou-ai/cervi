//go:build server

// Package health 提供企业服务端存活和就绪探针。
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/uptrace/bun"
)

const readinessTimeout = 3 * time.Second

type response struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// Liveness 提供不依赖外部资源的进程存活探针。
type Liveness struct {
	version string
}

// NewLiveness 创建进程存活探针。
func NewLiveness(version string) *Liveness {
	return &Liveness{version: version}
}

// ServeHTTP 返回进程存活状态。
func (s *Liveness) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	writeResponse(writer, request, http.StatusOK, response{Status: "ok", Version: s.version})
}

// Readiness 提供依赖 PostgreSQL 的服务就绪探针。
type Readiness struct {
	db      *bun.DB
	version string
}

// NewReadiness 创建服务就绪探针。
func NewReadiness(db *bun.DB, version string) *Readiness {
	return &Readiness{db: db, version: version}
}

// ServeHTTP 检查 PostgreSQL 后返回服务就绪状态。
func (s *Readiness) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		writer.Header().Set("Allow", "GET, HEAD")
		http.Error(writer, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), readinessTimeout)
	defer cancel()
	if _, err := s.db.ExecContext(ctx, "SELECT 1"); err != nil {
		writeResponse(writer, request, http.StatusServiceUnavailable, response{Status: "unavailable", Version: s.version})
		return
	}
	writeResponse(writer, request, http.StatusOK, response{Status: "ready", Version: s.version})
}

// writeResponse 输出统一的探针响应。
func writeResponse(writer http.ResponseWriter, request *http.Request, status int, body response) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	if request.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(writer).Encode(body)
}
