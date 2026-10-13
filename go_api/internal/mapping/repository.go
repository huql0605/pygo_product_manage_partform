package mapping

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Upsert 写入或更新一条规则：同 (category, source_column) 已存在时只更新 target_field。
// 这样「反复配置同一件事」不会报重复错误，配错了也能直接改。
func (r *Repository) Upsert(m *FieldMapping) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "category"}, {Name: "source_column"}},
		DoUpdates: clause.AssignmentColumns([]string{"target_field", "updated_at"}),
	}).Create(m).Error
}

// ListByCategory 返回某品类的全部映射规则。
func (r *Repository) ListByCategory(category string) ([]FieldMapping, error) {
	var out []FieldMapping
	err := r.db.Where("category = ?", category).Order("id").Find(&out).Error
	return out, err
}
