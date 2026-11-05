package sale

import "github.com/shopspring/decimal"

type SaleRow struct {
	Productor string            `json:"productor" binding:"required,min=1,max=100"`
	Name      string            `json:"name" binding:"required,min=1,max=100"`
	Price     decimal.Decimal   `json:"price"`
	Sales     int64             `json:"sales" binding:"gte=0"`
	Month     string            `json:"month" binding:"required"`
	Attrs     map[string]string `json:"attrs"`
}

type BatchCreateSalesReq struct {
	Product string `json:"product" binding:"required,min=1"`

	Rows []SaleRow `json:"rows" binding:"required,min=1"`
}
