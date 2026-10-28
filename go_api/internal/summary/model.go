package summary

import (
	"time"

	"github.com/shopspring/decimal"
)

type SalesSummary struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	ProductID uint   `json:"product_id" gorm:"not null;uniqueIndex:uk_dws_grain,priority:1"`
	Productor string `json:"productor" gorm:"not null;index:idx_dws_brand;uniqueIndex:uk_dws_grain,priority:2"`
	Name      string `json:"name" gorm:"not null;uniqueIndex:uk_dws_grain,priority:3"`
	Month     string `json:"month" gorm:"type:varchar(7);not null;index:idx_dws_month;uniqueIndex:uk_dws_grain,priority:4"`

	TotalSales  int64           `json:"total_sales" gorm:"not null;default:0"`
	TotalAmount decimal.Decimal `json:"total_amount" gorm:"type:numeric(18,4);not null;default:0"`
	PriceSum    decimal.Decimal `json:"price_sum" gorm:"type:numeric(18,4);not null;default:0"`
	RecordCount int             `json:"record_count" gorm:"not null;default:0"`

	MinPrice decimal.Decimal `json:"min_price" gorm:"type:numeric(18,4)"`
	MaxPrice decimal.Decimal `json:"max_price" gorm:"type:numeric(18,4)"`

	BatchCount int `json:"batch_count" gorm:"not null;default:0"`

	UpdatedAt time.Time `json:"updated_at"`
}

func (SalesSummary) TableName() string {
	return "dws_sales"
}
