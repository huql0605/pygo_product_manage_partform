package raw

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/huql0605/go_api/internal/summary"
	"gorm.io/gorm"
)

var ErrBatchNotFound = errors.New("批次不存在")

// SummaryRefresher 由 *summary.Service 实现。删除批次后需要重建汇总层。
type SummaryRefresher interface {
	Refresh() (*summary.RefreshResult, error)
}

type Service struct {
	repo    *Repository
	summary SummaryRefresher
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) WithSummary(r SummaryRefresher) *Service {
	s.summary = r
	return s
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

const (
	defaultPageSize = 20
	maxPageSize     = 200
)

// ListBatches 返回批次列表。page 从 1 开始，size 会被夹到 [1, maxPageSize]。
func (s *Service) ListBatches(category, status string, page, size int) (*ListImportsResult, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	items, total, err := s.repo.ListBatches(category, status, (page-1)*size, size)
	if err != nil {
		return nil, err
	}
	return &ListImportsResult{Items: items, Total: total, Page: page, Size: size}, nil
}

// DeleteBatch 删除批次及其明细行与原始数据，随后重建汇总层。
// 删除不可撤销；重跑导入需要重新上传。
func (s *Service) DeleteBatch(batchID uint) (*DeleteImportResult, error) {
	if _, err := s.repo.GetBatch(batchID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBatchNotFound
		}
		return nil, err
	}

	deletedSales, err := s.repo.PurgeBatch(batchID)
	if err != nil {
		return nil, err
	}

	res := &DeleteImportResult{BatchID: batchID, DeletedSales: deletedSales}
	if s.summary != nil {
		if _, err := s.summary.Refresh(); err != nil {
			res.SummaryError = err.Error()
		} else {
			res.SummaryRefreshed = true
		}
	}
	return res, nil
}
