package raw

import "fmt"

type CreateImportReq struct {
	Source   string           `json:"source" binding:"required,min=1,max=255"`
	Category string           `json:"category" binding:"required,min=1,max=50"`
	Rows     []map[string]any `json:"rows" binding:"required,min=1,max=10000"`
}
type ColumnInfo struct {
	Column string `json:"column" gorm:"column:column_name"`
	Sample string `json:"sample" gorm:"column:sample_value"`
}

func (r *CreateImportReq) Validate() error {
	for i, row := range r.Rows {
		if len(row) == 0 {
			return fmt.Errorf("第 %d 行为空对象", i+1)
		}
	}
	return nil
}
