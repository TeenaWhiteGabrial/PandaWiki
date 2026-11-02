package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

// PrepareNodeIndex persists the attempt before uploading. Retries reuse it;
// candidates never reuse or change the live RAG document's ID.
func (r *NodeRepository) PrepareNodeIndex(ctx context.Context, v *domain.NodeIndexVersion) (*domain.NodeIndexVersion, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, v.KBID); err != nil {
			return err
		}
		if v.TaskID != "" {
			var task domain.NodeLearningTask
			if err := tx.First(&task, "id = ?", v.TaskID).Error; err != nil {
				return err
			}
			if task.Status != consts.NodeLearningTaskWaiting && task.Status != consts.NodeLearningTaskRunning {
				v.Status = domain.IndexCanceled
				return nil
			}
		}
		var existing domain.NodeIndexVersion
		if err := tx.First(&existing, "id = ?", v.ID).Error; err == nil {
			*v = existing
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var kb domain.KnowledgeBase
		if err := tx.First(&kb, "id = ?", v.KBID).Error; err != nil {
			return err
		}
		var node domain.Node
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&node, "id = ? AND kb_id = ?", v.NodeID, v.KBID).Error; err != nil {
			return err
		}
		if v.BuildID == "" {
			if kb.DatasetID != v.DatasetID {
				v.Status = domain.IndexRetired
				v.Message = "知识库索引已切换，此旧任务不再执行"
				if err := tx.Create(v).Error; err != nil {
					return err
				}
				return finishIndexTask(tx, v, consts.NodeLearningTaskSkipped, v.Message)
			}
			latest, err := currentIndexRelease(tx, v.KBID, v.NodeID)
			if err != nil {
				return err
			}
			if latest.ID != v.NodeReleaseID {
				v.Status = domain.IndexRetired
				v.Message = "此文档版本已被更新，跳过旧学习任务"
				if err := tx.Create(v).Error; err != nil {
					return err
				}
				return finishIndexTask(tx, v, consts.NodeLearningTaskSkipped, v.Message)
			}
			if node.RagInfo.HasAvailableIndex() && node.RagInfo.ActiveDocID == "" {
				var old domain.NodeRelease
				if err := tx.Where("kb_id = ? AND node_id = ? AND doc_id <> ''", v.KBID, v.NodeID).Order("updated_at DESC, id DESC").First(&old).Error; err == nil {
					node.RagInfo.ActiveDocID, node.RagInfo.ActiveReleaseID = old.DocID, old.ID
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			if err := tx.Model(&domain.NodeIndexVersion{}).Where("kb_id = ? AND node_id = ? AND build_id = '' AND status = ?", v.KBID, v.NodeID, domain.IndexBuilding).
				Updates(map[string]any{"status": domain.IndexRetired, "message": "已由更新的学习任务替代", "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			node.RagInfo.PendingIndexID = v.ID
			if err := tx.Model(&node).UpdateColumn("rag_info", &node.RagInfo).Error; err != nil {
				return err
			}
		} else {
			var build domain.KBIndexBuild
			if err := tx.First(&build, "id = ? AND kb_id = ?", v.BuildID, v.KBID).Error; err != nil {
				return err
			}
			if build.Status != domain.IndexBuilding || build.DatasetID != v.DatasetID || build.SourceDatasetID != kb.DatasetID {
				return fmt.Errorf("此模型重建已结束或失效")
			}
		}
		v.Status = domain.IndexBuilding
		return tx.Create(v).Error
	})
	return v, err
}

func (r *NodeRepository) FailPendingNodeIndex(ctx context.Context, kbID, nodeID, versionID, message string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, kbID); err != nil {
			return err
		}
		var node domain.Node
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&node, "id = ? AND kb_id = ?", nodeID, kbID).Error; err != nil {
			return err
		}
		if node.RagInfo.PendingIndexID == "" || node.RagInfo.PendingIndexID != versionID {
			return nil
		}
		if err := tx.Model(&domain.NodeIndexVersion{}).Where("id = ? AND status = ?", node.RagInfo.PendingIndexID, domain.IndexBuilding).Updates(map[string]any{"status": domain.IndexFailed, "message": message, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		node.RagInfo.PendingIndexID = ""
		node.RagInfo.TaskStatus = consts.NodeLearningTaskFailed
		node.RagInfo.TaskMessage = message
		node.RagInfo.SyncedAt = time.Now()
		if node.RagInfo.LearningProgress != nil {
			node.RagInfo.LearningProgress.Finish(time.Now())
		}
		if !node.RagInfo.HasAvailableIndex() {
			node.RagInfo.Status = consts.NodeRagStatusFailed
			node.RagInfo.Message = message
		}
		return tx.Model(&node).UpdateColumn("rag_info", &node.RagInfo).Error
	})
}

func (r *NodeRepository) BindNodeIndexDocument(ctx context.Context, versionID, docID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var v domain.NodeIndexVersion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, "id = ?", versionID).Error; err != nil {
			return err
		}
		if v.DocID != "" && v.DocID != docID {
			return fmt.Errorf("索引任务已绑定其他文档，拒绝覆盖")
		}
		if err := tx.Model(&v).Updates(map[string]any{"doc_id": docID, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if v.TaskID != "" {
			return tx.Model(&domain.NodeLearningTask{}).Where("id = ?", v.TaskID).Updates(map[string]any{"doc_id": docID, "updated_at": time.Now()}).Error
		}
		return nil
	})
}

func (r *NodeRepository) SaveNodeIndexExtraction(ctx context.Context, id, content string, meta domain.NodeMeta) error {
	return r.db.WithContext(ctx).Model(&domain.NodeIndexVersion{}).Where("id = ? AND status = ? AND doc_id = ''", id, domain.IndexBuilding).
		Updates(map[string]any{"parsed_content": content, "parsed_meta": meta, "updated_at": time.Now()}).Error
}

func (r *NodeRepository) GetNodeIndexByDocument(ctx context.Context, docID string) (*domain.NodeIndexVersion, error) {
	var v domain.NodeIndexVersion
	err := r.db.WithContext(ctx).First(&v, "doc_id = ?", docID).Error
	return &v, err
}

func finishIndexTask(tx *gorm.DB, v *domain.NodeIndexVersion, status consts.NodeLearningTaskStatus, message string) error {
	if v.TaskID == "" {
		return nil
	}
	return tx.Model(&domain.NodeLearningTask{}).Where("id = ? AND status IN ?", v.TaskID, []consts.NodeLearningTaskStatus{consts.NodeLearningTaskWaiting, consts.NodeLearningTaskRunning}).
		Updates(map[string]any{"status": status, "doc_id": v.DocID, "error_message": message, "finished_at": time.Now(), "updated_at": time.Now()}).Error
}

func saveIndexResult(tx *gorm.DB, v *domain.NodeIndexVersion, status, message string) error {
	v.Status, v.Message, v.UpdatedAt = status, message, time.Now()
	return tx.Model(v).Updates(map[string]any{"status": status, "message": message, "updated_at": v.UpdatedAt}).Error
}

// ObserveNodeIndex is the only place a successful candidate becomes live.
// All pointer, release, and task updates are one transaction. Late, canceled,
// superseded and failed attempts can never replace the serving version.
func (r *NodeRepository) ObserveNodeIndex(ctx context.Context, docID, status, message string) (bool, error) {
	v, err := r.GetNodeIndexByDocument(ctx, docID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, v.KBID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(v, "id = ?", v.ID).Error; err != nil {
			return err
		}
		if v.Status != domain.IndexBuilding {
			return nil
		}
		var node domain.Node
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&node, "id = ? AND kb_id = ?", v.NodeID, v.KBID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return saveIndexResult(tx, v, domain.IndexRetired, "文档已删除")
			}
			return err
		}
		if v.TaskID != "" {
			var task domain.NodeLearningTask
			if err := tx.First(&task, "id = ?", v.TaskID).Error; err != nil {
				return err
			}
			if task.Status == consts.NodeLearningTaskCanceled || task.Status == consts.NodeLearningTaskCancelRequested || task.Status == consts.NodeLearningTaskFailed {
				return saveIndexResult(tx, v, domain.IndexCanceled, "学习已取消或失败，保留原索引")
			}
		}
		ragInfo := node.RagInfo
		ownsUI := (v.TaskID != "" && ragInfo.TaskID == v.TaskID) || (v.TaskID == "" && ragInfo.PendingIndexID == v.ID)
		status = strings.ToUpper(status)
		if ownsUI && ragInfo.LearningProgress != nil {
			p := ragInfo.LearningProgress
			now := time.Now()
			p.LastCheckedAt = &now
			p.CheckError = ""
			p.RAGStatus = status
			p.DatasetID = v.DatasetID
			p.DocID = docID
			if status == "PENDING" {
				p.Advance("rag_pending", message, now)
			}
			if status == "RUNNING" || status == "REINDEXING" {
				p.Advance("rag_running", message, now)
			}
		}
		switch status {
		case "SUCCEEDED":
			if v.BuildID != "" {
				if err := finishIndexTask(tx, v, consts.NodeLearningTaskSucceeded, "新索引已就绪，等待整批切换"); err != nil {
					return err
				}
				if ownsUI {
					ragInfo.TaskStatus = consts.NodeLearningTaskSucceeded
					ragInfo.TaskMessage = "新索引已就绪，问答继续使用原索引，等待整批切换"
				}
				if err := saveIndexResult(tx, v, domain.IndexReady, message); err != nil {
					return err
				}
			} else {
				var kb domain.KnowledgeBase
				if err := tx.First(&kb, "id = ?", v.KBID).Error; err != nil {
					return err
				}
				latest, err := currentIndexRelease(tx, v.KBID, v.NodeID)
				if err != nil {
					return err
				}
				if kb.DatasetID != v.DatasetID || ragInfo.PendingIndexID != v.ID || latest.ID != v.NodeReleaseID {
					if err := finishIndexTask(tx, v, consts.NodeLearningTaskSkipped, "较新版本已生效，此结果不再切换"); err != nil {
						return err
					}
					return saveIndexResult(tx, v, domain.IndexRetired, "旧结果不再切换")
				}
				if err := activateIndex(tx, &node, v); err != nil {
					return err
				}
				return finishIndexTask(tx, v, consts.NodeLearningTaskSucceeded, message)
			}
		case "FAILED":
			if err := saveIndexResult(tx, v, domain.IndexFailed, message); err != nil {
				return err
			}
			if err := finishIndexTask(tx, v, consts.NodeLearningTaskFailed, message); err != nil {
				return err
			}
			if ownsUI {
				if ragInfo.PendingIndexID == v.ID {
					ragInfo.PendingIndexID = ""
				}
				ragInfo.TaskStatus = consts.NodeLearningTaskFailed
				ragInfo.TaskMessage = message
				if !ragInfo.HasAvailableIndex() {
					ragInfo.Status = consts.NodeRagStatusFailed
					ragInfo.Message = message
				}
			}
		case "PENDING", "RUNNING", "REINDEXING":
			if ownsUI && !ragInfo.HasAvailableIndex() {
				ragInfo.Status = consts.NodeRagInfoStatus(status)
				ragInfo.Message = message
			}
		default:
			return nil
		}
		if ownsUI {
			ragInfo.SyncedAt = time.Now()
			if (status == "FAILED" || status == "SUCCEEDED") && ragInfo.LearningProgress != nil {
				ragInfo.LearningProgress.Finish(time.Now())
			}
			return tx.Model(&node).UpdateColumn("rag_info", &ragInfo).Error
		}
		return nil
	})
	return true, err
}

func activateIndex(tx *gorm.DB, node *domain.Node, v *domain.NodeIndexVersion) error {
	return activateIndexFromDataset(tx, node, v, v.DatasetID)
}

func activateIndexFromDataset(tx *gorm.DB, node *domain.Node, v *domain.NodeIndexVersion, previousDataset string) error {
	now := time.Now()
	// Persist the previous live document as a cleanup record, including legacy
	// installations without index-version rows. Never delete it during upload.
	old := node.RagInfo.ActiveDocID
	if old != "" && old != v.DocID {
		legacy := domain.NodeIndexVersion{ID: uuid.NewSHA1(uuid.NameSpaceOID, []byte(previousDataset+":"+old)).String(), KBID: v.KBID, NodeID: v.NodeID, NodeReleaseID: node.RagInfo.ActiveReleaseID, DatasetID: previousDataset, DocID: old, Status: domain.IndexRetired, CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&legacy).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&domain.NodeIndexVersion{}).Where("kb_id = ? AND node_id = ? AND dataset_id = ? AND id <> ? AND status = ?", v.KBID, v.NodeID, previousDataset, v.ID, domain.IndexActive).
		Updates(map[string]any{"status": domain.IndexRetired, "updated_at": now}).Error; err != nil {
		return err
	}
	releaseUpdates := map[string]any{"doc_id": v.DocID}
	if v.ParsedContent != nil {
		releaseUpdates["content"] = *v.ParsedContent
		releaseUpdates["meta"] = v.ParsedMeta
	}
	if err := tx.Model(&domain.NodeRelease{}).Where("id = ? AND node_id = ? AND kb_id = ?", v.NodeReleaseID, v.NodeID, v.KBID).UpdateColumns(releaseUpdates).Error; err != nil {
		return err
	}
	node.RagInfo.Status = consts.NodeRagStatusSucceeded
	node.RagInfo.Message = "索引已完成"
	node.RagInfo.ActiveDocID = v.DocID
	node.RagInfo.ActiveReleaseID = v.NodeReleaseID
	node.RagInfo.PendingIndexID = ""
	node.RagInfo.SyncedAt = now
	if v.TaskID == "" || node.RagInfo.TaskID == v.TaskID {
		node.RagInfo.TaskStatus = consts.NodeLearningTaskSucceeded
		node.RagInfo.TaskMessage = "新索引已生效"
		if node.RagInfo.LearningProgress != nil {
			node.RagInfo.LearningProgress.Finish(now)
		}
	}
	if err := tx.Model(node).UpdateColumn("rag_info", &node.RagInfo).Error; err != nil {
		return err
	}
	return saveIndexResult(tx, v, domain.IndexActive, "新索引已生效")
}

func (r *NodeRepository) ListBuildingNodeIndexes(ctx context.Context) ([]domain.NodeIndexVersion, error) {
	var result []domain.NodeIndexVersion
	err := r.db.WithContext(ctx).Where("status = ? AND doc_id <> ''", domain.IndexBuilding).Order("updated_at").Find(&result).Error
	return result, err
}

func (r *NodeRepository) ListRetiredNodeIndexes(ctx context.Context, before time.Time, limit int) ([]domain.NodeIndexVersion, error) {
	var result []domain.NodeIndexVersion
	err := r.db.WithContext(ctx).Model(&domain.NodeIndexVersion{}).Select("node_index_versions.*").
		Joins("JOIN knowledge_bases kb ON kb.id=node_index_versions.kb_id AND kb.dataset_id=node_index_versions.dataset_id").
		Where("node_index_versions.status IN ? AND node_index_versions.doc_id <> '' AND node_index_versions.updated_at < ?", []string{domain.IndexRetired, domain.IndexFailed, domain.IndexCanceled}, before).Order("node_index_versions.updated_at").Limit(limit).Find(&result).Error
	return result, err
}

func (r *NodeRepository) MarkNodeIndexDeleted(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&domain.NodeIndexVersion{}).Where("id = ? AND status IN ?", id, []string{domain.IndexRetired, domain.IndexFailed, domain.IndexCanceled}).Updates(map[string]any{"status": domain.IndexDeleted, "updated_at": time.Now()}).Error
}

func (r *NodeRepository) FilterServingIndexDocuments(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return ids, nil
	}
	var result []string
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT r.doc_id FROM node_releases r JOIN nodes n ON n.id=r.node_id AND n.kb_id=r.kb_id
	 WHERE r.doc_id IN ? AND (COALESCE(n.rag_info->>'active_doc_id','')='' OR n.rag_info->>'active_doc_id'=r.doc_id)
	 AND NOT EXISTS (SELECT 1 FROM node_index_versions v WHERE v.doc_id=r.doc_id AND v.status<>?)`, ids, domain.IndexActive).Scan(&result).Error
	return result, err
}

func (r *NodeRepository) ReconcileTerminalIndexTasks(ctx context.Context) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`UPDATE node_learning_tasks t SET status='skipped',error_message='已由更新的索引版本替代',finished_at=now(),updated_at=now()
		 FROM node_index_versions v WHERE v.task_id=t.id AND v.status='retired' AND t.status IN ('waiting','running')`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`UPDATE node_index_versions v SET status=CASE WHEN t.status='failed' THEN 'failed' ELSE 'canceled' END,
		 message=t.error_message,updated_at=now() FROM node_learning_tasks t WHERE v.task_id=t.id AND v.status='building'
		 AND t.status IN ('failed','canceled','cancel_requested','skipped')`).Error; err != nil {
			return err
		}
		return tx.Exec(`UPDATE nodes n SET rag_info=jsonb_set(n.rag_info,'{pending_index_id}','""'::jsonb)
		 FROM node_index_versions v WHERE n.rag_info->>'pending_index_id'=v.id AND v.status IN ('failed','canceled','retired','deleted')`).Error
	})
}

func (r *NodeRepository) CanDeleteRetiredIndex(ctx context.Context, v domain.NodeIndexVersion) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&domain.Node{}).Where("rag_info->>'active_doc_id' = ?", v.DocID).Count(&count).Error; err != nil {
		return false, err
	}
	if count != 0 {
		return false, nil
	}
	var kb domain.KnowledgeBase
	if err := r.db.WithContext(ctx).First(&kb, "id = ?", v.KBID).Error; err != nil {
		return false, err
	}
	return kb.DatasetID == v.DatasetID, nil
}
