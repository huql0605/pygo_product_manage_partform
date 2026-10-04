package product

import "errors"

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List() ([]Product, error) {
	return s.repo.List()
}

func (s *Service) Create(p *Product) error {
	if p.Name == "" {
		return errors.New("name require")
	}
	return s.repo.Create(p)
}
