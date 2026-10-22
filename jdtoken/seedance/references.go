package seedance

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type referencePin struct {
	ChannelID   int
	AccountHash string
	Model       string
}

const referencePinKey = "jdtoken_seedance_reference_pin"

// PrepareReferences runs after TokenAuth and before either native or OpenAI
// plugin decoding. It rewrites owned platform references and pins the account.
func PrepareReferences() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "POST" || !(c.Request.URL.Path == "/doubao/api/v3/contents/generations/tasks" || c.Request.URL.Path == "/v1/video/generations" || c.Request.URL.Path == "/v1/videos") {
			c.Next()
			return
		}
		cfg, err := LoadConfig()
		if err != nil {
			writeError(c, apiError(503, "configuration_error", "Seedance configuration is invalid"))
			return
		}
		if !cfg.Enabled {
			c.Next()
			return
		}
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			writeError(c, apiError(400, "invalid_request", "Cannot read request body"))
			return
		}
		raw, err := storage.Bytes()
		if err != nil {
			writeError(c, err)
			return
		}
		var body map[string]any
		converted := false
		if c.ContentType() == "multipart/form-data" {
			_, params, e := mime.ParseMediaType(c.GetHeader("Content-Type"))
			if e != nil {
				writeError(c, apiError(400, "invalid_request", "Invalid multipart request"))
				return
			}
			form, e := multipart.NewReader(bytes.NewReader(raw), params["boundary"]).ReadForm(8 << 20)
			if e != nil {
				writeError(c, apiError(400, "invalid_request", "Invalid multipart request"))
				return
			}
			defer form.RemoveAll()
			names := form.Value["model"]
			configured := false
			if len(names) == 1 {
				for _, binding := range cfg.Bindings {
					if _, ok := binding.Models[names[0]]; ok {
						configured = true
						break
					}
				}
			}
			hasReference := false
			for _, values := range form.Value {
				for _, value := range values {
					var decoded any
					if common.UnmarshalJsonStr(value, &decoded) == nil {
						if hasMaterialReference(decoded) {
							hasReference = true
						}
					} else if hasMaterialReference(value) {
						hasReference = true
					}
				}
			}
			if !configured && !hasReference {
				c.Next()
				return
			}
			if len(form.File) > 0 {
				writeError(c, apiError(400, "invalid_request", "Upload files first and reference their upload IDs"))
				return
			}
			body = map[string]any{}
			for key, values := range form.Value {
				if len(values) != 1 {
					writeError(c, apiError(400, "invalid_request", "Duplicate multipart field"))
					return
				}
				body[key] = values[0]
			}
			if metadata, ok := body["metadata"].(string); ok {
				var decoded map[string]any
				if common.UnmarshalJsonStr(metadata, &decoded) != nil || decoded == nil {
					writeError(c, apiError(400, "invalid_request", "Invalid metadata JSON"))
					return
				}
				body["metadata"] = decoded
			}
			converted = true
		} else if c.ContentType() == "application/json" {
			if err = common.Unmarshal(raw, &body); err != nil {
				c.Next()
				return
			}
		} else {
			c.Next()
			return
		}

		name, _ := body["model"].(string)
		selected, _ := body["seedance_service"].(string)
		binding := selected
		var ch *model.Channel
		var b Binding
		references := 0
		changed := converted
		// Walk only structured media inputs, not arbitrary text prompt strings.
		var walk func(any, string) (any, error)
		walk = func(value any, field string) (any, error) {
			switch v := value.(type) {
			case map[string]any:
				for k, x := range v {
					childField := k
					if k == "url" {
						childField = field
					}
					out, e := walk(x, childField)
					if e != nil {
						return nil, e
					}
					v[k] = out
				}
			case []any:
				for i, x := range v {
					out, e := walk(x, field)
					if e != nil {
						return nil, e
					}
					v[i] = out
				}
			case string:
				if field == "text" || field == "prompt" {
					return value, nil
				}
				v = strings.TrimSpace(v)
				isAsset := strings.HasPrefix(strings.ToLower(v), "asset://")
				isUpload := strings.HasPrefix(strings.ToLower(v), "upload://")
				if !isAsset && !isUpload {
					return value, nil
				}
				references++
				if references > 16 {
					return nil, apiError(400, "too_many_references", "At most 16 material references are allowed")
				}
				refBinding := ""
				account := ""
				kind := ""
				id := v[8:]
				var asset *model.JDSeedanceResource
				if isAsset {
					if !strings.HasPrefix(id, "jda_") {
						return nil, apiError(400, "unmanaged_asset", "Use an owned JDOTOKEN material identifier")
					}
					asset, err = ownedResource(c, id, "asset")
					if err != nil {
						return nil, err
					}
					refBinding = asset.Binding
					account = asset.AccountHash
					kind = asset.AssetType
				} else {
					id = v[9:]
					var upload model.JDSeedanceUpload
					if err = model.DB.Where("id = ? AND user_id = ? AND state = ? AND expires_at > ?", id, userID(c), "complete", time.Now().Unix()+60).First(&upload).Error; err != nil {
						return nil, err
					}
					refBinding = upload.Binding
					account = upload.AccountHash
					kind = "Image"
					if strings.HasPrefix(upload.ContentType, "video/") {
						kind = "Video"
					}
				}
				expectedType := map[string]string{"image": "Image", "image_url": "Image", "images": "Image", "input_reference": "Image", "video": "Video", "video_url": "Video", "videos": "Video", "audio": "Audio", "audio_url": "Audio", "audios": "Audio"}[field]
				if expectedType != "" && expectedType != kind {
					return nil, apiError(422, "reference_type_mismatch", "Material type does not match its reference field")
				}
				if binding != "" && binding != refBinding {
					return nil, apiError(409, "reference_account_conflict", "All references must belong to the selected service")
				}
				binding = refBinding
				b, ch, err = ResolveBinding(c, cfg, binding, name)
				if err != nil {
					return nil, err
				}
				if AccountHash(ch) != account {
					return nil, apiError(409, "account_changed", "Material account changed")
				}
				if !containsType(b.Models[name].ReferenceTypes, kind) {
					return nil, apiError(422, "unsupported_reference", "This model does not support this reference type")
				}
				changed = true
				if isUpload {
					return uploadSource(c, id, binding, account)
				}
				if asset.UpstreamID == "" {
					return nil, apiError(409, "asset_not_ready", "Material creation is not confirmed")
				}
				if err = refreshAsset(c, b, ch, asset); err != nil {
					return nil, err
				}
				if asset.Status != "Active" {
					return nil, apiError(409, "asset_not_active", "Material must be Active before generation")
				}
				return "asset://" + asset.UpstreamID, nil
			}
			return value, nil
		}
		if _, err = walk(body, ""); err != nil {
			writeError(c, err)
			return
		}
		if binding != "" {
			b, ch, err = ResolveBinding(c, cfg, binding, name)
			if err != nil {
				writeError(c, err)
				return
			}
			if err = validateModelOptions(body, b.Models[name]); err != nil {
				writeError(c, err)
				return
			}

			constraints := service.GetChannelConstraints(c)
			if pin, present, _ := constraints.ResolvedPin(); present && pin.ChannelId != ch.Id {
				writeError(c, apiError(409, "channel_pin_conflict", "Token and material refer to different services"))
				return
			}
			constraints.AddPin(dto.ChannelPin{ChannelId: ch.Id, Source: dto.PinSourceOriginTask, Rank: dto.PinRankOriginTask, RetryMode: dto.PinRetrySingleAttempt})
			c.Set(referencePinKey, referencePin{ch.Id, AccountHash(ch), name})
			if common.GetContextKeyString(c, constant.ContextKeyUsingGroup) == "auto" {
				for _, g := range service.GetRequestAutoGroups(c, common.GetContextKeyString(c, constant.ContextKeyUserGroup)) {
					if model.IsChannelEnabledForGroupModel(g, name, ch.Id) {
						common.SetContextKey(c, constant.ContextKeyAutoGroup, g)
						break
					}
				}
			}
		}
		if selected != "" {
			delete(body, "seedance_service")
			changed = true
		}
		if changed {
			patched, e := common.Marshal(body)
			if e != nil {
				writeError(c, e)
				return
			}
			replacement, e := common.CreateBodyStorage(patched)
			if e != nil {
				writeError(c, e)
				return
			}
			_ = storage.Close()
			c.Set(common.KeyBodyStorage, replacement)
			c.Set(common.KeyRequestBody, nil)
			c.Request.Body = io.NopCloser(replacement)
			c.Request.ContentLength = int64(len(patched))
			c.Request.Header.Set("Content-Type", "application/json")
		}
		c.Next()
	}
}
func mapValue(value any) map[string]any { v, _ := value.(map[string]any); return v }

// ValidateSelected is a final guard against origin-task pins, channel edits,
// credential rotation, or retries changing the account after reference rewriting.
func ValidateSelected(c *gin.Context, ch *model.Channel, key string) error {
	value, ok := c.Get(referencePinKey)
	if !ok {
		cfg, err := LoadConfig()
		if err != nil || !cfg.Enabled {
			return nil
		}
		for _, b := range cfg.Bindings {
			if b.ChannelID != ch.Id || ch.ChannelInfo.IsMultiKey || c.GetString("expected_task_plugin_key") != "doubao" {
				continue
			}
			name := c.GetString("original_model")
			if caps, configured := b.Models[name]; configured {
				if !modelMappingMatches(ch, name, caps) {
					return apiError(409, "model_mapping_changed", "Model mapping no longer matches the configured capabilities")
				}
				value, _ := c.Get("task_request")
				if body, ok := value.(map[string]any); ok {
					if err := validateModelOptions(body, caps); err != nil {
						return err
					}
				}
			}
			c.Set("jdtoken_seedance_account_hash", AccountHash(ch))
			return nil
		}
		return nil
	}
	pin, ok := value.(referencePin)
	if !ok {
		return apiError(409, "invalid_reference_pin", "Invalid material route")
	}
	if origins, ok := common.GetContextKeyType[[]*model.Task](c, constant.ContextKeyOriginTasks); ok {
		for _, task := range origins {
			if task == nil || task.ChannelId != ch.Id || task.PrivateData.SeedanceAccountHash != pin.AccountHash {
				return apiError(409, "reference_account_conflict", "Selected account does not own these materials")
			}
		}
	}
	if ch.Id != pin.ChannelID || AccountHash(ch) != pin.AccountHash || strings.TrimSpace(key) != strings.TrimSpace(ch.Key) || !Eligible(c, ch, pin.Model) {
		return apiError(409, "reference_account_conflict", "Selected account does not own these materials")
	}
	c.Set("jdtoken_seedance_account_hash", AccountHash(ch))
	return nil
}

// AttachTaskIdentity is called once before the native task is persisted. The
// account fingerprint is private JSON, so no upstream task schema changes.
func AttachTaskIdentity(c *gin.Context, task *model.Task) {
	if task.Platform != "doubao" {
		return
	}
	if hash := c.GetString("jdtoken_seedance_account_hash"); hash != "" {
		task.PrivateData.SeedanceAccountHash = hash
		task.PrivateData.Key = common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	}
}

func hasMaterialReference(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for k, x := range v {
			if k != "prompt" && k != "text" && hasMaterialReference(x) {
				return true
			}
		}
	case []any:
		for _, x := range v {
			if hasMaterialReference(x) {
				return true
			}
		}
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		return strings.HasPrefix(s, "asset://") || strings.HasPrefix(s, "upload://")
	}
	return false
}

func validateModelOptions(body map[string]any, caps ModelCapabilities) error {
	for _, params := range []map[string]any{body, mapValue(body["metadata"])} {
		if _, has := params["frames"]; has && !caps.Frames {
			return apiError(422, "unsupported_frames", "Frames is not verified for this model; use duration")
		}
		if draft, _ := params["draft"].(bool); draft && !caps.Draft {
			return apiError(422, "unsupported_draft", "Draft is not enabled for this model")
		}
		if format, _ := params["output_format"].(string); strings.EqualFold(format, "mov") && !caps.MOV {
			return apiError(422, "unsupported_format", "MOV is not enabled for this model")
		}
		if content, ok := params["content"].([]any); ok {
			for _, item := range content {
				if mapValue(item)["type"] == "draft_task" && !caps.DraftUpgrade {
					return apiError(422, "unsupported_draft_upgrade", "Draft upgrade is not enabled for this model")
				}
			}
		}
	}
	return nil
}
