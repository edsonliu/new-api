package seedance

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func newID(prefix string) string { return prefix + strings.ReplaceAll(uuid.NewString(), "-", "") }
func userID(c *gin.Context) int  { return common.GetContextKeyInt(c, constant.ContextKeyUserId) }
func writeError(c *gin.Context, err error) {
	e := &APIError{500, "internal_error", "Unable to complete the operation"}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		e = &APIError{404, "not_found", "Resource not found"}
	} else {
		var known *APIError
		if errors.As(err, &known) {
			e = known
		}
	}
	if e.Code == "upstream_resource_busy" || e.Code == "upstream_rate_limited" {
		c.Header("Retry-After", "2")
	}
	c.AbortWithStatusJSON(e.Status, gin.H{"error": gin.H{"code": e.Code, "message": TranslateErrorMessage(c, e.Message)}})
}
func configForRequest(c *gin.Context) (Config, bool) {
	cfg, err := LoadConfig()
	if err != nil {
		writeError(c, apiError(503, "configuration_error", "Seedance services are not configured correctly"))
		return cfg, false
	}
	if !cfg.Enabled {
		writeError(c, apiError(503, "feature_disabled", "Seedance material services are disabled"))
		return cfg, false
	}
	if userID(c) <= 0 {
		writeError(c, apiError(401, "unauthorized", "Authentication required"))
		return cfg, false
	}
	return cfg, true
}
func decode(c *gin.Context, out any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return apiError(413, "request_too_large", "JSON request exceeds 1 MiB")
	}
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	if err := common.Unmarshal(raw, out); err != nil {
		return apiError(400, "invalid_request", "Invalid JSON request")
	}
	return nil
}
func result(c *gin.Context, action string, data any) {
	c.JSON(200, gin.H{"ResponseMetadata": gin.H{"RequestId": c.GetString(common.RequestIdKey), "Action": action, "Version": APIVersion}, "Result": data})
}
