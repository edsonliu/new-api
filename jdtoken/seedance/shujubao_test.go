package seedance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShujubaoVerificationOwnershipAndLifecycle(t *testing.T) {
	r, ch := setupFeature(t)
	cfg, err := LoadConfig()
	require.NoError(t, err)
	cfg.Bindings[0].Provider = "shujubao"
	cfg.Bindings[0].RealPersonVerification = true
	setConfig(t, cfg)
	var creates, queries atomic.Int32
	var verified, invalidResponse atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/api/assets/", r.URL.Path)
		assert.Equal(t, "Bearer test-only-key", r.Header.Get("Authorization"))
		var body map[string]any
		require.NoError(t, common.DecodeJson(r.Body, &body))
		out := map[string]any{}
		switch r.URL.Query().Get("Action") {
		case "CreateVisualValidateSession":
			n := creates.Add(1)
			assert.Equal(t, map[string]any{"CallbackURL": "https://93.184.215.14/callback"}, body)
			if invalidResponse.Load() {
				w.Write([]byte(`<html>not a confirmation</html>`))
				return
			}
			out = map[string]any{"BytedToken": "private-token-" + strconv.Itoa(int(n)), "H5Link": "https://verify.example/session", "CallbackURL": "https://unexpected.example/", "DoNotExpose": "private"}
		case "GetVisualValidateResult":
			queries.Add(1)
			assert.True(t, strings.HasPrefix(body["BytedToken"].(string), "private-token-"))
			if verified.Load() {
				out["GroupId"] = "provider-verified-group"
			}
		case "GetAssetGroup":
			out = map[string]any{"Id": "provider-verified-group", "GroupType": "LivenessFace"}
		case "DeleteAssetGroup":
		default:
			t.Errorf("unexpected action %s", r.URL.Query().Get("Action"))
		}
		raw, err := common.Marshal(map[string]any{"Result": out})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		w.Write(raw)
	}))
	defer srv.Close()
	require.NoError(t, model.DB.Model(ch).Update("base_url", srv.URL).Error)
	create := map[string]any{"Service": "primary", "Model": "seedance-test", "CallbackURL": "https://93.184.215.14/callback"}
	for _, callback := range []string{"", "http://93.184.215.14/cb", "https://127.0.0.1/cb", "https://user:pass@93.184.215.14/cb", "https://93.184.215.14/cb#fragment"} {
		code, out := action(t, r, "CreateVisualValidateSession", map[string]any{"Service": "primary", "CallbackURL": callback}, "invalid", false)
		require.Equal(t, 400, code, out)
	}
	assert.Zero(t, creates.Load())
	code, out := action(t, r, "CreateVisualValidateSession", create, "session-1", false)
	require.Equal(t, 200, code, out)
	session := out["Result"].(map[string]any)
	assert.NotContains(t, session, "DoNotExpose")
	assert.Equal(t, create["CallbackURL"], session["CallbackURL"])
	code, out = action(t, r, "CreateVisualValidateSession", create, "session-1", false)
	require.Equal(t, 200, code, out)
	assert.Equal(t, session, out["Result"])
	assert.Equal(t, int32(1), creates.Load())
	code, _ = action(t, r, "CreateVisualValidateSession", map[string]any{"Service": "primary", "CallbackURL": "https://93.184.215.14/callback"}, "session-1", false)
	assert.Equal(t, 409, code)
	query := map[string]any{"BytedToken": session["BytedToken"]}
	code, _ = action(t, r, "GetVisualValidateResult", query, "", true)
	assert.Equal(t, 404, code)
	assert.Zero(t, queries.Load())
	code, _ = request(t, r, "POST", "/doubao/api/v3/assets/?Action=GetVisualValidateResult&Version="+APIVersion, query, map[string]string{"Test-Restricted": "true"})
	assert.Equal(t, 403, code)
	assert.Zero(t, queries.Load())
	code, out = action(t, r, "GetVisualValidateResult", query, "", false)
	require.Equal(t, 409, code, out)
	assert.Equal(t, "verification_pending", out["error"].(map[string]any)["code"])
	require.NoError(t, model.DB.Model(ch).Update("key", "changed").Error)
	code, _ = action(t, r, "GetVisualValidateResult", query, "", false)
	assert.Equal(t, 409, code)
	require.NoError(t, model.DB.Model(ch).Update("key", "test-only-key").Error)
	verified.Store(true)
	var wg sync.WaitGroup
	results := make(chan map[string]any, 3)
	for range 3 {
		wg.Go(func() {
			status, result := action(t, r, "GetVisualValidateResult", query, "", false)
			assert.Equal(t, 200, status, result)
			results <- result
		})
	}
	wg.Wait()
	close(results)
	out = <-results
	require.Contains(t, out, "Result")
	group := out["Result"].(map[string]any)
	for other := range results {
		assert.Equal(t, group, other["Result"])
	}
	gid := group["GroupId"].(string)
	assert.True(t, strings.HasPrefix(gid, "jdg_"))
	assert.NotEqual(t, "provider-verified-group", gid)
	assert.Equal(t, "LivenessFace", group["GroupType"])
	queryCount := queries.Load()
	code, out = action(t, r, "GetVisualValidateResult", query, "", false)
	require.Equal(t, 200, code, out)
	assert.Equal(t, group, out["Result"])
	assert.Equal(t, queryCount, queries.Load(), "completed result replay must stay local")
	code, out = action(t, r, "GetAssetGroup", map[string]any{"Id": gid}, "", false)
	require.Equal(t, 200, code, out)
	assert.Equal(t, "LivenessFace", out["Result"].(map[string]any)["GroupType"])
	// Another user with their own valid session cannot claim the same upstream group.
	code, out = action(t, r, "CreateVisualValidateSession", create, "other-session", true)
	require.Equal(t, 200, code, out)
	otherQuery := map[string]any{"BytedToken": out["Result"].(map[string]any)["BytedToken"]}
	code, out = action(t, r, "GetVisualValidateResult", otherQuery, "", true)
	require.Equal(t, 409, code, out)
	assert.Equal(t, "upstream_resource_conflict", out["error"].(map[string]any)["code"])
	code, out = action(t, r, "DeleteAssetGroup", map[string]any{"Id": gid}, "", false)
	require.Equal(t, 200, code, out)
	code, _ = action(t, r, "GetVisualValidateResult", query, "", false)
	assert.Equal(t, 404, code, "completed session must not resurrect a deleted group")
	code, out = action(t, r, "GetVisualValidateResult", otherQuery, "", true)
	assert.Equal(t, 409, code, out)
	require.NoError(t, model.DB.Model(&model.JDSeedanceVerification{}).Where("id = ?", session["SessionId"]).Update("expires_at", time.Now().Unix()-1).Error)
	code, _ = action(t, r, "GetVisualValidateResult", query, "", false)
	assert.Equal(t, 410, code)
	invalidResponse.Store(true)
	code, _ = action(t, r, "CreateVisualValidateSession", create, "uncertain", false)
	assert.Equal(t, 502, code)
	count := creates.Load()
	code, _ = action(t, r, "CreateVisualValidateSession", create, "uncertain", false)
	assert.Equal(t, 409, code)
	assert.Equal(t, count, creates.Load(), "uncertain creation must not repeat")
	var expired model.JDSeedanceVerification
	require.NoError(t, model.DB.First(&expired, "id = ?", session["SessionId"]).Error)
	assert.Empty(t, expired.Response)
	cfg.Bindings[0].RealPersonVerification = false
	setConfig(t, cfg)
	code, _ = action(t, r, "CreateVisualValidateSession", create, "disabled", false)
	assert.Equal(t, 422, code)
}

func TestShujubaoErrorsAndAudioReferences(t *testing.T) {
	r, ch := setupFeature(t)
	cfg, err := LoadConfig()
	require.NoError(t, err)
	cfg.Bindings[0].Provider = "shujubao"
	cfg.Bindings[0].StoreTypes = []string{"Image", "Video", "Audio"}
	cfg.Bindings[0].Models["seedance-test"] = ModelCapabilities{ReferenceTypes: []string{"Image", "Video", "Audio"}}
	setConfig(t, cfg)
	newProvider(t, ch, "/v1/api/assets/")
	code, out := action(t, r, "CreateAssetGroup", map[string]any{"Service": "primary", "Name": "media"}, "media", false)
	gid := resultID(t, code, out)
	content := []any{}
	for _, kind := range []string{"Image", "Video", "Audio"} {
		code, out = action(t, r, "CreateAsset", map[string]any{"GroupId": gid, "AssetType": kind, "URL": "https://93.184.215.14/fixture"}, kind, false)
		id := resultID(t, code, out)
		field := strings.ToLower(kind) + "_url"
		content = append(content, map[string]any{"type": field, field: map[string]any{"url": "asset://" + id}, "role": "reference_" + strings.ToLower(kind)})
	}
	code, out = request(t, r, "POST", "/doubao/api/v3/contents/generations/tasks", map[string]any{"model": "seedance-test", "seedance_service": "primary", "content": content}, nil)
	require.Equal(t, 200, code, out)
	assert.NotContains(t, out, "seedance_service")
	for i, v := range out["content"].([]any) {
		item := v.(map[string]any)
		field := content[i].(map[string]any)["type"].(string)
		assert.True(t, strings.HasPrefix(item[field].(map[string]any)["url"].(string), "asset://provider-"))
	}
	for _, tc := range []struct {
		status int
		body   string
		want   int
		code   string
	}{
		{200, `{"code":1,"message":"private upstream detail"}`, 502, "upstream_rejected"},
		{429, `{"code":1,"message":"limited"}`, 429, "upstream_rate_limited"},
		{403, `{"code":1,"message":"real person not enabled"}`, 403, "upstream_capability_unavailable"},
		{400, `{"ResponseMetadata":{"Error":{"Code":"ValidatePending"}}}`, 409, "verification_pending"},
		{400, `{"ResponseMetadata":{"Error":{"Code":"BadRequest","Message":"素材状态正在同步中，请稍后再尝试修改"}}}`, 409, "upstream_resource_busy"},
		{404, `{"ResponseMetadata":{"Error":{"Code":"NotFound.asset_id"}}}`, 404, "not_found"},
		{200, `<html>oops</html>`, 502, "invalid_upstream_response"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer srv.Close()
			ch.BaseURL = &srv.URL
			_, err := AssetAction(context.Background(), cfg.Bindings[0], ch, "GetAsset", map[string]any{"Id": "test"})
			var api *APIError
			require.ErrorAs(t, err, &api)
			assert.Equal(t, tc.want, api.Status)
			assert.Equal(t, tc.code, api.Code)
			assert.NotContains(t, api.Message, "private upstream detail")
		})
	}
}
