package seedance

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// These sessions establish material ownership, not a JDOTOKEN login. Only a
// token issued to this user by this service can import a verified group. Never
// trust browser callback parameters as proof of successful verification.
func visualValidation(c *gin.Context, cfg Config, action string, req AssetRequest) (any, error) {
	if action == "CreateVisualValidateSession" {
		return createVisualSession(c, cfg, req)
	}
	if req.BytedToken == "" || len(req.BytedToken) > 2048 {
		return nil, apiError(400, "invalid_verification_token", "Provide a valid verification token issued to this user")
	}
	digest := sha256.Sum256([]byte(req.BytedToken))
	var session model.JDSeedanceVerification
	if err := model.DB.Where("user_id = ? AND token_hash = ?", userID(c), hex.EncodeToString(digest[:])).First(&session).Error; err != nil {
		return nil, err
	}
	b, ch, err := verificationChannel(c, cfg, &session)
	if err != nil {
		return nil, err
	}
	if (req.Binding != "" && req.Binding != session.Binding) || (req.Model != "" && req.Model != session.Model) {
		return nil, apiError(409, "reference_account_conflict", "All references must belong to the selected service")
	}
	if session.GroupID != "" {
		group, err := ownedResource(c, session.GroupID, "group")
		if err != nil {
			return nil, err
		}
		return map[string]any{"GroupId": group.ID, "GroupType": group.GroupType, "Service": b.Name}, nil
	}
	out, err := AssetAction(c.Request.Context(), b, ch, action, map[string]any{"BytedToken": req.BytedToken})
	if err != nil {
		return nil, err
	}
	id, _ := out["GroupId"].(string)
	if id == "" {
		return nil, apiError(409, "verification_pending", "Real-person verification is not complete")
	}
	if len(id) > 256 || strings.TrimSpace(id) != id {
		return nil, apiError(502, "invalid_upstream_response", "Invalid upstream response")
	}
	// Claim the same identity namespace as ordinary groups. An upstream group
	// can never be imported into another user's inventory, even on concurrent
	// results or if the provider returns a duplicate group identifier.
	identity := sha256.Sum256([]byte("group\x00" + session.AccountHash + "\x00" + id))
	identityHex := hex.EncodeToString(identity[:])
	group := model.JDSeedanceResource{
		ID: newID("jdg_"), UserID: session.UserID, Kind: "group",
		RequestKey: "verification:" + session.ID, RequestHash: session.RequestHash,
		Binding: session.Binding, ChannelID: session.ChannelID, AccountHash: session.AccountHash,
		UpstreamID: id, UpstreamIdentity: &identityHex, GroupType: "LivenessFace",
		Name: "Verified person", Status: "Active",
	}
	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&group).Error; err != nil {
			return err
		}
		var owned model.JDSeedanceResource
		if err := tx.Where("upstream_identity = ?", identityHex).First(&owned).Error; err != nil {
			return err
		}
		if owned.UserID != session.UserID || owned.Binding != session.Binding || owned.ChannelID != session.ChannelID || owned.AccountHash != session.AccountHash || owned.Deleted || owned.GroupType != "LivenessFace" {
			return apiError(409, "upstream_resource_conflict", "Upstream resource is already bound to another material")
		}
		group = owned
		res := tx.Model(&session).Where("group_id = ? OR group_id = ?", "", group.ID).Update("group_id", group.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			var current model.JDSeedanceVerification
			if err := tx.First(&current, "id = ?", session.ID).Error; err != nil {
				return err
			}
			if current.GroupID != group.ID {
				return apiError(409, "upstream_resource_conflict", "Upstream resource is already bound to another material")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"GroupId": group.ID, "GroupType": group.GroupType, "Service": b.Name}, nil
}

func verificationChannel(c *gin.Context, cfg Config, session *model.JDSeedanceVerification) (Binding, *model.Channel, error) {
	b, ch, err := ResolveBinding(c, cfg, session.Binding, session.Model)
	if err != nil {
		return b, ch, err
	}
	if !b.RealPersonVerification || b.Provider != "shujubao" {
		return b, ch, apiError(422, "verification_disabled", "Real-person verification is not enabled for this service")
	}
	if ch.Id != session.ChannelID || AccountHash(ch) != session.AccountHash {
		return b, ch, apiError(409, "account_changed", "Material account changed")
	}
	if session.ExpiresAt <= time.Now().Unix() {
		return b, ch, apiError(410, "verification_expired", "Verification session expired; create a new session")
	}
	return b, ch, nil
}

func createVisualSession(c *gin.Context, cfg Config, req AssetRequest) (any, error) {
	b, ch, err := ResolveBinding(c, cfg, req.Binding, req.Model)
	if err != nil {
		return nil, err
	}
	if !b.RealPersonVerification || b.Provider != "shujubao" {
		return nil, apiError(422, "verification_disabled", "Real-person verification is not enabled for this service")
	}
	key := c.GetHeader("Idempotency-Key")
	if key == "" || len(key) > 128 {
		return nil, apiError(400, "idempotency_key_required", "Provide an Idempotency-Key of 1 to 128 bytes for creation")
	}
	u, err := url.Parse(req.CallbackURL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || len(req.CallbackURL) > 8192 {
		return nil, apiError(400, "invalid_callback_url", "CallbackURL must be a public HTTPS URL without credentials or a fragment")
	}
	if err = common.ValidateURLWithFetchSetting(req.CallbackURL, true, false, false, false, nil, nil, []string{"443"}, true); err != nil {
		return nil, apiError(400, "invalid_callback_url", "CallbackURL must be a public HTTPS URL without credentials or a fragment")
	}
	if err = service.ValidateSSRFProtectedFetchURL(req.CallbackURL); err != nil {
		return nil, apiError(400, "invalid_callback_url", "CallbackURL must be a public HTTPS URL without credentials or a fragment")
	}
	raw, err := common.Marshal(map[string]string{"Service": req.Binding, "Model": req.Model, "CallbackURL": req.CallbackURL})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	requestHash := hex.EncodeToString(digest[:])
	var session model.JDSeedanceVerification
	err = model.DB.Where("user_id = ? AND request_key = ?", userID(c), key).First(&session).Error
	if err == nil {
		if session.RequestHash != requestHash {
			return nil, apiError(409, "idempotency_conflict", "Idempotency-Key was used with a different request")
		}
		if _, _, err = verificationChannel(c, cfg, &session); err != nil {
			return nil, err
		}
		if session.Response == "" {
			return nil, apiError(409, "creation_unconfirmed", "Resource creation has not been confirmed")
		}
		var result map[string]any
		if err = common.UnmarshalJsonStr(session.Response, &result); err != nil {
			return nil, err
		}
		return result, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	now := time.Now().Unix()
	// Keep idempotency tombstones, but discard expired H5 credentials. Nothing
	// here fetches callback URLs or emits tokens into logs or inventory lists.
	if err = model.DB.Model(&model.JDSeedanceVerification{}).Where("expires_at <= ? AND response <> ?", now, "").Update("response", "").Error; err != nil {
		return nil, err
	}
	session = model.JDSeedanceVerification{ID: newID("jdv_"), UserID: userID(c), RequestKey: key, RequestHash: requestHash, Binding: b.Name, ChannelID: ch.Id, AccountHash: AccountHash(ch), Model: req.Model, ExpiresAt: now + 1800}
	if err = model.DB.Create(&session).Error; err != nil {
		return nil, apiError(409, "creation_conflict", "Creation key already exists or could not be reserved")
	}
	out, err := AssetAction(c.Request.Context(), b, ch, "CreateVisualValidateSession", map[string]any{"CallbackURL": req.CallbackURL})
	if err != nil {
		return nil, err
	}
	token, _ := out["BytedToken"].(string)
	link, _ := out["H5Link"].(string)
	h5, parseErr := url.Parse(link)
	if token == "" || len(token) > 2048 || len(link) > 16384 || parseErr != nil || h5.Scheme != "https" || h5.Host == "" || h5.User != nil {
		return nil, apiError(502, "invalid_upstream_response", "Invalid upstream response")
	}
	digest = sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(digest[:])
	result := map[string]any{"SessionId": session.ID, "BytedToken": token, "H5Link": link, "CallbackURL": req.CallbackURL, "Service": b.Name, "ExpiresAt": session.ExpiresAt}
	raw, err = common.Marshal(result)
	if err != nil {
		return nil, err
	}
	if err = model.DB.Model(&session).Updates(map[string]any{"token_hash": tokenHash, "response": string(raw)}).Error; err != nil {
		// Never return a session that failed the unique ownership claim.
		return nil, apiError(409, "creation_unconfirmed", "Resource creation has not been confirmed")
	}
	return result, nil
}
