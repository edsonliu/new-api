package seedance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The same business contract is run against real SQLite, MySQL and PostgreSQL
// by setting JD_SEEDANCE_TEST_DSN to a disposable database. Never point at prod.
func setupFeature(t *testing.T) (*gin.Engine, *model.Channel) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	require.NoError(t, i18n.Init())
	oldPerf := perf_metrics_setting.GetSetting().Enabled
	perf := config.GlobalConfig.Get("perf_metrics_setting")
	require.NoError(t, config.UpdateConfigFromMap(perf, map[string]string{"enabled": "false"}))
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(perf, map[string]string{"enabled": strconv.FormatBool(oldPerf)}))
	})
	oldDB, oldLog := model.DB, model.LOG_DB
	oldPath, oldMaster, oldMemory := common.SQLitePath, common.IsMasterNode, common.MemoryCacheEnabled
	oldBatch, oldRedis, oldConsume := common.BatchUpdateEnabled, common.RedisEnabled, common.LogConsumeEnabled
	dsn := os.Getenv("JD_SEEDANCE_TEST_DSN")
	if dsn != "" {
		require.True(t, (strings.Contains(dsn, "@127.0.0.1:") || strings.Contains(dsn, "@tcp(127.0.0.1:")) && (strings.Contains(dsn, "/seedance_test?") || strings.HasSuffix(dsn, "/seedance_test")), "database tests require a disposable loopback seedance_test database")
	}
	t.Setenv("SQL_DSN", dsn)
	t.Setenv("LOG_SQL_DSN", "")
	common.SQLitePath = filepath.Join(t.TempDir(), "feature.db")
	common.IsMasterNode = true
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.RedisEnabled = false
	common.LogConsumeEnabled = false
	require.NoError(t, model.InitDB())
	require.NoError(t, model.InitLogDB())
	db := model.DB
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// Each test owns this disposable database; preserving schema across cases also
	// exercises repeated migrations. Remove only this fixture's test records.
	for _, table := range []string{"jdtoken_seedance_verifications", "jdtoken_seedance_resources", "jdtoken_seedance_uploads", "jdtoken_seedance_task_deletions", "tasks", "abilities", "channels", "tokens", "users", "logs"} {
		require.NoError(t, db.Exec("DELETE FROM "+table).Error)
	}
	t.Cleanup(func() {
		sqlDB.Close()
		model.DB = oldDB
		model.LOG_DB = oldLog
		common.SQLitePath = oldPath
		common.IsMasterNode = oldMaster
		common.MemoryCacheEnabled = oldMemory
		common.BatchUpdateEnabled = oldBatch
		common.RedisEnabled = oldRedis
		common.LogConsumeEnabled = oldConsume
	})
	common.OptionMapRWMutex.Lock()
	oldCfg := common.OptionMap[ConfigKey]
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	common.OptionMap[ConfigKey] = ""
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[ConfigKey] = oldCfg
		common.OptionMapRWMutex.Unlock()
	})
	service.InitHttpClient()
	fetch := system_setting.GetFetchSetting()
	oldFetch := *fetch
	fetch.EnableSSRFProtection = false
	t.Cleanup(func() { *fetch = oldFetch })
	ch := &model.Channel{Id: 31, Name: "test provider", Type: constant.ChannelTypeDoubaoVideo, Key: "test-only-key", Status: common.ChannelStatusEnabled, Models: "seedance-test", Group: "default"}
	require.NoError(t, db.Create(ch).Error)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "seedance-test", ChannelId: ch.Id, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.User{Id: 11, Username: "owner", AffCode: "owner", Group: "default", Quota: 1000, UsedQuota: 100}).Error)
	require.NoError(t, db.Create(&model.User{Id: 22, Username: "other", AffCode: "other", Group: "default", Quota: 1000}).Error)
	cfg := Config{Enabled: true, Bindings: []Binding{{Name: "primary", ChannelID: 31, Provider: "lingxi", StoreTypes: []string{"Image", "Video"}, Models: map[string]ModelCapabilities{"seedance-test": {ReferenceTypes: []string{"Image", "Video"}, Cancel: true}}}}}
	setConfig(t, cfg)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		id := 11
		if c.GetHeader("Test-User") == "other" {
			id = 22
		}
		common.SetContextKey(c, constant.ContextKeyUserId, id)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
		if c.GetHeader("Test-Restricted") == "true" {
			common.SetContextKey(c, constant.ContextKeyTokenModelLimitEnabled, true)
			c.Set("token_model_limit", map[string]bool{"different": true})
		}
		c.Next()
		common.CleanupBodyStorage(c)
	})
	r.POST("/doubao/api/v3/assets/", Assets)
	r.GET("/v1/seedance/capabilities", Capabilities)
	r.POST("/v1/seedance/uploads/presign", PresignUpload)
	r.POST("/v1/seedance/uploads/:upload_id/complete", CompleteUpload)
	r.PUT("/v1/seedance/uploads/:upload_id/content", UploadContent)
	r.GET("/v1/seedance/uploads/:upload_id/content", UploadContent)
	r.HEAD("/v1/seedance/uploads/:upload_id/content", UploadContent)
	r.GET("/doubao/api/v3/contents/generations/tasks", ListTasks)
	r.DELETE("/doubao/api/v3/contents/generations/tasks/:task_id", DeleteTask)
	r.POST("/doubao/api/v3/contents/generations/tasks", PrepareReferences(), func(c *gin.Context) {
		var body map[string]any
		require.NoError(t, common.UnmarshalBodyReusable(c, &body))
		c.JSON(200, body)
	})
	return r, ch
}
func setConfig(t *testing.T, cfg Config) {
	t.Helper()
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[ConfigKey] = string(raw)
	common.OptionMapRWMutex.Unlock()
}
func request(t *testing.T, r http.Handler, method, path string, body any, headers map[string]string) (int, map[string]any) {
	t.Helper()
	raw, err := common.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &out), w.Body.String())
	return w.Code, out
}
func action(t *testing.T, r http.Handler, name string, body any, key string, other bool) (int, map[string]any) {
	t.Helper()
	headers := map[string]string{"Idempotency-Key": key}
	if other {
		headers["Test-User"] = "other"
	}
	return request(t, r, "POST", "/doubao/api/v3/assets/?Action="+name+"&Version="+APIVersion, body, headers)
}
func resultID(t *testing.T, code int, out map[string]any) string {
	t.Helper()
	require.Equal(t, 200, code, out)
	v, ok := out["Result"].(map[string]any)
	require.True(t, ok, out)
	id, ok := v["Id"].(string)
	require.True(t, ok, out)
	return id
}

type assetFixture struct {
	mu      sync.Mutex
	rows    map[string]map[string]any
	calls   map[string]int
	path    string
	html    bool
	delay   time.Duration
	reuseID string
}

func newProvider(t *testing.T, ch *model.Channel, path string) *assetFixture {
	t.Helper()
	f := &assetFixture{rows: map[string]map[string]any{}, calls: map[string]int{}, path: path}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		assert.Equal(t, "Bearer test-only-key", r.Header.Get("Authorization"))
		assert.Equal(t, f.path, r.URL.Path)
		if f.html {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>not an API</html>"))
			return
		}
		var in map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &in))
		a := r.URL.Query().Get("Action")
		f.calls[a]++
		time.Sleep(f.delay)
		w.Header().Set("Content-Type", "application/json")
		id, _ := in["Id"].(string)
		row := f.rows[id]
		out := map[string]any{}
		switch a {
		case "CreateAssetGroup", "CreateAsset":
			if a == "CreateAsset" && f.reuseID != "" {
				out = map[string]any{"Id": f.reuseID}
				break
			}
			id = "provider-" + strconv.Itoa(len(f.rows)+f.calls[a]+1)
			row = map[string]any{"Id": id, "Name": in["Name"], "Status": "Active", "URL": "https://provider.invalid/private-link", "GroupId": in["GroupId"]}
			f.rows[id] = row
			out = map[string]any{"Id": id} // Status is optional on creation.
		case "GetAssetGroup", "GetAsset":
			if row == nil {
				w.WriteHeader(404)
				w.Write([]byte(`{"error":{"code":"NotFound"}}`))
				return
			}
			out = row
		case "UpdateAssetGroup", "UpdateAsset":
			for k, v := range in {
				row[k] = v
			}
		case "DeleteAssetGroup", "DeleteAsset":
			delete(f.rows, id)
		default:
			t.Errorf("unexpected upstream action %s: shared lists must stay local", a)
		}
		raw, err := common.Marshal(map[string]any{"Result": out})
		require.NoError(t, err)
		w.Write(raw)
	}))
	t.Cleanup(srv.Close)
	ch.BaseURL = &srv.URL
	require.NoError(t, model.DB.Model(ch).Update("base_url", srv.URL).Error)
	return f
}

func TestMaterialCRUDIsolationRoutingAndIdempotency(t *testing.T) {
	for _, tc := range []struct{ provider, path string }{{"lingxi", "/api/v3/assets/"}, {"mobilecloud", "/"}, {"shujubao", "/v1/api/assets/"}} {
		t.Run(tc.provider, func(t *testing.T) {
			r, ch := setupFeature(t)
			cfg, err := LoadConfig()
			require.NoError(t, err)
			cfg.Bindings[0].Provider = tc.provider
			setConfig(t, cfg)
			provider := newProvider(t, ch, tc.path)
			code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "group"}, "g1", false)
			gid := resultID(t, code, out)
			code, out = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "group"}, "g1", false)
			assert.Equal(t, gid, resultID(t, code, out))
			assert.Equal(t, 1, provider.calls["CreateAssetGroup"])
			for _, a := range []string{"GetAssetGroup", "UpdateAssetGroup", "DeleteAssetGroup"} {
				code, _ = action(t, r, a, map[string]any{"Id": gid, "Name": "stolen"}, "", true)
				assert.Equal(t, 404, code)
			}
			code, out = action(t, r, "UpdateAssetGroup", map[string]any{"Id": gid, "Name": "renamed", "Description": "description"}, "", false)
			require.Equal(t, 200, code, out)
			code, out = action(t, r, "GetAssetGroup", map[string]any{"Id": gid}, "", false)
			require.Equal(t, 200, code, out)
			assert.Equal(t, "renamed", out["Result"].(map[string]any)["Name"])
			code, out = action(t, r, "ListAssetGroups", map[string]any{}, "", true)
			require.Equal(t, 200, code, out)
			assert.Equal(t, float64(0), out["Result"].(map[string]any)["TotalCount"])
			code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/input.png", "Name": "image"}, "a1", false)
			aid := resultID(t, code, out)
			assert.Equal(t, "Processing", out["Result"].(map[string]any)["Status"])
			code, out = action(t, r, "GetAsset", map[string]any{"Id": aid}, "", false)
			require.Equal(t, 200, code, out)
			view := out["Result"].(map[string]any)
			assert.Equal(t, "Active", view["Status"])
			assert.Equal(t, gid, view["GroupId"])
			assert.Equal(t, "https://provider.invalid/private-link", view["URL"])
			code, out = action(t, r, "UpdateAsset", map[string]any{"Id": aid, "Name": "new name"}, "", false)
			require.Equal(t, 200, code, out)
			for _, a := range []string{"GetAsset", "UpdateAsset", "DeleteAsset"} {
				code, _ = action(t, r, a, map[string]any{"Id": aid}, "", true)
				assert.Equal(t, 404, code)
			}
			code, out = action(t, r, "ListAssets", map[string]any{"GroupIds": []string{gid}, "PageSize": 1}, "", false)
			require.Equal(t, 200, code, out)
			assert.Equal(t, float64(1), out["Result"].(map[string]any)["TotalCount"])
			body := map[string]any{"model": "seedance-test", "content": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "asset://" + aid}}}, "duration": 4, "generate_audio": false, "seed": 0}
			code, out = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", body, nil)
			require.Equal(t, 200, code, out)
			ref := out["content"].([]any)[0].(map[string]any)["image_url"].(map[string]any)["url"].(string)
			assert.True(t, strings.HasPrefix(ref, "asset://provider-"))
			assert.Equal(t, false, out["generate_audio"])
			assert.Equal(t, float64(0), out["seed"])
			code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", body, map[string]string{"Test-User": "other"})
			assert.Equal(t, 404, code)
			code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", body, map[string]string{"Test-Restricted": "true"})
			assert.Equal(t, 403, code)
			body["frames"] = 97
			code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", body, nil)
			assert.Equal(t, 422, code)
			delete(body, "frames")
			require.NoError(t, model.DB.Model(ch).Update("key", "rotated-key").Error)
			code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", body, nil)
			assert.Equal(t, 409, code)
			require.NoError(t, model.DB.Model(ch).Update("key", "test-only-key").Error)
			code, out = action(t, r, "DeleteAsset", map[string]any{"Id": aid}, "", false)
			require.Equal(t, 200, code, out)
			code, _ = action(t, r, "GetAsset", map[string]any{"Id": aid}, "", false)
			assert.Equal(t, 404, code)
			code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Video", "URL": "https://93.184.215.14/input.mp4"}, "a2", false)
			aid2 := resultID(t, code, out)
			code, out = action(t, r, "DeleteAssetGroup", map[string]any{"Id": gid}, "", false)
			require.Equal(t, 200, code, out)
			code, _ = action(t, r, "GetAsset", map[string]any{"Id": aid2}, "", false)
			assert.Equal(t, 404, code)
			code, out = action(t, r, "ListAssets", map[string]any{}, "", false)
			require.Equal(t, 200, code, out)
			assert.Equal(t, float64(0), out["Result"].(map[string]any)["TotalCount"])
		})
	}
}

func TestMobilecloudAdapterAndUnconfirmedCreation(t *testing.T) {
	r, ch := setupFeature(t)
	provider := newProvider(t, ch, "/")
	cfg, err := LoadConfig()
	require.NoError(t, err)
	cfg.Bindings[0].Provider = "mobilecloud"
	setConfig(t, cfg)
	code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "mobile"}, "mobile", false)
	gid := resultID(t, code, out)
	code, out = action(t, r, "GetAssetGroup", map[string]any{"Id": gid}, "", false)
	require.Equal(t, 200, code, out)
	provider.mu.Lock()
	provider.html = true
	provider.mu.Unlock()
	code, _ = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "uncertain"}, "uncertain", false)
	assert.Equal(t, 502, code)
	code, _ = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "uncertain"}, "uncertain", false)
	assert.Equal(t, 409, code)
	code, _ = request(t, r, "POST", "/doubao/api/v3/assets/?Action=ListAssets&Version=1900-01-01", map[string]any{}, nil)
	assert.Equal(t, 400, code)
	code, out = request(t, r, "GET", "/v1/seedance/capabilities", nil, map[string]string{"Test-Restricted": "true"})
	require.Equal(t, 200, code, out)
	assert.Empty(t, out["data"])
}

func TestUploadSignaturesOwnershipAndIngestion(t *testing.T) {
	r, ch := setupFeature(t)
	newProvider(t, ch, "/api/v3/assets/")
	t.Setenv("JDOTOKEN_SEEDANCE_UPLOAD_DIR", t.TempDir())
	t.Setenv("JDOTOKEN_SEEDANCE_PUBLIC_URL", "https://files.example.com")
	t.Setenv("JDOTOKEN_SEEDANCE_UPLOAD_SECRET", strings.Repeat("a", 32))
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var imageBytes bytes.Buffer
	require.NoError(t, png.Encode(&imageBytes, img))
	blob := imageBytes.Bytes()
	code, out := request(t, r, "POST", "/v1/seedance/uploads/presign", map[string]any{"service": "primary", "model": "seedance-test", "content_type": "image/png", "size": len(blob)}, nil)
	require.Equal(t, 200, code, out)
	id := out["id"].(string)
	uploadURL := out["upload_url"].(string)
	parsed, err := url.Parse(uploadURL)
	require.NoError(t, err)
	put := func(target string, data []byte) int {
		req := httptest.NewRequest("PUT", target, bytes.NewReader(data))
		req.Header.Set("Content-Type", "image/png")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	assert.Equal(t, 403, put(parsed.Path+"?expires=1&signature=bad", blob))
	assert.Equal(t, 200, put(parsed.RequestURI(), blob))
	assert.Equal(t, 409, put(parsed.RequestURI(), blob))
	code, _ = request(t, r, "POST", "/v1/seedance/uploads/"+id+"/complete", nil, map[string]string{"Test-User": "other"})
	assert.Equal(t, 404, code)
	code, out = request(t, r, "POST", "/v1/seedance/uploads/"+id+"/complete", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Equal(t, "not_ingested", out["material_status"])
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	common.SetContextKey(c, constant.ContextKeyUserId, 11)
	source, err := uploadSource(c, id, "primary", AccountHash(ch))
	require.NoError(t, err)
	get := httptest.NewRequest("GET", source, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, get)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "image/png", w.Header().Get("Content-Type"))
	assert.Equal(t, blob, w.Body.Bytes())
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("HEAD", source, nil))
	assert.Equal(t, 200, w.Code)
	assert.Empty(t, w.Body.Bytes())
	assert.Equal(t, strconv.Itoa(len(blob)), w.Header().Get("Content-Length"))
	code, out = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "upload-group"}, "ug", false)
	gid := resultID(t, code, out)
	code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "UploadId": id}, "ua", false)
	resultID(t, code, out)
	require.NoError(t, model.DB.Model(&model.JDSeedanceUpload{}).Where("id = ?", id).Update("expires_at", time.Now().Add(-time.Hour).Unix()).Error)
	cfg, err := loadUploadConfig()
	require.NoError(t, err)
	require.NoError(t, cleanExpiredUploads(cfg))
	_, err = os.Stat(filepath.Join(cfg.Directory, id))
	assert.True(t, os.IsNotExist(err))
}

type cancelAdaptor struct {
	terminal string
	barrier  chan struct{}
	entered  chan struct{}
}

func (*cancelAdaptor) Init(*relaycommon.RelayInfo) {}
func (a *cancelAdaptor) FetchTask(string, string, *model.Task, string) (*http.Response, error) {
	if a.entered != nil {
		a.entered <- struct{}{}
	}
	if a.barrier != nil {
		<-a.barrier
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"status":"` + a.terminal + `"}`))}, nil
}
func (a *cancelAdaptor) ParseTaskResult(*model.Task, *http.Response, []byte) (*relaycommon.TaskInfo, error) {
	status := model.TaskStatusFailure
	if a.terminal == "succeeded" {
		status = model.TaskStatusSuccess
	}
	return &relaycommon.TaskInfo{Status: string(status), Reason: "Cancelled"}, nil
}
func (*cancelAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int { return 0 }

func TestCancellationUsesNativeSettlementAndDeletionRetainsAudit(t *testing.T) {
	r, ch := setupFeature(t)
	var deletes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, "DELETE", req.Method)
		deletes.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer upstream.Close()
	ch.BaseURL = &upstream.URL
	require.NoError(t, model.DB.Model(ch).Update("base_url", upstream.URL).Error)
	oldFactory := service.GetTaskAdaptorFunc
	adaptor := &cancelAdaptor{terminal: "cancelled"}
	service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { service.GetTaskAdaptorFunc = oldFactory })
	task := &model.Task{TaskID: "task_cancel", UserId: 11, ChannelId: 31, Platform: "doubao", Status: model.TaskStatusQueued, Quota: 100, Group: "default", Properties: model.Properties{OriginModelName: "seedance-test"}, PrivateData: model.TaskPrivateData{Key: ch.Key, SeedanceAccountHash: AccountHash(ch), UpstreamTaskID: "upstream-task"}}
	require.NoError(t, model.DB.Create(task).Error)
	code, _ := request(t, r, "DELETE", "/doubao/api/v3/contents/generations/tasks/task_cancel", nil, map[string]string{"Test-User": "other"})
	assert.Equal(t, 404, code)
	assert.Equal(t, int32(0), deletes.Load())
	code, out := request(t, r, "DELETE", "/doubao/api/v3/contents/generations/tasks/task_cancel", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Equal(t, "cancelled", out["status"])
	var owner model.User
	require.NoError(t, model.DB.First(&owner, 11).Error)
	assert.Equal(t, 1100, owner.Quota)
	code, out = request(t, r, "DELETE", "/doubao/api/v3/contents/generations/tasks/task_cancel", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Equal(t, false, out["upstream_deleted"])
	assert.Equal(t, int32(1), deletes.Load())
	stored, found, err := model.GetByTaskId(11, "task_cancel")
	require.NoError(t, err)
	require.True(t, found)
	assert.False(t, stored.ResultRetrievable())
	assert.Equal(t, 0, stored.Quota)
	code, out = request(t, r, "GET", "/doubao/api/v3/contents/generations/tasks", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Equal(t, float64(0), out["total"])
	require.NoError(t, model.DB.First(&owner, 11).Error)
	assert.Equal(t, 1100, owner.Quota)
}

func TestCancellationAndPollRaceRefundsOnce(t *testing.T) {
	r, ch := setupFeature(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	ch.BaseURL = &srv.URL
	require.NoError(t, model.DB.Model(ch).Update("base_url", srv.URL).Error)
	old := service.GetTaskAdaptorFunc
	a := &cancelAdaptor{terminal: "cancelled", entered: make(chan struct{}, 2), barrier: make(chan struct{})}
	service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return a }
	t.Cleanup(func() { service.GetTaskAdaptorFunc = old })
	task := &model.Task{TaskID: "task_race", UserId: 11, ChannelId: 31, Platform: "doubao", Status: model.TaskStatusQueued, Quota: 100, Group: "default", Properties: model.Properties{OriginModelName: "seedance-test"}, PrivateData: model.TaskPrivateData{Key: ch.Key, SeedanceAccountHash: AccountHash(ch), UpstreamTaskID: "upstream-race"}}
	require.NoError(t, model.DB.Create(task).Error)
	duplicate := *task
	done := make(chan error, 1)
	go func() {
		done <- service.UpdateVideoTasks(context.Background(), "doubao", map[int][]string{31: {"upstream-race"}}, map[string]*model.Task{"upstream-race": &duplicate})
	}()
	<-a.entered
	response := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("DELETE", "/doubao/api/v3/contents/generations/tasks/task_race", nil))
		response <- w
	}()
	<-a.entered
	close(a.barrier)
	require.NoError(t, <-done)
	assert.Equal(t, 200, (<-response).Code)
	var owner model.User
	require.NoError(t, model.DB.First(&owner, 11).Error)
	assert.Equal(t, 1100, owner.Quota)
}

func TestAdditiveMigrationsAndUniqueOwnership(t *testing.T) {
	_, _ = setupFeature(t)
	db := model.DB
	// Existing release tables and an existing wallet survive removing the new
	// extension tables and adding them back, then running startup migrations twice.
	for _, entity := range []any{&model.JDSeedanceResource{}, &model.JDSeedanceUpload{}, &model.JDSeedanceTaskDeletion{}, &model.JDSeedanceVerification{}} {
		require.NoError(t, db.Migrator().DropTable(entity))
	}
	for range 2 {
		require.NoError(t, db.AutoMigrate(&model.JDSeedanceResource{}, &model.JDSeedanceUpload{}, &model.JDSeedanceTaskDeletion{}, &model.JDSeedanceVerification{}))
	}
	var owner model.User
	require.NoError(t, db.First(&owner, 11).Error)
	assert.Equal(t, 1000, owner.Quota)
	one := model.JDSeedanceResource{ID: "jda_one", UserID: 11, Kind: "asset", RequestKey: "request-one", Binding: "primary", AccountHash: "hash"}
	require.NoError(t, db.Create(&one).Error)
	duplicate := one
	duplicate.ID = "jda_two"
	assert.Error(t, db.Create(&duplicate).Error)
	duplicate.UserID = 22
	require.NoError(t, db.Create(&duplicate).Error)
	for range 2 {
		require.NoError(t, db.AutoMigrate(&model.JDSeedanceResource{}, &model.JDSeedanceUpload{}, &model.JDSeedanceTaskDeletion{}, &model.JDSeedanceVerification{}))
	}
	var count int64
	require.NoError(t, db.Model(&model.JDSeedanceResource{}).Count(&count).Error)
	assert.Equal(t, int64(2), count)
	assert.True(t, db.Migrator().HasIndex(&model.JDSeedanceResource{}, "uk_jds_request"))
	if db.Dialector.Name() == "sqlite" {
		var version string
		require.NoError(t, db.Raw("select sqlite_version()").Scan(&version).Error)
		t.Log("SQLite", version)
	} else {
		var version string
		require.NoError(t, db.Raw("select version()").Scan(&version).Error)
		t.Log(db.Dialector.Name(), version)
	}
}

func TestMaterialValidationAndFinalAccountGuard(t *testing.T) {
	r, ch := setupFeature(t)
	newProvider(t, ch, "/api/v3/assets/")
	code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "first"}, "unique", false)
	gid := resultID(t, code, out)
	code, _ = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "different"}, "unique", false)
	assert.Equal(t, 409, code)
	for _, source := range []string{"http://127.0.0.1/private", "http://169.254.169.254/latest/meta-data", "file:///etc/passwd", "https://user:password@example.com/file", "https://93.184.215.14:3306/file"} {
		code, _ = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": source}, newID("request_"), false)
		assert.Equal(t, 400, code, source)
	}
	code, _ = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Audio", "URL": "https://93.184.215.14/input.wav"}, "audio", false)
	assert.Equal(t, 422, code)
	code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/input.png"}, "image", false)
	aid := resultID(t, code, out)
	for _, reference := range []string{"asset://provider-raw", " ASSET://provider-raw ", "upload://jdu_missing"} {
		code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", map[string]any{"model": "seedance-test", "input_reference": reference}, nil)
		assert.Contains(t, []int{400, 404}, code)
	}
	raw, _ := common.Marshal(map[string]any{"model": "seedance-test", "input_reference": "asset://" + aid})
	guard := gin.New()
	guard.Use(func(c *gin.Context) {
		common.SetContextKey(c, constant.ContextKeyUserId, 11)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
		c.Next()
		common.CleanupBodyStorage(c)
	})
	guard.POST("/v1/videos", PrepareReferences(), func(c *gin.Context) {
		require.NoError(t, ValidateSelected(c, ch, ch.Key))
		common.SetContextKey(c, constant.ContextKeyOriginTasks, []*model.Task{{ChannelId: 99, PrivateData: model.TaskPrivateData{SeedanceAccountHash: AccountHash(ch)}}})
		assert.Error(t, ValidateSelected(c, ch, ch.Key))
		common.SetContextKey(c, constant.ContextKeyOriginTasks, []*model.Task{{ChannelId: ch.Id, PrivateData: model.TaskPrivateData{SeedanceAccountHash: "different-account"}}})
		assert.Error(t, ValidateSelected(c, ch, ch.Key))
		common.SetContextKey(c, constant.ContextKeyOriginTasks, []*model.Task{{ChannelId: ch.Id, PrivateData: model.TaskPrivateData{SeedanceAccountHash: AccountHash(ch)}}})
		require.NoError(t, ValidateSelected(c, ch, ch.Key))
		copy := *ch
		copy.Id = 99
		assert.Error(t, ValidateSelected(c, &copy, ch.Key))
		assert.Error(t, ValidateSelected(c, ch, "other-account"))
		c.Status(200)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/videos", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	guard.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, w.Body.String())
	code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", map[string]any{"model": "seedance-test", "content": []any{map[string]any{"type": "video_url", "video_url": map[string]any{"url": "asset://" + aid}}}}, nil)
	assert.Equal(t, 422, code)
	mapping := `{"seedance-test":"different-upstream-model"}`
	require.NoError(t, model.DB.Model(ch).Update("model_mapping", mapping).Error)
	code, out = request(t, r, "GET", "/v1/seedance/capabilities", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Empty(t, out["data"])
	code, _ = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", map[string]any{"model": "seedance-test", "input_reference": "asset://" + aid}, nil)
	assert.Equal(t, 403, code)
}

func TestListTasksPaginationDoesNotLeakOtherUsersOrRepeatBatches(t *testing.T) {
	r, _ := setupFeature(t)
	// 201 rows cross the ORM batch boundary, a real regression for descending
	// ordering combined with GORM's ascending primary-key continuation.
	rows := make([]model.Task, 201)
	for i := range rows {
		rows[i] = model.Task{TaskID: "task_page_" + strconv.Itoa(i), UserId: 11, Platform: "doubao", Status: model.TaskStatusSuccess, Properties: model.Properties{OriginModelName: "seedance-test"}}
	}
	require.NoError(t, model.DB.CreateInBatches(rows, 50).Error)
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "task_foreign", UserId: 22, Platform: "doubao", Status: model.TaskStatusSuccess}).Error)
	code, out := request(t, r, "GET", "/doubao/api/v3/contents/generations/tasks?page=2&page_size=200", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Equal(t, float64(201), out["total"])
	items := out["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, "task_page_200", items[0].(map[string]any)["id"])
	code, out = request(t, r, "GET", "/doubao/api/v3/contents/generations/tasks?filter.task_ids=task_foreign", nil, nil)
	require.Equal(t, 200, code, out)
	assert.Equal(t, float64(0), out["total"])
}

func TestAcceptedCancellationDoesNotRefundCompletedTask(t *testing.T) {
	r, ch := setupFeature(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	ch.BaseURL = &srv.URL
	require.NoError(t, model.DB.Model(ch).Update("base_url", srv.URL).Error)
	old := service.GetTaskAdaptorFunc
	service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor { return &cancelAdaptor{terminal: "succeeded"} }
	t.Cleanup(func() { service.GetTaskAdaptorFunc = old })
	require.NoError(t, model.DB.Create(&model.Task{TaskID: "task_finished", UserId: 11, ChannelId: 31, Platform: "doubao", Status: model.TaskStatusQueued, Quota: 100, Group: "default", Properties: model.Properties{OriginModelName: "seedance-test"}, PrivateData: model.TaskPrivateData{Key: ch.Key, SeedanceAccountHash: AccountHash(ch), UpstreamTaskID: "finished"}}).Error)
	code, out := request(t, r, "DELETE", "/doubao/api/v3/contents/generations/tasks/task_finished", nil, nil)
	assert.Equal(t, 409, code, out)
	var owner model.User
	require.NoError(t, model.DB.First(&owner, 11).Error)
	assert.Equal(t, 1000, owner.Quota)
}

func TestOrdinaryVideoOptionsUseConfiguredModelCapabilities(t *testing.T) {
	_, ch := setupFeature(t)
	newProvider(t, ch, "/api/v3/assets/")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("expected_task_plugin_key", "doubao")
	c.Set("original_model", "seedance-test")
	for _, body := range []map[string]any{{"frames": 97}, {"output_format": "mov"}, {"draft": true}, {"metadata": map[string]any{"content": []any{map[string]any{"type": "draft_task", "draft_task": map[string]any{"id": "task_draft"}}}}}} {
		c.Set("task_request", body)
		err := ValidateSelected(c, ch, ch.Key)
		require.Error(t, err)
		assert.Equal(t, 422, errorStatus(err))
	}
	c.Set("task_request", map[string]any{"duration": 4, "generate_audio": false, "seed": 0})
	require.NoError(t, ValidateSelected(c, ch, ch.Key))
	task := &model.Task{Platform: "doubao"}
	common.SetContextKey(c, constant.ContextKeyChannelKey, ch.Key)
	AttachTaskIdentity(c, task)
	assert.Equal(t, AccountHash(ch), task.PrivateData.SeedanceAccountHash)
	assert.Equal(t, ch.Key, task.PrivateData.Key)
}

// Run only after creating a fixture with an actual released
// source. This test never clears rows, unlike the ordinary isolated fixtures.
func TestReleasedDatabaseUpgrade(t *testing.T) {
	dsn := os.Getenv("JD_RELEASED_UPGRADE_DSN")
	sqlitePath := os.Getenv("JD_RELEASED_UPGRADE_SQLITE")
	if dsn == "" && sqlitePath == "" {
		t.Skip("requires a database created by the released source")
	}
	if dsn != "" {
		require.True(t, (strings.Contains(dsn, "@127.0.0.1:") || strings.Contains(dsn, "@tcp(127.0.0.1:")) && strings.Contains(dsn, "/seedance_test"), "requires disposable loopback fixture")
	}
	if sqlitePath != "" {
		require.True(t, strings.HasPrefix(sqlitePath, "/tmp/jd-seedance-release-baseline-"))
	}
	t.Setenv("SQL_DSN", dsn)
	t.Setenv("LOG_SQL_DSN", "")
	oldDB, oldLog, oldPath, oldMaster := model.DB, model.LOG_DB, common.SQLitePath, common.IsMasterNode
	common.SQLitePath = sqlitePath
	common.IsMasterNode = true
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLog
		common.SQLitePath = oldPath
		common.IsMasterNode = oldMaster
	})
	for range 2 {
		require.NoError(t, model.InitDB())
		require.NoError(t, model.InitLogDB())
		var user model.User
		require.NoError(t, model.DB.First(&user, 9101).Error)
		assert.Equal(t, 12345678, user.Quota)
		assert.Equal(t, 987654, user.UsedQuota)
		task, found, err := model.GetByTaskId(9101, "released-baseline-task")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, 321, task.Quota)
		assert.Equal(t, "fixture-only-key", task.PrivateData.Key)
		assert.Equal(t, "released-upstream", task.PrivateData.UpstreamTaskID)
		assert.Empty(t, task.PrivateData.SeedanceAccountHash)
		assert.True(t, model.DB.Migrator().HasIndex(&model.JDSeedanceResource{}, "uk_jds_request"))
		assert.True(t, model.DB.Migrator().HasIndex(&model.JDSeedanceResource{}, "uk_jds_upstream"))

		if os.Getenv("JD_ASSET_UPGRADE_FIXTURE") == "1" {
			require.True(t, model.DB.Migrator().HasColumn(&model.JDSeedanceResource{}, "canonical_id"))
			var asset model.JDSeedanceResource
			require.NoError(t, model.DB.First(&asset, "id = ?", "released-image").Error)
			assert.Empty(t, asset.CanonicalID)
			assert.False(t, asset.Deleted)
			assert.Equal(t, "Active", asset.Status)
			assert.Equal(t, "released-provider-image", asset.UpstreamID)
			assert.Equal(t, strings.Repeat("a", 64), *asset.UpstreamIdentity)
			assert.Equal(t, "https://example.invalid/image.png", asset.SourceURL)
			if os.Getenv("JD_SHUJUBAO_UPGRADE_FIXTURE") == "1" {
				current, err := common.Marshal(AssetRequest{GroupId: "released-group", AssetType: "Image", URL: "https://example.invalid/image.png"})
				require.NoError(t, err)
				hash := sha256.Sum256(current)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				common.SetContextKey(c, constant.ContextKeyUserId, 9101)
				replay, err := replayCreation(c, "asset", "released-key", hex.EncodeToString(hash[:]))
				require.NoError(t, err, "released idempotency hashes must remain valid")
				assert.Equal(t, "released-image", replay.(map[string]any)["Id"])
			}
			alias := model.JDSeedanceResource{ID: "upgrade-replay", UserID: 9101, Kind: "asset", RequestKey: "upgrade-key", RequestHash: "upgrade-hash", Binding: "primary", AccountHash: asset.AccountHash, CanonicalID: asset.ID, Deleted: true, Status: "Reused"}
			require.NoError(t, model.DB.Where("id = ?", alias.ID).FirstOrCreate(&alias).Error)
			collision := asset
			collision.ID = "upstream-collision"
			collision.RequestKey = "new-key"
			assert.Error(t, model.DB.Create(&collision).Error)
			collision.ID = "key-collision"
			collision.UpstreamIdentity = nil
			collision.RequestKey = asset.RequestKey
			assert.Error(t, model.DB.Create(&collision).Error)
		}
		assert.True(t, model.DB.Migrator().HasTable(&model.JDSeedanceUpload{}))
		assert.True(t, model.DB.Migrator().HasTable(&model.JDSeedanceTaskDeletion{}))
		assert.True(t, model.DB.Migrator().HasTable(&model.JDSeedanceVerification{}))
		assert.True(t, model.DB.Migrator().HasColumn(&model.JDSeedanceResource{}, "group_type"))
		sqlDB, err := model.DB.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	}
}

// Reproduce the production burst: multiple creates and status polls share a
// group. Each logical create must reach the provider exactly once despite
// simultaneous retries with the same key.
func TestMaterialConcurrentBatchAndReplay(t *testing.T) {
	r, ch := setupFeature(t)
	provider := newProvider(t, ch, "/api/v3/assets/")
	code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "batch"}, "batch-group", false)
	gid := resultID(t, code, out)
	code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/poll.png"}, "poll", false)
	aid := resultID(t, code, out)
	provider.mu.Lock()
	provider.delay = 40 * time.Millisecond
	provider.mu.Unlock()
	// An earlier request owns the group when the burst arrives.
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("id = ?", gid).Update("busy_until", time.Now().Unix()+120).Error)
	var wg sync.WaitGroup
	ids := make([]string, 12)
	for i := range 12 {
		wg.Go(func() {
			key := "batch-" + strconv.Itoa(i/2)
			code, out := action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/" + key + ".png"}, key, false)
			ids[i] = resultID(t, code, out)
		})
	}
	for range 6 {
		wg.Go(func() {
			code, out := action(t, r, "GetAsset", map[string]any{"Id": aid}, "", false)
			assert.Equal(t, 200, code, out)
		})
	}
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("id = ?", gid).Update("busy_until", 0).Error)
	wg.Wait()
	for i := range 6 {
		assert.NotEmpty(t, ids[2*i])
		assert.Equal(t, ids[2*i], ids[2*i+1])
	}
	provider.mu.Lock()
	assert.Equal(t, 7, provider.calls["CreateAsset"])
	assert.Equal(t, 6, provider.calls["GetAsset"])
	provider.mu.Unlock()
	code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/changed.png"}, "batch-0", false)
	require.Equal(t, 409, code, out)
	assert.Equal(t, "idempotency_conflict", out["error"].(map[string]any)["code"])
	var group model.JDSeedanceResource
	require.NoError(t, model.DB.First(&group, "id = ?", gid).Error)
	assert.Zero(t, group.BusyUntil)
	// Unconfirmed upstream failures release the group but never resubmit a
	// creation whose outcome is unknown.
	provider.mu.Lock()
	provider.html = true
	provider.mu.Unlock()
	failedBody := map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/uncertain.png"}
	code, out = action(t, r, "CreateAsset", failedBody, "uncertain", false)
	require.Equal(t, 502, code, out)
	provider.mu.Lock()
	provider.html = false
	provider.mu.Unlock()
	code, out = action(t, r, "CreateAsset", failedBody, "uncertain", false)
	require.Equal(t, 409, code, out)
	assert.Equal(t, "creation_unconfirmed", out["error"].(map[string]any)["code"])
	code, out = action(t, r, "GetAssetGroup", map[string]any{"Id": gid}, "", false)
	require.Equal(t, 200, code, out)
	provider.mu.Lock()
	assert.Equal(t, 7, provider.calls["CreateAsset"])
	provider.mu.Unlock()
}

func TestMaterialContentionDeadlineDeletionAndRecovery(t *testing.T) {
	r, ch := setupFeature(t)
	provider := newProvider(t, ch, "/api/v3/assets/")
	code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "locked"}, "locked", false)
	gid := resultID(t, code, out)
	code, out = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "independent"}, "independent", false)
	otherID := resultID(t, code, out)
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("id = ?", gid).Update("busy_until", time.Now().Unix()+120).Error)
	code, out = action(t, r, "GetAssetGroup", map[string]any{"Id": otherID}, "", false)
	require.Equal(t, 200, code, out)
	code, _ = action(t, r, "GetAssetGroup", map[string]any{"Id": gid}, "", true)
	require.Equal(t, 404, code)
	raw, err := common.Marshal(map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/blocked.png"})
	require.NoError(t, err)
	for _, deadline := range []time.Duration{80 * time.Millisecond, 20 * time.Second} {
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		req := httptest.NewRequest("POST", "/doubao/api/v3/assets/?Action=CreateAsset&Version="+APIVersion, bytes.NewReader(raw)).WithContext(ctx)
		req.Header.Set("Idempotency-Key", "blocked")
		w := httptest.NewRecorder()
		start := time.Now()
		r.ServeHTTP(w, req)
		elapsed := time.Since(start)
		cancel()
		assert.Equal(t, 409, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "resource_busy")
		assert.Equal(t, "2", w.Header().Get("Retry-After"))
		if deadline > time.Second {
			assert.GreaterOrEqual(t, elapsed, 15*time.Second)
			assert.Less(t, elapsed, 18*time.Second)
		} else {
			assert.Less(t, elapsed, time.Second)
		}
	}
	provider.mu.Lock()
	assert.Zero(t, provider.calls["CreateAsset"])
	provider.mu.Unlock()
	var count int64
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("request_key = ?", "blocked").Count(&count).Error)
	assert.Zero(t, count, "timed out requests must not reserve a creation")
	// Expired leases recover without manual cleanup; a retry can use its key.
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("id = ?", gid).Update("busy_until", time.Now().Unix()-1).Error)
	code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/blocked.png"}, "blocked", false)
	_ = resultID(t, code, out)
	// A delete winning while a child waits must yield 404, never a late create.
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("id = ?", gid).Update("busy_until", time.Now().Unix()+120).Error)
	var wg sync.WaitGroup
	wg.Go(func() {
		code, out := action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/deleted.png"}, "after-delete", false)
		assert.Equal(t, 404, code, out)
	})
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, model.DB.Model(&model.JDSeedanceResource{}).Where("id = ?", gid).Updates(map[string]any{"deleted": true, "busy_until": 0}).Error)
	wg.Wait()
	provider.mu.Lock()
	assert.Equal(t, 1, provider.calls["CreateAsset"])
	provider.mu.Unlock()
}

func TestMaterialProviderDeduplicationReplaysCanonicalAsset(t *testing.T) {
	r, ch := setupFeature(t)
	provider := newProvider(t, ch, "/api/v3/assets/")
	code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "dedup"}, "group", false)
	gid := resultID(t, code, out)
	body := map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/image.png", "Name": "original"}
	code, out = action(t, r, "CreateAsset", body, "first", false)
	aid := resultID(t, code, out)
	var canonical model.JDSeedanceResource
	require.NoError(t, model.DB.First(&canonical, "id = ?", aid).Error)
	provider.mu.Lock()
	provider.reuseID = canonical.UpstreamID
	provider.mu.Unlock()
	// Different keys and names, including concurrent attempts, still return the
	// original resource metadata. A caller does not gain a second deletion handle.
	var wg sync.WaitGroup
	for i := range 2 {
		wg.Go(func() {
			key := "duplicate-" + strconv.Itoa(i)
			duplicate := map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/image.png", "Name": key}
			code, out := action(t, r, "CreateAsset", duplicate, key, false)
			assert.Equal(t, aid, resultID(t, code, out))
			assert.Equal(t, "original", out["Result"].(map[string]any)["Name"])
			code, out = action(t, r, "CreateAsset", duplicate, key, false)
			assert.Equal(t, aid, resultID(t, code, out))
		})
	}
	wg.Wait()
	provider.mu.Lock()
	assert.Equal(t, 3, provider.calls["CreateAsset"])
	provider.mu.Unlock()
	code, out = action(t, r, "ListAssets", map[string]any{"GroupId": gid}, "", false)
	require.Equal(t, 200, code, out)
	assert.Equal(t, float64(1), out["Result"].(map[string]any)["TotalCount"])
	var alias model.JDSeedanceResource
	require.NoError(t, model.DB.Where("request_key = ?", "duplicate-0").First(&alias).Error)
	assert.Equal(t, aid, alias.CanonicalID)
	assert.True(t, alias.Deleted)
	assert.Nil(t, alias.UpstreamIdentity)
	assert.Empty(t, alias.UpstreamID)
	code, out = action(t, r, "GetAsset", map[string]any{"Id": alias.ID}, "", false)
	assert.Equal(t, 404, code, out)
	code, out = action(t, r, "DeleteAsset", map[string]any{"Id": alias.ID}, "", false)
	assert.Equal(t, 404, code, out)
	code, out = action(t, r, "CreateAsset", body, "duplicate-0", false)
	require.Equal(t, 409, code, out)
	assert.Equal(t, "idempotency_conflict", out["error"].(map[string]any)["code"])
	code, out = action(t, r, "DeleteAsset", map[string]any{"Id": aid}, "", false)
	require.Equal(t, 200, code, out)
	duplicate := map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/image.png", "Name": "duplicate-0"}
	code, out = action(t, r, "CreateAsset", duplicate, "duplicate-0", false)
	require.Equal(t, 409, code, out)
	assert.Equal(t, "creation_unconfirmed", out["error"].(map[string]any)["code"])
	provider.mu.Lock()
	assert.Equal(t, 1, provider.calls["DeleteAsset"])
	assert.Equal(t, 3, provider.calls["CreateAsset"])
	provider.mu.Unlock()
}

func TestMaterialProviderDeduplicationRejectsUnsafeReuse(t *testing.T) {
	for _, scenario := range []string{"other-user", "other-group", "other-type", "deleted"} {
		t.Run(scenario, func(t *testing.T) {
			r, ch := setupFeature(t)
			provider := newProvider(t, ch, "/api/v3/assets/")
			code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "original"}, "original-group", false)
			gid := resultID(t, code, out)
			code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": "Image", "URL": "https://93.184.215.14/original.png"}, "original", false)
			aid := resultID(t, code, out)
			var canonical model.JDSeedanceResource
			require.NoError(t, model.DB.First(&canonical, "id = ?", aid).Error)
			provider.mu.Lock()
			provider.reuseID = canonical.UpstreamID
			provider.mu.Unlock()
			other := scenario == "other-user"
			if other || scenario == "other-group" {
				code, out = action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "other"}, "other-group", other)
				gid = resultID(t, code, out)
			}
			kind := "Image"
			if scenario == "other-type" {
				kind = "Video"
			}
			if scenario == "deleted" {
				code, out = action(t, r, "DeleteAsset", map[string]any{"Id": aid}, "", false)
				require.Equal(t, 200, code, out)
			}
			body := map[string]any{"GroupId": gid, "AssetType": kind, "URL": "https://93.184.215.14/duplicate.png"}
			for range 2 {
				code, out = action(t, r, "CreateAsset", body, "duplicate", other)
				require.Equal(t, 409, code, out)
				assert.Equal(t, "upstream_resource_conflict", out["error"].(map[string]any)["code"])
				raw, err := common.Marshal(out)
				require.NoError(t, err)
				assert.NotContains(t, string(raw), aid)
				assert.NotContains(t, string(raw), canonical.UpstreamID)
			}
			provider.mu.Lock()
			assert.Equal(t, 2, provider.calls["CreateAsset"])
			deletes := provider.calls["DeleteAsset"]
			provider.mu.Unlock()
			if scenario == "deleted" {
				assert.Equal(t, 1, deletes)
			} else {
				assert.Zero(t, deletes)
			}
			var reservation model.JDSeedanceResource
			require.NoError(t, model.DB.Where("request_key = ?", "duplicate").First(&reservation).Error)
			assert.True(t, reservation.Deleted)
			assert.Empty(t, reservation.CanonicalID)
			assert.Nil(t, reservation.UpstreamIdentity)
		})
	}
}
