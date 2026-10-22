package seedance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxUploadSize int64 = 100 << 20

type uploadConfig struct{ Directory, BaseURL, Secret string }

func loadUploadConfig() (uploadConfig, error) {
	cfg := uploadConfig{os.Getenv("JDOTOKEN_SEEDANCE_UPLOAD_DIR"), strings.TrimRight(os.Getenv("JDOTOKEN_SEEDANCE_PUBLIC_URL"), "/"), os.Getenv("JDOTOKEN_SEEDANCE_UPLOAD_SECRET")}
	u, err := url.Parse(cfg.BaseURL)
	if cfg.Directory == "" || !filepath.IsAbs(cfg.Directory) || len(cfg.Secret) < 32 || err != nil || u.Host == "" || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return cfg, errors.New("upload storage is not configured")
	}
	return cfg, nil
}
func uploadSignature(cfg uploadConfig, id, method string, expires int64) string {
	h := hmac.New(sha256.New, []byte(cfg.Secret))
	fmt.Fprintf(h, "%s\n%s\n%d", method, id, expires)
	return hex.EncodeToString(h.Sum(nil))
}
func uploadURL(cfg uploadConfig, row *model.JDSeedanceUpload, method string) string {
	return fmt.Sprintf("%s/v1/seedance/uploads/%s/content?expires=%d&signature=%s", cfg.BaseURL, row.ID, row.ExpiresAt, uploadSignature(cfg, row.ID, method, row.ExpiresAt))
}
func uploadMIMEAllowed(s string) bool {
	switch s {
	case "image/png", "image/jpeg", "image/webp", "video/mp4", "video/quicktime":
		return true
	}
	return false
}

func PresignUpload(c *gin.Context) {
	cfg, ok := configForRequest(c)
	if !ok {
		return
	}
	storage, err := loadUploadConfig()
	if err != nil {
		writeError(c, apiError(503, "upload_disabled", "Upload storage is not configured"))
		return
	}
	var req struct {
		Service     string `json:"service"`
		Model       string `json:"model"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	}
	if err = decode(c, &req); err != nil {
		writeError(c, err)
		return
	}
	b, ch, err := ResolveBinding(c, cfg, req.Service, req.Model)
	if err != nil {
		writeError(c, err)
		return
	}
	if req.Model == "" || !uploadMIMEAllowed(req.ContentType) || req.Size <= 0 || req.Size > maxUploadSize {
		writeError(c, apiError(400, "invalid_upload", "Provide a model, supported image/video content_type, and size from 1 to 104857600 bytes"))
		return
	}
	kind := "Image"
	if strings.HasPrefix(req.ContentType, "video/") {
		kind = "Video"
	}
	if !containsType(b.Models[req.Model].ReferenceTypes, kind) {
		writeError(c, apiError(422, "unsupported_asset_type", "The selected model cannot reference this media type"))
		return
	}
	// A bounded reservation covers the on-disk size before any bytes are accepted.
	// Multi-instance installations must share this directory; see deployment notes.
	if err = os.MkdirAll(storage.Directory, 0700); err != nil {
		writeError(c, err)
		return
	}
	if err = cleanExpiredUploads(storage); err != nil {
		writeError(c, err)
		return
	}
	row := model.JDSeedanceUpload{ID: newID("jdu_"), UserID: userID(c), Binding: b.Name, ChannelID: ch.Id, AccountHash: AccountHash(ch), Model: req.Model, ContentType: req.ContentType, Size: req.Size, State: "pending", ExpiresAt: time.Now().Add(24 * time.Hour).Unix()}
	err = model.WithJDSeedanceUploadReservation(func(tx *gorm.DB) error {
		var active, reserved int64
		if err := tx.Model(&model.JDSeedanceUpload{}).Where("user_id = ? AND expires_at > ?", userID(c), time.Now().Unix()).Count(&active).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.JDSeedanceUpload{}).Select("COALESCE(SUM(size), 0)").Scan(&reserved).Error; err != nil {
			return err
		}
		if active >= 20 || reserved+req.Size > 5<<30 {
			return apiError(429, "upload_capacity", "Upload reservation limit reached; retry after existing uploads expire")
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"id": row.ID, "upload_url": uploadURL(storage, &row, "PUT"), "method": "PUT", "headers": gin.H{"Content-Type": row.ContentType}, "expires_at": row.ExpiresAt, "max_bytes": row.Size, "state": row.State, "complete_url": "/v1/seedance/uploads/" + row.ID + "/complete"})
}
func cleanExpiredUploads(cfg uploadConfig) error {
	var rows []model.JDSeedanceUpload
	if err := model.DB.Where("expires_at < ?", time.Now().Unix()).Limit(200).Find(&rows).Error; err != nil {
		return err
	}
	for _, r := range rows {
		if !validUploadID(r.ID) {
			continue
		}
		for _, suffix := range []string{"", ".part"} {
			if err := os.Remove(filepath.Join(cfg.Directory, r.ID+suffix)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		if err := model.DB.Delete(&r).Error; err != nil {
			return err
		}
	}
	return nil
}
func validUploadID(id string) bool {
	if len(id) != 36 || !strings.HasPrefix(id, "jdu_") {
		return false
	}
	_, err := hex.DecodeString(id[4:])
	return err == nil
}
func signedUpload(c *gin.Context, cfg uploadConfig) (*model.JDSeedanceUpload, error) {
	id := c.Param("upload_id")
	expiry, err := strconv.ParseInt(c.Query("expires"), 10, 64)
	if err != nil || !validUploadID(id) || expiry < time.Now().Unix() {
		return nil, apiError(403, "invalid_upload_signature", "Invalid or expired upload URL")
	}
	method := c.Request.Method
	if method == http.MethodHead {
		method = http.MethodGet
	}
	if !hmac.Equal([]byte(c.Query("signature")), []byte(uploadSignature(cfg, id, method, expiry))) {
		return nil, apiError(403, "invalid_upload_signature", "Invalid or expired upload URL")
	}
	var row model.JDSeedanceUpload
	if err = model.DB.Where("id = ? AND expires_at = ?", id, expiry).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
func UploadContent(c *gin.Context) {
	cfg, err := loadUploadConfig()
	if err != nil {
		writeError(c, apiError(503, "upload_disabled", "Upload storage is not configured"))
		return
	}
	feature, err := LoadConfig()
	if err != nil || !feature.Enabled {
		writeError(c, apiError(503, "feature_disabled", "Uploads are disabled"))
		return
	}
	row, err := signedUpload(c, cfg)
	if err != nil {
		writeError(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Referrer-Policy", "no-referrer")
	if c.Request.Method != http.MethodPut {
		if row.State != "complete" {
			writeError(c, apiError(409, "upload_not_complete", "Complete the upload before downloading"))
			return
		}
		c.Header("Content-Type", row.ContentType)
		c.Header("Content-Disposition", `attachment; filename="material"`)
		c.File(filepath.Join(cfg.Directory, row.ID))
		return
	}
	if c.ContentType() != row.ContentType || (c.Request.ContentLength >= 0 && c.Request.ContentLength != row.Size) {
		writeError(c, apiError(400, "upload_mismatch", "Content type or length does not match the upload reservation"))
		return
	}
	claim := model.DB.Model(row).Where("state = ?", "pending").Update("state", "uploading")
	if claim.Error != nil {
		writeError(c, claim.Error)
		return
	}
	if claim.RowsAffected != 1 {
		writeError(c, apiError(409, "upload_already_used", "Upload URL has already been used"))
		return
	}
	path := filepath.Join(cfg.Directory, row.ID)
	success := false
	defer func() {
		if !success {
			os.Remove(path + ".part")
			os.Remove(path)
			model.DB.Model(row).Update("state", "failed")
		}
	}()
	f, err := os.OpenFile(path+".part", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		writeError(c, err)
		return
	}
	digest := sha256.New()
	count, copyErr := io.Copy(io.MultiWriter(f, digest), io.LimitReader(c.Request.Body, row.Size+1))
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil || count != row.Size {
		writeError(c, apiError(400, "upload_mismatch", "Uploaded bytes do not match the reserved size"))
		return
	}
	if err = verifyMedia(path+".part", row.ContentType); err != nil {
		writeError(c, err)
		return
	}
	if err = os.Rename(path+".part", path); err != nil {
		writeError(c, err)
		return
	}
	row.SHA256 = hex.EncodeToString(digest.Sum(nil))
	if err = model.DB.Model(row).Updates(map[string]any{"state": "uploaded", "sha256": row.SHA256}).Error; err != nil {
		writeError(c, err)
		return
	}
	success = true
	c.JSON(200, gin.H{"id": row.ID, "state": "uploaded", "size": count, "sha256": row.SHA256})
}
func verifyMedia(path, declared string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 512)
	n, err := f.Read(header)
	if err != nil && err != io.EOF {
		return err
	}
	header = header[:n]
	detected := http.DetectContentType(header)
	match := detected == declared
	// QuickTime/ISO BMFF brand headers need explicit recognition; codec and
	// duration approval belongs to the provider's material preprocessing.
	if declared == "video/quicktime" && n >= 12 && string(header[4:8]) == "ftyp" && string(header[8:12]) == "qt  " {
		match = true
	}
	if !match {
		return apiError(400, "invalid_media", "File signature does not match its declared media type")
	}
	return nil
}
func CompleteUpload(c *gin.Context) {
	cfg, ok := configForRequest(c)
	if !ok {
		return
	}
	storage, err := loadUploadConfig()
	if err != nil {
		writeError(c, apiError(503, "upload_disabled", "Upload storage is not configured"))
		return
	}
	var row model.JDSeedanceUpload
	if err = model.DB.Where("id = ? AND user_id = ? AND expires_at > ?", c.Param("upload_id"), userID(c), time.Now().Unix()).First(&row).Error; err != nil {
		writeError(c, err)
		return
	}
	_, ch, err := ResolveBinding(c, cfg, row.Binding, row.Model)
	if err != nil {
		writeError(c, err)
		return
	}
	if ch.Id != row.ChannelID || AccountHash(ch) != row.AccountHash {
		writeError(c, apiError(409, "account_changed", "Upload account changed"))
		return
	}
	if row.State != "uploaded" && row.State != "complete" {
		writeError(c, apiError(409, "upload_not_ready", "Upload has not completed successfully"))
		return
	}
	st, err := os.Stat(filepath.Join(storage.Directory, row.ID))
	if err != nil || st.Size() != row.Size {
		writeError(c, apiError(409, "upload_missing", "Uploaded file is missing or has changed"))
		return
	}
	if err = verifyMedia(filepath.Join(storage.Directory, row.ID), row.ContentType); err != nil {
		writeError(c, err)
		return
	}
	if err = model.DB.Model(&row).Update("state", "complete").Error; err != nil {
		writeError(c, err)
		return
	}
	c.JSON(200, gin.H{"id": row.ID, "state": "complete", "source": "upload://" + row.ID, "size": row.Size, "sha256": row.SHA256, "expires_at": row.ExpiresAt, "material_status": "not_ingested"})
}
func uploadSource(c *gin.Context, id, binding, account string) (string, error) {
	storage, err := loadUploadConfig()
	if err != nil {
		return "", apiError(503, "upload_disabled", "Upload storage is not configured")
	}
	var row model.JDSeedanceUpload
	if err = model.DB.Where("id = ? AND user_id = ? AND state = ? AND expires_at > ?", id, userID(c), "complete", time.Now().Unix()+60).First(&row).Error; err != nil {
		return "", err
	}
	if row.Binding != binding || row.AccountHash != account {
		return "", apiError(409, "upload_account_mismatch", "Upload belongs to a different service account")
	}
	return uploadURL(storage, &row, "GET"), nil
}
func containsType(types []string, t string) bool {
	for _, v := range types {
		if t == v {
			return true
		}
	}
	return false
}
