package sale

import (
	"fmt"
	"regexp"
)

var monthRe = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

type ProductResolver interface {
	FindIDByName(name string) (uint, error)
}

type Service struct {
	repo     *Repository
	products ProductResolver
}

func NewService(repo *Repository, prodcuts ProductResolver) *Service {
	return &Service{repo: repo, products: prodcuts}
}

type CreateBatchResult struct {
	ProductID uint `json:"product_id"`
	Inserted  int  `json:"inserted"`
}

func (s *Service) CreateBatch(product string, rows []SaleRow) (*CreateBatchResult, error) {
	productID, err := s.products.FindIDByName(product)
	if err != nil {
		return nil, fmt.Errorf("品类 %q 不存在 , 请先创建商品", product)
	}
	items := make([]Sales, 0, len(rows))
	for i, row := range rows {
		if !monthRe.MatchString(row.Month) {
			return nil, fmt.Errorf("地 %d 行月份格式错误: %q (需要 YYYY-MM)", i+1, row.Month)
		}
		items = append(items, Sales{
			ProductID: productID,
			Productor: row.Productor,
			Name:      row.Name,
			Price:     row.Price,
			Sales:     row.Sales,
			Month:     row.Month,
		})
	}
	if err := s.repo.CreateMany(items); err != nil {
		return nil, err
	}
	return &CreateBatchResult{ProductID: productID, Inserted: len(items)}, nil
}
