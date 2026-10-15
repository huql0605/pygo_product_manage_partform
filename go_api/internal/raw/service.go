package raw

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

var ErrBatchNotFound = errors.New("批次不存在")

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type CreateResult struct {
	BatchID  uint `json:"batch_id"`
	RowCount int  `json:"row_count"`
}

func (s *Service) CreateBatch(source, category string, rows []map[string]any) (*CreateResult, error) {
	records := make([]RawRecord, 0, len(rows))
	for i, row := range rows {
		b, err := json.Marshal(row)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行序列化失败: %w", i+1, err)
		}
		records = append(records, RawRecord{
			RowIndex: i,
			RowData:  b,
		})
	}

	batch := &ImportBatch{
		Source:   source,
		Category: category,
		Status:   StatusLoaded,
		RowCount: len(records),
	}
	if err := s.repo.CreateBatch(batch, records); err != nil {
		return nil, err
	}
	return &CreateResult{BatchID: batch.ID, RowCount: len(records)}, nil

}

func (s *Service) ListColumns(batchID uint) ([]ColumnInfo, error) {
	if _, err := s.repo.GetBatch(batchID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}
	return s.repo.ListColumns(batchID)
}

// CountRecords 统计某批次已入库的原始行数。
func (s *Service) CountRecords(batchID uint) (int64, error) {
	if _, err := s.repo.GetBatch(batchID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrBatchNotFound
		}
		return 0, err
	}
	return s.repo.CountRecords(batchID)
}
