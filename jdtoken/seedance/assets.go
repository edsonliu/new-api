package seedance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AssetRequest struct {
	Id          string   `json:"Id"`
	GroupId     string   `json:"GroupId"`
	GroupIds    []string `json:"GroupIds"`
	Name        *string  `json:"Name"`
	Description *string  `json:"Description"`
	GroupType   string   `json:"GroupType"`
	AssetType   string   `json:"AssetType"`
	URL         string   `json:"URL"`
	UploadID    string   `json:"UploadId"`
	Binding     string   `json:"Service"`
	Model       string   `json:"Model"`
	Page        int      `json:"Page"`
	PageNumber  int      `json:"PageNumber"`
	PageSize    int      `json:"PageSize"`
	NextToken   string   `json:"NextToken"`
	Status      string   `json:"Status"`
}

var actions = map[string]string{"CreateAsset": "asset", "GetAsset": "asset", "UpdateAsset": "asset", "DeleteAsset": "asset", "ListAssets": "asset", "CreateAssetGroup": "group", "GetAssetGroup": "group", "UpdateAssetGroup": "group", "DeleteAssetGroup": "group", "ListAssetGroups": "group"}

func Assets(c *gin.Context) {
	cfg, ok := configForRequest(c)
	if !ok {
		return
	}
	action := c.Query("Action")
	kind, ok := actions[action]
	if !ok {
		writeError(c, apiError(400, "invalid_action", "Unsupported Action"))
		return
	}
	if c.Query("Version") != APIVersion {
		writeError(c, apiError(400, "invalid_version", "Version must be 2024-01-01"))
		return
	}
	var req AssetRequest
	if err := decode(c, &req); err != nil {
		writeError(c, err)
		return
	}
	if (req.Name != nil && utf8.RuneCountInString(*req.Name) > 64) || (req.Description != nil && utf8.RuneCountInString(*req.Description) > 256) {
		writeError(c, apiError(400, "invalid_request", "Name or description is too long"))
		return
	}
	var value any
	var err error
	switch {
	case strings.HasPrefix(action, "List"):
		value, err = listAssets(c, kind, req)
	case strings.HasPrefix(action, "Create"):
		value, err = createAsset(c, cfg, kind, req)
	default:
		value, err = changeAsset(c, cfg, kind, action, req)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	result(c, action, value)
}
func ownedResource(c *gin.Context, id, kind string) (*model.JDSeedanceResource, error) {
	var r model.JDSeedanceResource
	err := model.DB.Where("id = ? AND user_id = ? AND kind = ? AND deleted = ?", id, userID(c), kind, false).First(&r).Error
	return &r, err
}
func resourceChannel(c *gin.Context, cfg Config, r *model.JDSeedanceResource, name string) (Binding, *model.Channel, error) {
	b, ch, err := ResolveBinding(c, cfg, r.Binding, name)
	if err != nil {
		return b, ch, err
	}
	if ch.Id != r.ChannelID || AccountHash(ch) != r.AccountHash {
		return b, nil, apiError(409, "account_changed", "Material account changed; create a new material for the current account")
	}
	return b, ch, nil
}
func resourceView(r *model.JDSeedanceResource) map[string]any {
	v := map[string]any{"Id": r.ID, "Name": r.Name, "Service": r.Binding, "CreateTime": time.Unix(r.CreatedAt, 0).UTC().Format(time.RFC3339), "UpdateTime": time.Unix(r.UpdatedAt, 0).UTC().Format(time.RFC3339)}
	if r.Kind == "group" {
		v["Description"] = r.Description
		v["GroupType"] = "AIGC"
	} else {
		v["GroupId"] = r.GroupID
		v["AssetType"] = r.AssetType
		v["Status"] = r.Status
		v["Reference"] = "asset://" + r.ID
	}
	return v
}
func acquireResource(c *gin.Context, id string) (func(), error) {
	now := time.Now().Unix()
	res := model.DB.Model(&model.JDSeedanceResource{}).Where("id = ? AND user_id = ? AND deleted = ? AND busy_until <= ?", id, userID(c), false, now).Update("busy_until", now+120)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected != 1 {
		return nil, apiError(409, "resource_busy", "Resource is busy; retry later")
	}
	return func() {
		model.DB.Model(&model.JDSeedanceResource{}).Where("id = ? AND busy_until = ?", id, now+120).Update("busy_until", 0)
	}, nil
}
func createAsset(c *gin.Context, cfg Config, kind string, req AssetRequest) (any, error) {
	key := c.GetHeader("Idempotency-Key")
	if key == "" || len(key) > 128 {
		return nil, apiError(400, "idempotency_key_required", "Provide an Idempotency-Key of 1 to 128 bytes for creation")
	}
	requestBytes, err := common.Marshal(req)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(requestBytes)
	requestHash := hex.EncodeToString(digest[:])
	var existing model.JDSeedanceResource
	err = model.DB.Where("user_id = ? AND kind = ? AND request_key = ?", userID(c), kind, key).First(&existing).Error
	if err == nil {
		if existing.RequestHash != requestHash {
			return nil, apiError(409, "idempotency_conflict", "Idempotency-Key was used with a different request")
		}
		if existing.UpstreamID != "" && !existing.Deleted {
			return resourceView(&existing), nil
		}
		return nil, apiError(409, "creation_unconfirmed", "This creation key has already been used; check the original operation")
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	var b Binding
	var ch *model.Channel
	var group *model.JDSeedanceResource
	if kind == "asset" {
		if !slices.Contains([]string{"Image", "Video", "Audio"}, req.AssetType) {
			return nil, apiError(400, "invalid_asset_type", "AssetType must be Image, Video or Audio")
		}
		group, err = ownedResource(c, req.GroupId, "group")
		if err != nil {
			return nil, err
		}
		b, ch, err = resourceChannel(c, cfg, group, req.Model)
	} else {
		if req.Name == nil || strings.TrimSpace(*req.Name) == "" {
			return nil, apiError(400, "name_required", "Name is required")
		}
		if req.GroupType != "" && req.GroupType != "AIGC" {
			return nil, apiError(422, "unsupported_group_type", "Only AIGC groups are supported")
		}
		b, ch, err = ResolveBinding(c, cfg, req.Binding, req.Model)
	}
	if err != nil {
		return nil, err
	}
	if b.Provider == "sdgo" {
		return nil, apiError(422, "unsupported_capability", "This service supports uploads but not independent material management")
	}
	payload := map[string]any{}
	if req.Name != nil {
		payload["Name"] = *req.Name
	}
	if kind == "group" {
		payload["GroupType"] = "AIGC"
		if req.Description != nil {
			payload["Description"] = *req.Description
		}
	}
	if kind == "asset" {
		if !slices.Contains(b.StoreTypes, req.AssetType) {
			return nil, apiError(422, "unsupported_asset_type", "Material type is not enabled for this service")
		}
		if req.UploadID != "" {
			if req.URL != "" {
				return nil, apiError(400, "invalid_request", "Use URL or UploadId, not both")
			}
			req.URL, err = uploadSource(c, req.UploadID, b.Name, AccountHash(ch))
			if err != nil {
				return nil, err
			}
		}
		if len(req.URL) > 8192 {
			return nil, apiError(400, "invalid_url", "URL exceeds 8192 bytes")
		}
		u, e := url.Parse(req.URL)
		if e != nil || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, apiError(400, "invalid_url", "URL must be a public HTTP(S) URL")
		}
		if req.UploadID == "" {
			if err = common.ValidateURLWithFetchSetting(req.URL, true, false, false, false, nil, nil, []string{"80", "443"}, true); err != nil {
				return nil, apiError(400, "invalid_url", "URL must resolve to a public address on port 80 or 443")
			}
			if err = service.ValidateSSRFProtectedFetchURL(req.URL); err != nil {
				return nil, apiError(400, "invalid_url", "URL is not allowed")
			}
		}
		unlock, e := acquireResource(c, group.ID)
		if e != nil {
			return nil, e
		}
		defer unlock()
		// Recheck after acquiring the group lease, including a concurrent deletion.
		group, err = ownedResource(c, group.ID, "group")
		if err != nil {
			return nil, err
		}
		if group.UpstreamID == "" {
			return nil, apiError(409, "group_not_ready", "Group creation is not confirmed")
		}
		payload["GroupId"] = group.UpstreamID
		payload["URL"] = req.URL
		payload["AssetType"] = req.AssetType
	}
	prefix := "jda_"
	if kind == "group" {
		prefix = "jdg_"
	}
	r := model.JDSeedanceResource{ID: newID(prefix), UserID: userID(c), Kind: kind, RequestKey: key, RequestHash: requestHash, Binding: b.Name, ChannelID: ch.Id, AccountHash: AccountHash(ch), GroupID: req.GroupId, AssetType: req.AssetType, Status: "Creating", SourceURL: req.URL}
	if req.Name != nil {
		r.Name = *req.Name
	}
	if req.Description != nil {
		r.Description = *req.Description
	}
	if err = model.DB.Create(&r).Error; err != nil {
		return nil, apiError(409, "creation_conflict", "Creation key already exists or could not be reserved")
	}
	action := "CreateAsset"
	if kind == "group" {
		action = "CreateAssetGroup"
	}
	out, err := AssetAction(c.Request.Context(), b, ch, action, payload)
	if err != nil {
		return nil, err
	}
	id, _ := out["Id"].(string)
	if id == "" || len(id) > 256 {
		return nil, apiError(502, "creation_unconfirmed", "Upstream did not confirm a resource identifier")
	}
	identityHash := sha256.Sum256([]byte(kind + "\x00" + r.AccountHash + "\x00" + id))
	identity := hex.EncodeToString(identityHash[:])
	r.UpstreamIdentity = &identity
	r.UpstreamID = id
	r.Status = "Processing"
	if kind == "group" {
		r.Status = "Active"
	}
	if status, ok := out["Status"].(string); ok {
		r.Status = normalizeAssetStatus(status)
	}
	err = model.DB.Model(&r).Updates(map[string]any{"upstream_id": r.UpstreamID, "upstream_identity": identity, "status": r.Status, "updated_at": time.Now().Unix()}).Error
	if err != nil {
		return nil, err
	}
	return resourceView(&r), nil
}
func normalizeAssetStatus(s string) string {
	switch strings.ToLower(s) {
	case "active":
		return "Active"
	case "failed", "rejected":
		return "Failed"
	default:
		return "Processing"
	}
}
func refreshAsset(c *gin.Context, b Binding, ch *model.Channel, r *model.JDSeedanceResource) error {
	out, err := AssetAction(c.Request.Context(), b, ch, "GetAsset", map[string]any{"Id": r.UpstreamID})
	if err != nil {
		return err
	}
	s, _ := out["Status"].(string)
	r.Status = normalizeAssetStatus(s)
	return model.DB.Model(r).Where("deleted = ?", false).Updates(map[string]any{"status": r.Status, "updated_at": time.Now().Unix()}).Error
}
func changeAsset(c *gin.Context, cfg Config, kind, action string, req AssetRequest) (any, error) {
	id := req.Id
	if id == "" && kind == "group" {
		id = req.GroupId
	}
	r, err := ownedResource(c, id, kind)
	if err != nil {
		return nil, err
	}
	b, ch, err := resourceChannel(c, cfg, r, "")
	if err != nil {
		return nil, err
	}
	if r.UpstreamID == "" {
		return nil, apiError(409, "creation_unconfirmed", "Resource creation has not been confirmed")
	}
	lockID := r.ID
	if kind == "asset" {
		lockID = r.GroupID
	}
	unlock, err := acquireResource(c, lockID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	r, err = ownedResource(c, id, kind)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{"Id": r.UpstreamID}
	if strings.HasPrefix(action, "Update") {
		if req.Name != nil {
			payload["Name"] = *req.Name
		}
		if kind == "group" && req.Description != nil {
			payload["Description"] = *req.Description
		}
	}
	out, err := AssetAction(c.Request.Context(), b, ch, action, payload)
	// Repeated cleanup can complete locally after upstream has already deleted it.
	if err != nil && !(strings.HasPrefix(action, "Delete") && errorStatus(err) == 404) {
		return nil, err
	}
	updates := map[string]any{"updated_at": time.Now().Unix()}
	switch {
	case strings.HasPrefix(action, "Delete"):
		err = model.DB.Transaction(func(tx *gorm.DB) error {
			if kind == "group" {
				if e := tx.Model(&model.JDSeedanceResource{}).Where("group_id = ? AND user_id = ?", r.ID, userID(c)).Updates(map[string]any{"deleted": true, "status": "Deleted"}).Error; e != nil {
					return e
				}
			}
			return tx.Model(r).Updates(map[string]any{"deleted": true, "status": "Deleted"}).Error
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"Id": r.ID, "Deleted": true}, nil
	case strings.HasPrefix(action, "Update"):
		if req.Name != nil {
			r.Name = *req.Name
			updates["name"] = r.Name
		}
		if kind == "group" && req.Description != nil {
			r.Description = *req.Description
			updates["description"] = r.Description
		}
	case kind == "asset":
		s, _ := out["Status"].(string)
		r.Status = normalizeAssetStatus(s)
		updates["status"] = r.Status
	}
	if err = model.DB.Model(r).Updates(updates).Error; err != nil {
		return nil, err
	}
	r.UpdatedAt = time.Now().Unix()
	view := resourceView(r)
	if action == "GetAsset" {
		if raw, ok := out["URL"].(string); ok && len(raw) <= 8192 {
			u, e := url.Parse(raw)
			if e == nil && u.Host != "" && u.User == nil && (u.Scheme == "http" || u.Scheme == "https") {
				view["URL"] = raw
			}
		}
	}
	return view, nil
}
func listAssets(c *gin.Context, kind string, req AssetRequest) (any, error) {
	if req.NextToken != "" {
		page, err := strconv.Atoi(req.NextToken)
		if err != nil {
			return nil, apiError(400, "invalid_pagination", "Invalid NextToken")
		}
		req.PageNumber = page
	}
	page := req.PageNumber
	if page == 0 {
		page = req.Page
	}
	if page == 0 {
		page = 1
	}
	size := req.PageSize
	if size == 0 {
		size = 20
	}
	if page < 1 || page > 100000 || size < 1 || size > 200 || len(req.GroupIds) > 100 {
		return nil, apiError(400, "invalid_pagination", "Page must be 1..100000 and PageSize 1..200")
	}
	q := model.DB.Model(&model.JDSeedanceResource{}).Where("user_id = ? AND kind = ? AND deleted = ?", userID(c), kind, false)
	if req.GroupId != "" {
		req.GroupIds = append(req.GroupIds, req.GroupId)
	}
	if len(req.GroupIds) > 0 {
		q = q.Where("group_id IN ?", req.GroupIds)
	}
	if req.Status != "" {
		q = q.Where("status = ?", req.Status)
	}
	if req.AssetType != "" {
		q = q.Where("asset_type = ?", req.AssetType)
	}
	if req.Binding != "" {
		q = q.Where("binding = ?", req.Binding)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []model.JDSeedanceResource
	if err := q.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, resourceView(&rows[i]))
	}
	next := ""
	if int64(page*size) < total {
		next = fmt.Sprint(page + 1)
	}
	return map[string]any{"Items": items, "TotalCount": total, "PageNumber": page, "PageSize": size, "StatusConsistency": "last_observed", "NextToken": next}, nil
}
