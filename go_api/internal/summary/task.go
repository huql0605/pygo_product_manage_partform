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

// RefreshTask 是汇总刷新的 outbox 记录。
// 它和明细写入在同一个事务里落库，worker 只处理已提交的任务。
type RefreshTask struct {
	ID        int64     `json:"id" gorm:"primarykey"`
	Grains    string    `json:"grains" gorm:"type:text;not null"`
	Status    string    `json:"status" gorm:"not null;default:pending;index"`
	Attempts  int       `json:"attempts" gorm:"not null;default:0"`
	RunAfter  time.Time `json:"run_after" gorm:"not null;default:now();index"`
	LastError string    `json:"last_error" gorm:"type:text;not null;default:''"`
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
