package uow

import "gorm.io/gorm"

type UnitOfWork struct {
	db *gorm.DB
}

func New(db *gorm.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

func (u *UnitOfWork) Do(fn func(tx *gorm.DB) error) error {
	return u.db.Transaction(fn)
}
