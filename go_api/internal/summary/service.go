package summary

import "gorm.io/gorm"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type RefreshResult struct {
	Rows int64 `json:"rows"`
}

// Refresh 按批次当前明细重算。删除批次请先取 grain，再调用 RefreshGrains。
func (s *Service) Refresh(batchID uint) (*RefreshResult, error) {
	n, err := s.repo.Refresh(batchID)
	if err != nil {
		return nil, err
	}
	return &RefreshResult{Rows: n}, nil
}

// RefreshGrains 重算调用方指定的 grain，供删除批次和 transform 变更 grain 时使用。
func (s *Service) RefreshGrains(grains []Grain) (*RefreshResult, error) {
	n, err := s.repo.RefreshGrains(grains)
	if err != nil {
		return nil, err
	}
	return &RefreshResult{Rows: n}, nil
}

// GrainsByBatchTx 在调用方事务内读取批次当前覆盖的 grain。
func (s *Service) GrainsByBatchTx(tx *gorm.DB, batchID uint) ([]Grain, error) {
	return s.repo.GrainsByBatchTx(tx, batchID)
}

func (s *Service) RefreshAll() (*RefreshResult, error) {
	n, err := s.repo.RefreshAll()
	if err != nil {
		return nil, err
	}
	return &RefreshResult{Rows: n}, nil
}

type VerifyResult struct {
	OK          bool   `json:"ok"`
	DiffCount   int    `json:"diff_count"`
	Differences []Diff `json:"differences"`
}

// Verify 检查汇总层与明细层是否一致。
func (s *Service) Verify() (*VerifyResult, error) {
	diffs, err := s.repo.Verify()
	if err != nil {
		return nil, err
	}
	if diffs == nil {
		diffs = []Diff{}
	}
	return &VerifyResult{
		OK:          len(diffs) == 0,
		DiffCount:   len(diffs),
		Differences: diffs,
	}, nil
}
