package product

import "time"

type Product struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"not null;index"`
	Price     float64   `json:"price" gorm:"not null"`
	CreateAt  time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
