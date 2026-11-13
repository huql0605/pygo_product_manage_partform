package sale

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

// Sales 是明细层（DWD）的销售事实表，对应表 sales。
// 一行表示「某品类、某品牌、某型号、某月份」的一条销售记录，
// 由原始层经映射转换写入；batch_id 可回溯到 import_batches。
type Sales struct {
	// ID 自增主键。
	ID uint `json:"id" gorm:"primarykey"`
	// ProductID 品类 ID，指向 products.id（如 phone、laptop），不是具体型号。
	ProductID uint `json:"product_id" gorm:"not null;index"`
	// BatchID 来源批次 ID，指向 import_batches.id。
	// 字段可空以兼容历史数据；当前手工导入也会创建 source=manual 的批次并回填。
	BatchID *uint `json:"batch_id" gorm:"index"`
	// Productor 品牌或厂商，如 Apple。
	Productor string `json:"productor" gorm:"not null;index"`
	// Name 商品型号名；与 Productor 一起标识一款具体商品。
	Name string `json:"name" gorm:"not null;index"`
	// Price 单价，numeric(18,4)；用 decimal 避免浮点误差，禁止改成 float64。
	Price decimal.Decimal `json:"price" gorm:"type:numeric(18,4);not null"`
	// Sales 销量（件），非负整数。
	Sales int64 `json:"sales" gorm:"not null"`
	// Month 统计月份，格式 YYYY-MM，varchar(7)。
	Month string `json:"month" gorm:"type:varchar(7);not null;index"`
	// Attrs 非标准列的键值快照，JSONB，默认 {}。
	// 映射时所有不属于 StandardFields 的源列都会以字符串形式写入这里，原始数据不丢弃。
	// 属性报表直接扫该列，依赖 router.MustMigrate 创建的 GIN 索引 idx_sales_attrs。
	Attrs json.RawMessage `json:"attrs" gorm:"type:jsonb;not null;default:'{}'"`
	// CreatedAt 创建时间，由 GORM 自动维护。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间，由 GORM 自动维护。
	UpdatedAt time.Time `json:"updated_at"`
}
