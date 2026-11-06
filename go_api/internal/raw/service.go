package raw

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/huql0605/go_api/internal/summary"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var ErrBatchNotFound = errors.New("批次不存在")
var ErrBatchBusy = errors.New("批次正在被其他操作占用，请稍后重试")

// SummaryRefresher 由 *summary.Service 实现。删除批次后需要重建汇总层。
type SummaryRefresher interface {
	Refresh() (*summary.RefreshResult, error)
}

type Service struct {
	repo       *Repository
	summary    SummaryRefresher
	signingKey string
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) WithSummary(r SummaryRefresher) *Service {
	s.summary = r
	return s
}

func (s *Service) WithSigningKey(key string) *Service {
	s.signingKey = key
	return s
}

type CreateResult struct {
	BatchID    uint   `json:"batch_id"`
	RowCount   int    `json:"row_count"`
	Duplicated bool   `json:"duplicated"`
	FileHash   string `json:"file_hash,omitempty"`
}

func (s *Service) buildRecords(rows []map[string]any) ([]RawRecord, error) {
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
	return records, nil
}

func (s *Service) CreateBatch(source, category string, rows []map[string]any) (*CreateResult, error) {
	records, err := s.buildRecords(rows)
	if err != nil {
		return nil, err
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
	deletedSales, err := s.repo.PurgeBatch(batchID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBatchNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "55P03" {
			return nil, ErrBatchBusy
		}
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

func (s *Service) CreateBatchFromFile(in UploadImportInput) (*CreateResult, error) {
	if len(in.Content) == 0 {
		return nil, fmt.Errorf("原始文件为空")
	}
	if in.Category == "" {
		return nil, fmt.Errorf("category 不能为空")
	}
	sum := sha256.Sum256(in.Content)
	hash := hex.EncodeToString(sum[:])
	rowsSum := sha256.Sum256([]byte(in.RowsJSON))
	rowsHash := hex.EncodeToString(rowsSum[:])

	if in.ExpectedHash != "" && !strings.EqualFold(in.ExpectedHash, hash) {
		return nil, fmt.Errorf("文件指纹不一致：客户端 %s，服务端 %s", in.ExpectedHash, hash)
	}
	expected := hex.EncodeToString(
		s.hmacSHA256(s.signingKey, hash+"\n"+in.Category+"\n"+in.ReaderVersion+"\n"+rowsHash),
	)
	if !hmac.Equal([]byte(expected), []byte(in.Signature)) {
		return nil, fmt.Errorf("签名不一致")
	}

	records, err := s.buildRecords(in.Rows)
	if err != nil {
		return nil, err
	}
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = in.Filename
	}

	batch := &ImportBatch{
		Source:        source,
		Category:      in.Category,
		Status:        StatusLoaded,
		RowCount:      len(records),
		FileHash:      hash,
		ReaderVersion: in.ReaderVersion,
	}
	file := &RawFile{
		SHA256:      hash,
		Size:        int64(len(in.Content)),
		Filename:    in.Filename,
		ContentType: in.ContentType,
		StorageKind: "db",
		Content:     in.Content,
	}

	duplicated, err := s.repo.CreateBatchWithFile(batch, file, records)
	if err != nil {
		return nil, err
	}

	return &CreateResult{
		BatchID:    batch.ID,
		RowCount:   batch.RowCount,
		Duplicated: duplicated,
		FileHash:   hash,
	}, nil
}

func (s *Service) hmacSHA256(key string, data string) []byte {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
