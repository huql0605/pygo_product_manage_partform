package raw

import (
	"encoding/json"
	"time"
)

const (
	StatusLoaded = "loaded"
	StatusMapped = "mapped"
	StatusFailed = "failed"
)

type ImportBatch struct {
	ID            uint      `json:"id" gorm:"primarykey"`
	Source        string    `json:"source" gorm:"not null"`
	Category      string    `json:"category" gorm:"not null;index"`
	Status        string    `json:"status" gorm:"not null;default:loaded;index"`
	RowCount      int       `json:"row_count" gorm:"not null;default:0"`
	FileHash      string    `json:"file_hash" gorm:"type:char(64);not null;default:'';index"`
	RawFileID     *uint     `json:"raw_file_id" gorm:"uniqueIndex:uk_batch_cat_file,priority:2"`
	ReaderVersion string    `json:"reader_version" gorm:"type:varchar(32);not null;default:''"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RawRecord struct {
	ID        uint            `json:"id" gorm:"primarykey"`
	BatchID   uint            `json:"batch_id" gorm:"not null;index"`
	RowIndex  int             `json:"row_index" gorm:"not null"`
	RowData   json.RawMessage `json:"row_data" gorm:"type:jsonb;not null"`
	CreatedAt time.Time
}

type RawFile struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	SHA256      string    `json:"sha256" gorm:"type:char(64);not null;uniqueindex"`
	Size        int64     `json:"size" gorm:"not null"`
	Filename    string    `json:"filename" gorm:"type:text;not null"`
	ContentType string    `json:"content_type" gorm:"type:text;not null;default:''"`
	StorageKind string    `json:"storage_kind" gorm:"type:varchar(16);not null;default:'db'"`
	ObjectKey   string    `json:"object_key" gorm:"type:text;not null;default:''"`
	Content     []byte    `json:"-" gorm:"type:bytea"`
	CreatedAt   time.Time `json:"created_at"`
}

func (RawFile) TableName() string {
	return "raw_files"
}
