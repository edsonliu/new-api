package model

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// JDSeedanceResource is an owned reference, never a copy of a shared upstream
// account's inventory. AccountHash changes when the endpoint or credential changes.
type JDSeedanceResource struct {
	ID               string  `gorm:"primaryKey;size:64"`
	UserID           int     `gorm:"not null;index:idx_jds_owner_kind;uniqueIndex:uk_jds_request,priority:1"`
	Kind             string  `gorm:"size:16;not null;index:idx_jds_owner_kind;uniqueIndex:uk_jds_request,priority:2"`
	RequestKey       string  `gorm:"size:128;not null;uniqueIndex:uk_jds_request,priority:3"`
	RequestHash      string  `gorm:"size:64;not null;default:''"`
	GroupID          string  `gorm:"size:64;index"`
	GroupType        string  `gorm:"size:32"`
	Binding          string  `gorm:"size:64;not null"`
	ChannelID        int     `gorm:"not null"`
	AccountHash      string  `gorm:"size:64;not null"`
	UpstreamID       string  `gorm:"size:256"`
	UpstreamIdentity *string `gorm:"size:64;uniqueIndex:uk_jds_upstream"`
	// Hidden idempotency records point at the one owned, deletable resource.
	// They never hold an upstream identity or appear in material lists.
	CanonicalID string `gorm:"size:64"`
	AssetType   string `gorm:"size:16"`
	Name        string `gorm:"size:256"`
	Description string `gorm:"size:1024"`
	Status      string `gorm:"size:32"`
	SourceURL   string `gorm:"type:text"`
	Deleted     bool   `gorm:"not null;index"`
	BusyUntil   int64  `gorm:"not null"`
	CreatedAt   int64
	UpdatedAt   int64
}

func (JDSeedanceResource) TableName() string { return "jdtoken_seedance_resources" }

// Verification sessions are private, short-lived provider credentials. Never
// expose this table through inventory lists or dashboard serializers.
type JDSeedanceVerification struct {
	ID          string `gorm:"primaryKey;size:64"`
	UserID      int    `gorm:"not null;uniqueIndex:uk_jdsv_request,priority:1"`
	RequestKey  string `gorm:"size:128;not null;uniqueIndex:uk_jdsv_request,priority:2"`
	RequestHash string `gorm:"size:64;not null"`
	Binding     string `gorm:"size:64;not null"`
	ChannelID   int
	AccountHash string  `gorm:"size:64;not null"`
	Model       string  `gorm:"size:256"`
	TokenHash   *string `gorm:"size:64;uniqueIndex:uk_jdsv_token"`
	Response    string  `gorm:"type:text"`
	GroupID     string  `gorm:"size:64"`
	ExpiresAt   int64   `gorm:"index"`
	CreatedAt   int64
}

func (JDSeedanceVerification) TableName() string { return "jdtoken_seedance_verifications" }

// Uploads contain opaque object keys, never paths supplied by a client.
type JDSeedanceUpload struct {
	ID          string `gorm:"primaryKey;size:64"`
	UserID      int    `gorm:"not null;index"`
	Binding     string `gorm:"size:64;not null"`
	ChannelID   int
	AccountHash string `gorm:"size:64"`
	Model       string `gorm:"size:256"`
	ContentType string `gorm:"size:64"`
	Size        int64
	SHA256      string `gorm:"size:64"`
	State       string `gorm:"size:32;index"`
	ExpiresAt   int64  `gorm:"index"`
	CreatedAt   int64
}

func (JDSeedanceUpload) TableName() string { return "jdtoken_seedance_uploads" }

type JDSeedanceTaskDeletion struct {
	TaskID    int64 `gorm:"primaryKey;autoIncrement:false"`
	UserID    int   `gorm:"not null;index"`
	CreatedAt int64
}

func (JDSeedanceTaskDeletion) TableName() string { return "jdtoken_seedance_task_deletions" }

// HideJDSeedanceTask keeps the task and its billing/audit records. All existing
// native/protocol/artifact readers already honour ResultDiscarded.
func HideJDSeedanceTask(userID int, taskID string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var task Task
		if err := lockForUpdate(tx).Where("user_id = ? AND task_id = ?", userID, taskID).First(&task).Error; err != nil {
			return err
		}
		if task.Platform != "doubao" || (task.Status != TaskStatusSuccess && task.Status != TaskStatusFailure) {
			return errors.New("task is not terminal")
		}
		task.PrivateData.ResultDiscarded = true
		if err := tx.Model(&task).Update("private_data", task.PrivateData).Error; err != nil {
			return err
		}
		return tx.FirstOrCreate(&JDSeedanceTaskDeletion{TaskID: task.ID, UserID: userID, CreatedAt: time.Now().Unix()}, "task_id = ?", task.ID).Error
	})
}

// WithJDSeedanceUploadReservation serializes capacity reservations across app
// instances. The write also acquires SQLite's writer lock before reading totals.
func WithJDSeedanceUploadReservation(fn func(*gorm.DB) error) error {
	lock := JDSeedanceResource{ID: "upload_reservation", UserID: 0, Kind: "internal", RequestKey: "upload_reservation", Binding: "internal", AccountHash: "internal"}
	if err := DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&lock).Error; err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&JDSeedanceResource{}).Where("id = ?", lock.ID).Update("busy_until", time.Now().Unix()).Error; err != nil {
			return err
		}
		return fn(tx)
	})
}
