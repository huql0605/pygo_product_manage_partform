package database

import (
	"gorm.io/driver/postgres" //type:ignore
	"gorm.io/gorm"
)

func NewPostgres(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}
