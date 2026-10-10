package product

type CreateProductReq struct {
	Name  string  `json:"name" binding:"required,min=1,max=100"`
	Price float64 `json:"price" binding:"gte=0"`
}
