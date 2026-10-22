package sale

type SaleRow struct {
	Productor string            `json:"productor" binding:"required,min=1,max=100"`
	Name      string            `json:"name" binding:"required,min=1,max=100"`
	Price     float64           `json:"price" binding:"gte=0"`
	Sales     int64             `json:"sales" binding:"gte=0"`
	Month     string            `json:"month" binding:"required"`
	Attrs     map[string]string `json:"attrs"`
}

type BatchCreateSalesReq struct {
	Product string    `json:"product" binding:"required,min=1"`
	BatchID *uint     `json:"batch_id"`
	Rows    []SaleRow `json:"rows" binding:"required,min=1"`
}
