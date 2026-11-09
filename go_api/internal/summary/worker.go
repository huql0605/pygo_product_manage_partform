package summary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var errNoTask = errors.New("没有待处理的汇总刷新任务")

// Worker 轮询 summary_refresh_tasks，把汇总刷新从 HTTP 请求里解耦。
type Worker struct {
	repo     *Repository
	interval time.Duration
	logger   *log.Logger
}

func NewWorker(repo *Repository, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = time.Second
	}
	return &Worker{repo: repo, interval: interval, logger: log.Default()}
}

func (w *Worker) WithLogger(logger *log.Logger) *Worker {
	w.logger = logger
	return w
}

// Run 持续消费任务，直到 ctx 取消。
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		for {
			processed, err := w.ProcessOne(ctx)
			if err != nil {
				w.logger.Printf("summary worker: %v", err)
				break
			}
			if !processed {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// ProcessOne 处理一条任务。返回 false 表示当前没有待处理任务。
func (w *Worker) ProcessOne(ctx context.Context) (bool, error) {
	var task RefreshTask
	var refreshErr error

	err := w.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND run_after <= ?", TaskPending, time.Now()).
			Order("id").
			First(&task).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errNoTask
		}
		if err != nil {
			return err
		}

		var grains []Grain
		if err := json.Unmarshal([]byte(task.Grains), &grains); err != nil {
			refreshErr = fmt.Errorf("解析任务 %d 的 grain 失败: %w", task.ID, err)
			return refreshErr
		}
		if err := w.repo.RefreshGrainsTx(tx, grains); err != nil {
			refreshErr = err
			return err
		}

		return tx.Model(&RefreshTask{}).
			Where("id = ?", task.ID).
			Updates(map[string]any{"status": TaskDone, "last_error": ""}).Error
	})

	if errors.Is(err, errNoTask) {
		return false, nil
	}
	if err != nil {
		if refreshErr == nil {
			return true, err
		}
		if recordErr := w.recordFailure(ctx, task, refreshErr); recordErr != nil {
			return true, fmt.Errorf("刷新失败: %v；记录失败状态失败: %w", refreshErr, recordErr)
		}
		w.logger.Printf("summary refresh task %d failed: %v", task.ID, refreshErr)
		return true, nil
	}
	return true, nil
}

func (w *Worker) recordFailure(ctx context.Context, task RefreshTask, cause error) error {
	attempts := task.Attempts + 1
	status := TaskPending
	if attempts >= maxTaskAttempts {
		status = TaskFailed
	}
	backoff := time.Duration(1<<min(attempts, 6)) * time.Second

	return w.repo.db.WithContext(ctx).
		Model(&RefreshTask{}).
		Where("id = ?", task.ID).
		Updates(map[string]any{
			"status":     status,
			"attempts":   attempts,
			"run_after":  time.Now().Add(backoff),
			"last_error": cause.Error(),
		}).Error
}
