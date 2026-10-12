package raw

import (
	"encoding/json"
	"fmt"
)

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
