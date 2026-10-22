package sale

import (
	"encoding/json"
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
	ProductID uint  `json:"product_id"`
	BatchID   *uint `json:"batch_id"`
	Inserted  int   `json:"inserted"`
}

func (s *Service) buildItems(productID uint, batchID *uint, rows []SaleRow) ([]Sales, error) {
	items := make([]Sales, 0, len(rows))
	for i, row := range rows {
		if !monthRe.MatchString(row.Month) {
			return nil, fmt.Errorf("第 %d 行月份格式错误： %q (需要 YYYY-MM)", i+1, row.Month)
		}
		attrs, err := json.Marshal(row.Attrs)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行附加属性格式错误: %w", i+1, err)
		}
		if len(row.Attrs) == 0 {
			attrs = []byte("{}")
		}
		items = append(items, Sales{
			ProductID: productID,
			BatchID:   batchID,
			Productor: row.Productor,
			Name:      row.Name,
			Price:     row.Price,
			Sales:     row.Sales,
			Month:     row.Month,
			Attrs:     json.RawMessage(attrs),
		})
	}
	return items, nil
}

func (s *Service) CreateBatch(product string, batchID *uint, rows []SaleRow) (*CreateBatchResult, error) {
	productID, err := s.products.FindIDByName(product)
	if err != nil {
		return nil, fmt.Errorf("品类 %q 不存在 , 请先创建商品", product)
	}
	items, err := s.buildItems(productID, batchID, rows)
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateMany(items); err != nil {
		return nil, err
	}
	return &CreateBatchResult{ProductID: productID, BatchID: batchID, Inserted: len(items)}, nil
}

func (s *Service) ReplaceBatch(product string, batchID uint, rows []SaleRow) (*CreateBatchResult, error) {
	productID, err := s.products.FindIDByName(product)
	if err != nil {
		return nil, fmt.Errorf("品类 %q 不存在，请先创建商品", product)
	}
	items, err := s.buildItems(productID, &batchID, rows)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceBatch(batchID, items); err != nil {
		return nil, err
	}
	return &CreateBatchResult{ProductID: productID, BatchID: &batchID, Inserted: len(items)}, nil
}
