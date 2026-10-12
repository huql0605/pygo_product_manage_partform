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
	ID        uint      `json:"id" gorm:"primarykey"`
	Source    string    `json:"source" gorm:"not null"`
	Category  string    `json:"category" gorm:"not null;index"`
	Status    string    `json:"status" gorm:"not null;default:loaded;index"`
	RowCount  int       `json:"row_count" gorm:"not null;default:0"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RawRecord struct {
	ID        uint            `json:"id" gorm:"primarykey"`
	BatchID   uint            `json:"batch_id" gorm:"not null;index"`
	RowIndex  int             `json:"row_index" gorm:"not null"`
	RowData   json.RawMessage `json:"row_data" gorm:"type:jsonb;not null"`
	CreatedAt time.Time
}
