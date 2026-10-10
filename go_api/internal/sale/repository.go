package sale

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateMany(rows []Sales) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.CreateInBatches(rows, 500).Error
	})
}
