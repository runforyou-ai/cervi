//go:build server

package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	cervii18n "github.com/runforyou-ai/cervi/internal/i18n"
)

// positiveQueryInteger 读取正整数查询参数。
func positiveQueryInteger(c *gin.Context, name string, defaultValue int) (int, bool) {
	value := c.Query(name)
	if value == "" {
		return defaultValue, true
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		writeError(c, http.StatusBadRequest, "VALIDATION_FAILED", cervii18n.ErrorValidationFailed, map[string]cervii18n.Key{
			name: cervii18n.FieldQueryPositiveInteger,
		})
		return 0, false
	}
	return parsed, true
}
