package summary

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type RefreshResult struct {
	Rows int64 `json:"rows"`
}

func (s *Service) Refresh() (*RefreshResult, error) {
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

// Verif :检查汇总层与明细层是否一致

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
