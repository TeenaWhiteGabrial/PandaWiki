package pg

import (
	"context"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *NodeRepository) updateLearningProgress(ctx context.Context, kbID, nodeID, taskID string, update func(*domain.LearningProgress)) error {
	if taskID == "" {
		return nil
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var node domain.Node
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("kb_id = ? AND id = ?", kbID, nodeID).First(&node).Error; err != nil {
			return err
		}
		// A delayed worker/event must not overwrite a newer attempt's timeline.
		if node.RagInfo.TaskID != taskID {
			return nil
		}
		if node.RagInfo.LearningProgress == nil {
			node.RagInfo.LearningProgress = &domain.LearningProgress{}
		}
		update(node.RagInfo.LearningProgress)
		return tx.Model(&domain.Node{}).Where("kb_id = ? AND id = ?", kbID, nodeID).UpdateColumn("rag_info", &node.RagInfo).Error
	})
}

func (r *NodeRepository) RecordNodeLearningStage(ctx context.Context, kbID, nodeID, taskID, stage, message string) error {
	return r.updateLearningProgress(ctx, kbID, nodeID, taskID, func(p *domain.LearningProgress) { p.Advance(stage, message, time.Now()) })
}

// ObserveNodeLearningRAG records successful checks separately from real progress.
// API failures and missing documents remain visible instead of masquerading as work.
func (r *NodeRepository) ObserveNodeLearningRAG(ctx context.Context, kbID, nodeID, taskID, datasetID, docID, status, message, source, checkError string) error {
	return r.updateLearningProgress(ctx, kbID, nodeID, taskID, func(p *domain.LearningProgress) {
		now := time.Now()
		p.LastCheckedAt = &now
		p.CheckError = checkError
		p.Source = source
		if datasetID != "" {
			p.DatasetID = datasetID
		}
		p.DocID = docID
		if checkError != "" {
			return
		}
		if status != "" && !(status == string(consts.NodeRagStatusPending) && p.RAGStatus == string(consts.NodeRagStatusRunning)) {
			p.RAGStatus = status
		}
		switch consts.NodeRagInfoStatus(status) {
		case consts.NodeRagStatusPending:
			p.Advance("rag_pending", message, now)
		case consts.NodeRagStatusRunning, consts.NodeRagStatusReindexing:
			p.Advance("rag_running", message, now)
		case consts.NodeRagStatusSucceeded, consts.NodeRagStatusFailed:
			p.Finish(now)
		}
	})
}
