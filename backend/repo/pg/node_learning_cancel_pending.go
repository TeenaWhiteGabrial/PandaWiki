package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func learningCancelKey(kbID string) consts.SystemSettingKey {
	return consts.SystemSettingKey("learning_cancel_pending:" + kbID)
}

func lockLearningSubmission(tx *gorm.DB, kbID string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "learning_submission:"+kbID).Error
}

// Cancellation and the last pre-upload check share a lock. Once submission has
// been claimed it is deliberately left alone, including an HTTP call in flight.
func (r *NodeRepository) BeginLearningSubmission(ctx context.Context, kbID, nodeID, taskID string) (bool, error) {
	allowed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, kbID); err != nil {
			return err
		}
		var task domain.NodeLearningTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND kb_id = ? AND node_id = ?", taskID, kbID, nodeID).First(&task).Error; err != nil {
			return err
		}
		if task.Status != consts.NodeLearningTaskRunning && task.Status != consts.NodeLearningTaskWaiting {
			return nil
		}
		if task.DocID != "" {
			return nil
		}
		var node domain.Node
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND kb_id = ?", nodeID, kbID).First(&node).Error; err != nil {
			return err
		}
		if node.RagInfo.TaskID != taskID {
			return nil
		}
		if node.RagInfo.LearningProgress == nil {
			node.RagInfo.LearningProgress = &domain.LearningProgress{}
		}
		now := time.Now()
		node.RagInfo.LearningProgress.SubmissionStartedAt = &now
		if err := tx.Model(&node).UpdateColumn("rag_info", &node.RagInfo).Error; err != nil {
			return err
		}
		allowed = true
		return nil
	})
	return allowed, err
}

func (r *NodeRepository) LegacyLearningCanceled(ctx context.Context, kbID string, sequence uint64, publishedAt time.Time) (bool, error) {
	return legacyLearningCanceled(r.db.WithContext(ctx), kbID, sequence, publishedAt)
}

func legacyLearningCanceled(tx *gorm.DB, kbID string, sequence uint64, publishedAt time.Time) (bool, error) {
	var setting domain.SystemSetting
	result := tx.Where("key = ?", learningCancelKey(kbID)).Limit(1).Find(&setting)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	var cutoff domain.VectorQueueBoundary
	if err := json.Unmarshal(setting.Value, &cutoff); err != nil {
		return false, err
	}
	if cutoff.Sequence == 0 {
		return false, nil
	}
	if sequence == 0 || publishedAt.IsZero() {
		return false, fmt.Errorf("旧任务缺少队列位置，无法安全判断取消范围")
	}
	return sequence <= cutoff.Sequence && !publishedAt.After(cutoff.PublishedAt), nil
}

func (r *NodeRepository) BeginLegacyLearningSubmission(ctx context.Context, kbID string, sequence uint64, publishedAt time.Time) (bool, error) {
	allowed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, kbID); err != nil {
			return err
		}
		canceled, err := legacyLearningCanceled(tx, kbID, sequence, publishedAt)
		allowed = !canceled && err == nil
		return err
	})
	return allowed, err
}

func submissionHasStarted(task domain.NodeLearningTask, node domain.Node) bool {
	if task.DocID != "" {
		return true
	}
	if node.RagInfo.TaskID != task.ID || node.RagInfo.LearningProgress == nil {
		return false
	}
	p := node.RagInfo.LearningProgress
	if p.SubmissionStartedAt != nil || p.DocID != "" {
		return true
	}
	for _, stage := range p.Stages {
		switch stage.Key {
		case "submit", "bind_doc", "rag_pending", "rag_running":
			return true
		}
	}
	return false
}

// Persist a legacy-message cutoff and cancel registered, unsubmitted tasks in
// one transaction. No RAGLite documents, stream messages or source files are deleted.
func (r *NodeRepository) CancelPendingLearning(ctx context.Context, kbID string, cutoff domain.VectorQueueBoundary) (int, int, error) {
	canceled, protected := 0, 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, kbID); err != nil {
			return err
		}
		var old domain.SystemSetting
		if err := tx.Where("key = ?", learningCancelKey(kbID)).First(&old).Error; err == nil {
			var previous domain.VectorQueueBoundary
			if err := json.Unmarshal(old.Value, &previous); err != nil {
				return err
			}
			if previous.PublishedAt.After(cutoff.PublishedAt) || (previous.PublishedAt.Equal(cutoff.PublishedAt) && previous.Sequence > cutoff.Sequence) {
				cutoff = previous
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		value, err := json.Marshal(cutoff)
		if err != nil {
			return err
		}
		setting := domain.SystemSetting{Key: learningCancelKey(kbID), Value: value, Description: "旧版学习队列取消边界"}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"})}).Create(&setting).Error; err != nil {
			return err
		}
		var tasks []domain.NodeLearningTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kb_id = ? AND status IN ?", kbID, []consts.NodeLearningTaskStatus{consts.NodeLearningTaskWaiting, consts.NodeLearningTaskRunning, consts.NodeLearningTaskCancelRequested}).Order("id").Find(&tasks).Error; err != nil {
			return err
		}
		for _, task := range tasks {
			var node domain.Node
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND kb_id = ?", task.NodeID, kbID).First(&node).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					protected++
					continue
				}
				return err
			}
			if submissionHasStarted(task, node) {
				protected++
				continue
			}
			now := time.Now()
			if err := tx.Model(&task).Updates(map[string]any{"status": consts.NodeLearningTaskCanceled, "error_message": "已取消，未提交给 RAGLite", "finished_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
			if node.RagInfo.TaskID == task.ID {
				node.RagInfo.TaskStatus = consts.NodeLearningTaskCanceled
				node.RagInfo.TaskMessage = "已取消，未提交给 RAGLite"
				if node.RagInfo.LearningProgress != nil {
					node.RagInfo.LearningProgress.Finish(now)
				}
				if err := tx.Model(&node).UpdateColumn("rag_info", &node.RagInfo).Error; err != nil {
					return err
				}
			}
			canceled++
		}
		return nil
	})
	return canceled, protected, err
}
