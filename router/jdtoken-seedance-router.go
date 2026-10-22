package router

import (
	"context"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"net/http"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/jdtoken/seedance"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// A NoRoute extension avoids reserving native plugin paths in the host's
// static-route registry. Existing submit/retrieve routes still belong to the
// plugin; only the explicit missing methods below are handled here.
type seedanceDispatchContextKey struct{}
type seedanceDispatchContext struct{ requestID, language string }

func jdSeedanceDispatcher(next gin.HandlerFunc) gin.HandlerFunc {
	engine := gin.New()
	trustedProxies, _, err := common.ResolveTrustedProxies(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		panic(err)
	}
	if err = common.ConfigureTrustedProxies(engine, trustedProxies); err != nil {
		panic(err)
	}
	engine.Use(func(c *gin.Context) {
		if state, ok := c.Request.Context().Value(seedanceDispatchContextKey{}).(seedanceDispatchContext); ok {
			if state.requestID != "" {
				c.Set(common.RequestIdKey, state.requestID)
			}
			if state.language != "" {
				common.SetContextKey(c, constant.ContextKeyLanguage, state.language)
			}
		}
		c.Next()
	})
	engine.RedirectTrailingSlash = false
	engine.Use(gin.Recovery(), middleware.RouteTag("relay"), middleware.BodyStorageCleanup())
	protected := engine.Group("", middleware.TokenAuth(), middleware.SystemPerformanceCheck(), middleware.GlobalAPIRateLimit())
	protected.POST("/doubao/api/v3/assets/", seedance.Assets)
	protected.POST("/doubao/api/v3/assets", seedance.Assets)
	protected.GET("/v1/seedance/capabilities", seedance.Capabilities)
	protected.POST("/v1/seedance/uploads/presign", seedance.PresignUpload)
	protected.POST("/v1/seedance/uploads/:upload_id/complete", seedance.CompleteUpload)
	protected.GET("/doubao/api/v3/contents/generations/tasks", seedance.ListTasks)
	protected.DELETE("/doubao/api/v3/contents/generations/tasks/:task_id", seedance.DeleteTask)
	protected.GET("/v1/videos", seedance.ListTasks)
	protected.DELETE("/v1/videos/:task_id", seedance.DeleteTask)
	engine.PUT("/v1/seedance/uploads/:upload_id/content", seedance.UploadContent)
	engine.GET("/v1/seedance/uploads/:upload_id/content", seedance.UploadContent)
	engine.HEAD("/v1/seedance/uploads/:upload_id/content", seedance.UploadContent)
	return func(c *gin.Context) {
		p, m := c.Request.URL.Path, c.Request.Method
		native := "/doubao/api/v3/contents/generations/tasks"
		owns := strings.HasPrefix(p, "/v1/seedance/") || p == "/doubao/api/v3/assets" || p == "/doubao/api/v3/assets/" ||
			(m == http.MethodGet && (p == native || p == "/v1/videos")) ||
			(m == http.MethodDelete && (strings.HasPrefix(p, native+"/") || strings.HasPrefix(p, "/v1/videos/")))
		if owns {
			state := seedanceDispatchContext{c.GetString(common.RequestIdKey), common.GetContextKeyString(c, constant.ContextKeyLanguage)}
			request := c.Request.WithContext(context.WithValue(c.Request.Context(), seedanceDispatchContextKey{}, state))
			engine.ServeHTTP(c.Writer, request)
			c.Abort()
			return
		}
		next(c)
	}
}
