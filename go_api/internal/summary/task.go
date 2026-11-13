package summary

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

const (
	TaskPending = "pending"
	TaskDone    = "done"
	TaskFailed  = "failed"

	maxTaskAttempts = 10
)

// RefreshTask 是汇总刷新的 outbox 记录，对应表 summary_refresh_tasks。
// 它和明细写入在同一个事务里落库，worker 只处理已提交的任务。
type RefreshTask struct {
	// ID 自增主键。
	ID int64 `json:"id" gorm:"primarykey"`
	// Grains 待重算的 grain 列表，JSON 数组文本，元素为 {product_id, productor, name, month}。
	Grains string `json:"grains" gorm:"type:text;not null"`
	// Status 任务状态：pending / done / failed，见 TaskPending 等常量。
	Status string `json:"status" gorm:"not null;default:pending;index"`
	// Attempts 已失败次数；达到 maxTaskAttempts（10）后状态置为 failed。
	Attempts int `json:"attempts" gorm:"not null;default:0"`
	// RunAfter 下次可执行时间；失败后按 2^attempts 秒退避（最大 64 秒）。
	RunAfter time.Time `json:"run_after" gorm:"not null;default:now();index"`
	// LastError 最近一次失败原因；刷新成功后清空。
	LastError string `json:"last_error" gorm:"type:text;not null;default:''"`
	// CreatedAt 任务入队时间，由 GORM 自动维护。
	CreatedAt time.Time `json:"created_at"`
}

func (RefreshTask) TableName() string {
	return "summary_refresh_tasks"
}

// EnqueueRefreshTx 在调用方事务内写入一条刷新任务。
// 明细写入和任务入队必须同事务，否则进程崩溃会丢刷新。
func EnqueueRefreshTx(tx *gorm.DB, grains []Grain) error {
	grains = uniqueGrains(grains)
	if len(grains) == 0 {
		return nil
	}
	payload, err := json.Marshal(grains)
	if err != nil {
		return err
	}
	return tx.Create(&RefreshTask{
		Grains:   string(payload),
		Status:   TaskPending,
		RunAfter: time.Now(),
	}).Error
}
