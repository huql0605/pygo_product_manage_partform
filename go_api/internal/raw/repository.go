package raw

import "gorm.io/gorm"

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateBatch(batch *ImportBatch, records []RawRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		for i := range records {
			records[i].BatchID = batch.ID
		}
		return tx.CreateInBatches(records, 500).Error
	})

}
