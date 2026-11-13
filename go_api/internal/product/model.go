package product

import "time"

// Product 是品类维度表，对应表 products。
// 这里的「商品」是粗粒度品类（phone、laptop），不是具体型号；
// 具体型号由 sales 中的 productor + name 表达。
type Product struct {
	// ID 自增主键。
	ID uint `json:"id" gorm:"primaryKey"`
	// Name 品类名，全局唯一；导入接口里的 category 取的就是这个值。
	Name string `json:"name" gorm:"not null;uniqueIndex"`
	// Description 品类说明，可空。
	Description string `json:"description"`
	// Price 品类默认价格，仅供展示；统计金额以 sales.price 为准。
	Price float64 `json:"price" gorm:"not null;default:0"`
	// CreatedAt 创建时间，由 GORM 自动维护。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间，由 GORM 自动维护。
	UpdatedAt time.Time `json:"updated_at"`
}
