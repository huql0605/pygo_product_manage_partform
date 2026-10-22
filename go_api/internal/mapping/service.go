package mapping

import (
	"errors"
	"fmt"

	"github.com/huql0605/go_api/internal/raw"
	"github.com/huql0605/go_api/internal/sale"
	"github.com/huql0605/go_api/internal/summary"
	"gorm.io/gorm"
)

type RawReader interface {
	GetBatch(id uint) (*raw.ImportBatch, error)
	ListRecords(batchId uint) ([]raw.RawRecord, error)
	UpdateStatus(batchID uint, status string) error
}

type SaleWriter interface {
	ReplaceBatch(product string, batchID uint, row []sale.SaleRow) (*sale.CreateBatchResult, error)
}

// RuleStore 映射规则的存取。*Repository 隐式实现。
type RuleStore interface {
	Upsert(m *FieldMapping) error
	ListByCategory(category string) ([]FieldMapping, error)
}

type SummaryRefresher interface {
	Refresh() (*summary.RefreshResult, error)
}

type Service struct {
	repo    RuleStore
	raws    RawReader
	sales   SaleWriter
	summary SummaryRefresher
}

func NewService(repo RuleStore, raws RawReader, sales SaleWriter) *Service {
	return &Service{repo: repo, raws: raws, sales: sales}
}

func (s *Service) WithSummary(r SummaryRefresher) *Service {
	s.summary = r
	return s
}

// Upsert 校验并保存一条映射规则。
func (s *Service) Upsert(req CreateMappingReq) (*FieldMapping, error) {
	if !IsStandardField(req.TargetField) {
		return nil, fmt.Errorf("目标字段 %q 不在标准字段白名单 %v 中", req.TargetField, StandardFields)
	}
	m := &FieldMapping{
		Category:     req.Category,
		SourceColumn: req.SourceColumn,
		TargetField:  req.TargetField,
	}
	if err := s.repo.Upsert(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Service) ListByCategory(category string) ([]FieldMapping, error) {
	if category == "" {
		return nil, fmt.Errorf("category 不能为空")
	}
	return s.repo.ListByCategory(category)
}

type TransformResult struct {
	BatchID          uint     `json:"batch_id"`
	Product          string   `json:"product"`
	Inserted         int      `json:"inserted"`
	IgnoredColumns   []string `json:"ignored_columns"` // ExtraColumns 未映射到标准字段的列，但值已写入 sales.attrs，数据未丢弃。
	SummaryRefreshed bool     `json:"summary_refreshed"`
	SummaryError     string   `json:"summary_error,omitempty"`
}

// Transform 把某批次的原始数据翻译成标准层（sales）数据。
// 幂等：同批次重复转换只覆盖，不翻倍（靠 sale.Service.ReplaceBatch）。
// 状态流转：成功 → mapped；转换失败 → failed（尽力标记，但不覆盖真正的错误）。
func (s *Service) Transform(batchID uint) (*TransformResult, error) {
	res, err := s.transform(batchID)
	if err != nil {
		// 批次不存在就不必标记了；其它失败尽力标 failed
		if !errors.Is(err, raw.ErrBatchNotFound) {
			_ = s.raws.UpdateStatus(batchID, raw.StatusFailed)
		}
		return nil, err
	}
	if err := s.raws.UpdateStatus(batchID, raw.StatusMapped); err != nil {
		return nil, err
	}
	if s.summary != nil {
		if _, err := s.summary.Refresh(); err != nil {
			res.SummaryError = err.Error()
		} else {
			res.SummaryRefreshed = true
		}
	}
	return res, nil
}

// transform 只负责核心转换，不碰批次状态。
func (s *Service) transform(batchID uint) (*TransformResult, error) {
	batch, err := s.raws.GetBatch(batchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, raw.ErrBatchNotFound
		}
		return nil, err
	}
	rules, err := s.ruleMap(batch.Category)
	if err != nil {
		return nil, err
	}
	records, err := s.raws.ListRecords(batchID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("批次 %d 没有原始数据", batchID)
	}

	rows := make([]sale.SaleRow, 0, len(records))
	ignoredSet := map[string]struct{}{}

	for _, rec := range records {
		lineNo := rec.RowIndex + 1
		obj, err := decodeRow(rec.RowData)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行 JSON 解析失败: %w", lineNo, err)
		}
		values, ignored, err := resolveRow(obj, rules)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行: %w", lineNo, err)
		}

		for _, col := range ignored {
			ignoredSet[col] = struct{}{}
		}

		attrs := make(map[string]string, len(ignored))
		for _, col := range ignored {
			attrs[col] = valueToString(obj[col])
		}
		row, err := buildRow(values, lineNo)
		if err != nil {
			return nil, err
		}
		row.Attrs = attrs
		rows = append(rows, *row)
	}

	res, err := s.sales.ReplaceBatch(batch.Category, batchID, rows)
	if err != nil {
		return nil, err
	}

	return &TransformResult{
		BatchID:        batchID,
		Product:        batch.Category,
		Inserted:       res.Inserted,
		IgnoredColumns: sortedKeys(ignoredSet),
	}, nil
}

func (s *Service) ruleMap(category string) (map[string]string, error) {
	items, err := s.repo.ListByCategory(category)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(items))
	for _, m := range items {
		out[m.SourceColumn] = m.TargetField
	}
	return out, nil
}
