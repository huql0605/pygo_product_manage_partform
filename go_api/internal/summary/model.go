package summary

import (
	"time"

	"github.com/shopspring/decimal"
)

// SalesSummary 是汇总层（DWS）的预聚合表，对应表 dws_sales。
//
// 唯一粒度（grain）为 (product_id, productor, name, month)；
// 刷新方式是按 grain 先删后插（见 Repository.RefreshGrainsTx），
// 因此没有 CreatedAt，只有 UpdatedAt。
type SalesSummary struct {
	// ID 自增主键。
	ID uint `json:"id" gorm:"primarykey"`
	// ProductID 品类 ID，粒度第 1 列。
	ProductID uint `json:"product_id" gorm:"not null;uniqueIndex:uk_dws_grain,priority:1"`
	// Productor 品牌或厂商，粒度第 2 列。
	Productor string `json:"productor" gorm:"not null;index:idx_dws_brand;uniqueIndex:uk_dws_grain,priority:2"`
	// Name 商品型号名，粒度第 3 列。
	Name string `json:"name" gorm:"not null;uniqueIndex:uk_dws_grain,priority:3"`
	// Month 统计月份 YYYY-MM，粒度第 4 列。
	Month string `json:"month" gorm:"type:varchar(7);not null;index:idx_dws_month;uniqueIndex:uk_dws_grain,priority:4"`

	// TotalSales 销量合计，SUM(sales)。
	TotalSales int64 `json:"total_sales" gorm:"not null;default:0"`
	// TotalAmount 销售额合计，SUM(price * sales)。
	TotalAmount decimal.Decimal `json:"total_amount" gorm:"type:numeric(18,4);not null;default:0"`
	// PriceSum 单价合计，SUM(price)；与 TotalSales 搭配可算成交均价。
	PriceSum decimal.Decimal `json:"price_sum" gorm:"type:numeric(18,4);not null;default:0"`
	// RecordCount 参与聚合的明细行数，COUNT(*)。
	RecordCount int `json:"record_count" gorm:"not null;default:0"`

	// MinPrice 该粒度内的最低单价。
	MinPrice decimal.Decimal `json:"min_price" gorm:"type:numeric(18,4)"`
	// MaxPrice 该粒度内的最高单价。
	MaxPrice decimal.Decimal `json:"max_price" gorm:"type:numeric(18,4)"`

	// BatchCount 贡献该粒度的不同批次数，COUNT(DISTINCT batch_id)；
	// batch_id 为 NULL 的历史行不计入。
	BatchCount int `json:"batch_count" gorm:"not null;default:0"`

	// UpdatedAt 最近一次重算时间，插入时由刷新 SQL 写成 now()。
	UpdatedAt time.Time `json:"updated_at"`
}

func (SalesSummary) TableName() string {
	return "dws_sales"
}
