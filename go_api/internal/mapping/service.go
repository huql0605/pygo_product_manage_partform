package mapping

import (
	"errors"
	"fmt"

	"github.com/huql0605/go_api/internal/raw"
	"github.com/huql0605/go_api/internal/sale"
	"github.com/huql0605/go_api/internal/summary"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type TxRunner interface {
	Do(fu func(tx *gorm.DB) error) error
}

type RawReader interface {
	GetBatchForUpdateTx(tx *gorm.DB, id uint) (*raw.ImportBatch, error)
	ListRecordsTx(tx *gorm.DB, batchId uint) ([]raw.RawRecord, error)
	UpdateStatusTx(tx *gorm.DB, batchID uint, status string) error
}

type SaleWriter interface {
	ReplaceBatchTx(tx *gorm.DB, product string, batchID uint, row []sale.SaleRow) (*sale.CreateBatchResult, error)
}

// RuleStore 映射规则的存取。*Repository 隐式实现。
type RuleStore interface {
	Upsert(m *FieldMapping) error
	ListByCategory(category string) ([]FieldMapping, error)
	ListByCategoryTx(tx *gorm.DB, category string) ([]FieldMapping, error)
}

type SummaryRefresher interface {
	GrainsByBatchTx(tx *gorm.DB, batchID uint) ([]summary.Grain, error)
	EnqueueRefreshTx(tx *gorm.DB, grains []summary.Grain) error
}

type Service struct {
	repo    RuleStore
	raws    RawReader
	sales   SaleWriter
	summary SummaryRefresher
	tx      TxRunner
}

func (s *Service) WithTx(tx TxRunner) *Service {
	s.tx = tx
	return s
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
	SummaryPending   bool     `json:"summary_pending"`
	SummaryError     string   `json:"summary_error,omitempty"`
}

// Transform 把某批次的原始数据翻译成标准层（sales）数据。
// 幂等：同批次重复转换只覆盖，不翻倍（靠 sale.Service.ReplaceBatch）。
// 状态流转：成功 → mapped；转换失败不改状态——本轮未写入任何数据，库内仍是上一版。
func (s *Service) Transform(batchID uint) (*TransformResult, error) {
	if s.tx == nil {
		return nil, fmt.Errorf("mapping: transaction runner未配置")
	}

	var res *TransformResult
	err := s.tx.Do(func(tx *gorm.DB) error {
		var err error
		res, err = s.transformTx(tx, batchID)
		return err
	})
	if err != nil {
		return nil, normalizeTransformError(err)
	}
	return res, nil
}

// transform 只负责核心转换，不碰批次状态。
func (s *Service) transformTx(tx *gorm.DB, batchID uint) (*TransformResult, error) {
	batch, err := s.raws.GetBatchForUpdateTx(tx, batchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, raw.ErrBatchNotFound
		}
		return nil, err
	}
	rules, err := s.ruleMapTx(tx, batch.Category)
	if err != nil {
		return nil, err
	}
	records, err := s.raws.ListRecordsTx(tx, batchID)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("批次 %d 没有原始数据", batchID)
	}

	rows, ignoredSet, err := buildSalesRows(records, rules)
	if err != nil {
		return nil, err
	}

	var oldGrains []summary.Grain
	if s.summary != nil {
		oldGrains, err = s.summary.GrainsByBatchTx(tx, batchID)
		if err != nil {
			return nil, err
		}
	}

	replaceRes, err := s.sales.ReplaceBatchTx(
		tx, batch.Category, batchID, rows,
	)
	if err != nil {
		return nil, err
	}

	var grains []summary.Grain
	if s.summary != nil {
		newGrains, err := s.summary.GrainsByBatchTx(tx, batchID)
		if err != nil {
			return nil, err
		}
		grains = mergeGrains(oldGrains, newGrains)
		if err := s.summary.EnqueueRefreshTx(tx, grains); err != nil {
			return nil, err
		}
	}

	if err := s.raws.UpdateStatusTx(
		tx, batchID, raw.StatusMapped,
	); err != nil {
		return nil, err
	}

	return &TransformResult{
		BatchID:        batchID,
		Product:        batch.Category,
		Inserted:       replaceRes.Inserted,
		IgnoredColumns: sortedKeys(ignoredSet),
		SummaryPending: s.summary != nil,
	}, nil
}

func mergeGrains(a, b []summary.Grain) []summary.Grain {
	seen := make(map[summary.Grain]struct{}, len(a)+len(b))
	out := make([]summary.Grain, 0, len(a)+len(b))
	for _, group := range [][]summary.Grain{a, b} {
		for _, g := range group {
			if _, ok := seen[g]; ok {
				continue
			}
			seen[g] = struct{}{}
			out = append(out, g)
		}
	}
	return out
}

func buildSalesRows(records []raw.RawRecord, rules map[string]string) ([]sale.SaleRow, map[string]struct{}, error) {
	rows := make([]sale.SaleRow, 0, len(records))
	ignoredSet := map[string]struct{}{}

	for _, rec := range records {
		lineNo := rec.RowIndex + 1
		obj, err := decodeRow(rec.RowData)
		if err != nil {
			return nil, nil, fmt.Errorf("第 %d 行 JSON 解析失败: %w", lineNo, err)
		}
		values, ignored, err := resolveRow(obj, rules)
		if err != nil {
			return nil, nil, fmt.Errorf("第 %d 行: %w", lineNo, err)
		}

		for _, col := range ignored {
			ignoredSet[col] = struct{}{}
		}

		attrs := make(map[string]string, len(obj))
		for col, v := range obj {
			if IsStandardField(col) {
				continue
			}
			attrs[col] = valueToString(v)
		}
		row, err := buildRow(values, lineNo)
		if err != nil {
			return nil, nil, err
		}
		row.Attrs = attrs
		rows = append(rows, *row)
	}
	return rows, ignoredSet, nil
}

func (s *Service) ruleMapTx(tx *gorm.DB, category string) (map[string]string, error) {
	items, err := s.repo.ListByCategoryTx(tx, category)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(items))
	for _, m := range items {
		out[m.SourceColumn] = m.TargetField
	}
	return out, nil
}

var ErrBatchBusy = errors.New("批次正在转换，请稍后重试")

func normalizeTransformError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return raw.ErrBatchNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
		return ErrBatchBusy
	}
	return err
}
