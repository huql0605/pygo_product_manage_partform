package product

import "time"

type Product struct {
	Id        uint      `json:"id" gorm:"primaryKey"`
	Productor string    `json:"productor" gorm:"not null"`
	Name      string    `json:"name" gorm:"not null;index"`
	Price     float64   `json:"price" gorm:"not null"`
	Sales     int       `json:"sales" gorm:"not null"`
	Month     time.Time `json:"month" gorm:"not null"`
	CreateAt  time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
