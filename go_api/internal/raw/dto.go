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

// ListImportsResult 批次列表的分页结果。
type ListImportsResult struct {
	Items []ImportBatch `json:"items"`
	Total int64         `json:"total"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
}

// DeleteImportResult 删除批次的结果。删除会同时清掉该批次的明细行，
// 并重建汇总层，因此需要把这两步的结果回报给调用方。
type DeleteImportResult struct {
	BatchID          uint   `json:"batch_id"`
	DeletedSales     int64  `json:"deleted_sales"`
	SummaryRefreshed bool   `json:"summary_refreshed"`
	SummaryError     string `json:"summary_error,omitempty"`
}
