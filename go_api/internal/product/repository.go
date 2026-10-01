package product

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List() ([]Product, error) {
	var out []Product
	err := r.db.Order("id desc").Find(&out).Error
	return out, err

}

func (r *Repository) Create(p *Product) error {
	return r.db.Create(p).Error
}

func (r *Repository) GetById(id uint) (*Product, error) {
	var p Product
	if err := r.db.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}
