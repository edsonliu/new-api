package seedance

import (
	"sort"

	"github.com/gin-gonic/gin"
)

func Capabilities(c *gin.Context) {
	cfg, ok := configForRequest(c)
	if !ok {
		return
	}
	_, storageErr := loadUploadConfig()
	entries := []map[string]any{}
	for _, b := range cfg.Bindings {
		names := make([]string, 0, len(b.Models))
		for name := range b.Models {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if _, _, err := ResolveBinding(c, cfg, b.Name, name); err != nil {
				continue
			}
			caps := b.Models[name]
			entries = append(entries, map[string]any{"service": b.Name, "model": name, "material_management": b.Provider != "sdgo", "storage_types": b.StoreTypes, "reference_types": caps.ReferenceTypes, "upload": map[string]any{"enabled": storageErr == nil, "max_bytes": maxUploadSize, "expires_seconds": 86400, "audio": false}, "task_list": true, "task_cancel": caps.Cancel, "task_delete": map[string]any{"local": true, "upstream": false}, "draft": caps.Draft, "draft_upgrade": caps.DraftUpgrade, "mov": caps.MOV, "frames": caps.Frames, "automatic_ingestion": false, "real_person_verification": b.RealPersonVerification, "cross_account_copy": false, "verification": "administrator_configured"})
		}
	}
	c.JSON(200, gin.H{"object": "list", "data": entries, "asset_api_version": APIVersion})
}
