package sale

import (
	"encoding/json"
	"time"
)

type Sales struct {
	ID        uint            `json:"id" gorm:"primarykey"`
	ProductID uint            `json:"product_id" gorm:"not null;index"`
	BatchID   *uint           `json:"batch_id" gorm:"index"` //手工导入时为空
	Productor string          `json:"productor" gorm:"not null;index"`
	Name      string          `json:"name" gorm:"not null;index"`
	Price     float64         `json:"price" gorm:"not null"`
	Sales     int64           `json:"sales" gorm:"not null"`
	Month     string          `json:"month" gorm:"type:varchar(7);not null;index"`
	Attrs     json.RawMessage `json:"attrs" gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
