package pg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *NodeRepository) GetPendingIndexBuild(ctx context.Context, kbID string) (*domain.KBIndexBuild, error) {
	var build domain.KBIndexBuild
	err := r.db.WithContext(ctx).Where("kb_id = ? AND status = ?", kbID, domain.IndexBuilding).First(&build).Error
	return &build, err
}

func (r *NodeRepository) GetLatestIndexBuild(ctx context.Context, kbID string) (*domain.KBIndexBuild, error) {
	var build domain.KBIndexBuild
	err := r.db.WithContext(ctx).Where("kb_id = ?", kbID).Order("created_at DESC, id DESC").First(&build).Error
	return &build, err
}

func latestIndexReleases(tx *gorm.DB, kbID string, nodeIDs ...string) ([]domain.NodeRelease, error) {
	var releases []domain.NodeRelease
	query := tx.Model(&domain.NodeRelease{}).Select("DISTINCT ON (node_releases.node_id) node_releases.*").
		Joins("JOIN nodes ON nodes.id=node_releases.node_id AND nodes.kb_id=node_releases.kb_id").
		Where("node_releases.kb_id = ? AND node_releases.type = ? AND nodes.type = ?", kbID, domain.NodeTypeDocument, domain.NodeTypeDocument)
	var publication domain.KBRelease
	if err := tx.Where("kb_id = ?", kbID).Order("created_at DESC, id DESC").First(&publication).Error; err == nil {
		query = query.Joins("JOIN kb_release_node_releases published ON published.node_release_id=node_releases.id AND published.release_id = ?", publication.ID)
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	} else {
		query = query.Where("nodes.status <> ? OR COALESCE(nodes.rag_info->>'active_doc_id','') <> ''", domain.NodeStatusUnreleased)
	}
	if len(nodeIDs) > 0 {
		query = query.Where("node_releases.node_id IN ?", nodeIDs)
	}
	err := query.Order("node_releases.node_id, node_releases.updated_at DESC, node_releases.id DESC").Find(&releases).Error
	return releases, err
}

func (r *NodeRepository) GetPublishedIndexReleases(ctx context.Context, kbID string, nodeIDs []string) ([]domain.NodeRelease, error) {
	if len(nodeIDs) == 0 {
		return []domain.NodeRelease{}, nil
	}
	return latestIndexReleases(r.db.WithContext(ctx), kbID, nodeIDs...)
}

func currentIndexRelease(tx *gorm.DB, kbID, nodeID string) (*domain.NodeRelease, error) {
	releases, err := latestIndexReleases(tx, kbID, nodeID)
	if err != nil {
		return nil, err
	}
	if len(releases) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &releases[0], nil
}

// StartIndexBuild creates visible, ordinary learning tasks in one transaction.
// The serving dataset and all serving document pointers remain untouched.
func (r *NodeRepository) StartIndexBuild(ctx context.Context, build *domain.KBIndexBuild) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, build.KBID); err != nil {
			return err
		}
		var kb domain.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&kb, "id = ?", build.KBID).Error; err != nil {
			return err
		}
		if kb.DatasetID != build.SourceDatasetID {
			return fmt.Errorf("知识库索引已变化，请重新保存配置")
		}
		var count int64
		if err := tx.Model(&domain.KBIndexBuild{}).Where("kb_id = ? AND status = ?", kb.ID, domain.IndexBuilding).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("已有模型索引重建正在进行，请等待其结束后再切换模型")
		}
		if err := tx.Model(&domain.NodeLearningTask{}).Where("kb_id = ? AND status IN ?", kb.ID, []consts.NodeLearningTaskStatus{consts.NodeLearningTaskWaiting, consts.NodeLearningTaskRunning, consts.NodeLearningTaskCancelRequested}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("尚有文档学习任务，请等待完成或取消未提交任务后再切换向量模型")
		}
		releases, err := latestIndexReleases(tx, kb.ID)
		if err != nil {
			return err
		}
		build.Status = domain.IndexBuilding
		if err := tx.Create(build).Error; err != nil {
			return err
		}
		for _, release := range releases {
			id := uuid.NewString()
			task := domain.NodeLearningTask{ID: id, KBID: kb.ID, NodeID: release.NodeID, NodeReleaseID: release.ID, DatasetID: build.DatasetID, IndexBuildID: build.ID, Action: consts.NodeLearningTaskActionRebuild, Status: consts.NodeLearningTaskWaiting, Priority: 10, TriggeredBy: "model_index_build"}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			version := domain.NodeIndexVersion{ID: id, KBID: kb.ID, NodeID: release.NodeID, NodeReleaseID: release.ID, DatasetID: build.DatasetID, BuildID: build.ID, TaskID: id, Status: domain.IndexBuilding}
			if err := tx.Create(&version).Error; err != nil {
				return err
			}
			var node domain.Node
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&node, "id = ?", release.NodeID).Error; err != nil {
				return err
			}
			if node.RagInfo.HasAvailableIndex() && node.RagInfo.ActiveDocID == "" && release.DocID != "" {
				node.RagInfo.ActiveDocID = release.DocID
				node.RagInfo.ActiveReleaseID = release.ID
			}
			node.RagInfo.TaskID = id
			node.RagInfo.TaskStatus = consts.NodeLearningTaskWaiting
			node.RagInfo.TaskAction = consts.NodeLearningTaskActionRebuild
			node.RagInfo.TaskMessage = "新模型索引排队中，问答继续使用原索引"
			node.RagInfo.LearningProgress = &domain.LearningProgress{}
			node.RagInfo.LearningProgress.Advance("waiting", node.RagInfo.TaskMessage, time.Now())
			if err := tx.Model(&node).UpdateColumn("rag_info", &node.RagInfo).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CompleteIndexBuilds is restart-safe. A failed/canceled/mutated snapshot never
// switches. Both dataset and every live document pointer commit atomically.
func (r *NodeRepository) CompleteIndexBuilds(ctx context.Context) error {
	var builds []domain.KBIndexBuild
	if err := r.db.WithContext(ctx).Where("status = ?", domain.IndexBuilding).Find(&builds).Error; err != nil {
		return err
	}
	for _, build := range builds {
		if err := r.completeIndexBuild(ctx, build.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *NodeRepository) completeIndexBuild(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var build domain.KBIndexBuild
		if err := tx.First(&build, "id = ?", id).Error; err != nil {
			return err
		}
		if err := lockLearningSubmission(tx, build.KBID); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&build, "id = ?", id).Error; err != nil {
			return err
		}
		if build.Status != domain.IndexBuilding {
			return nil
		}
		var kb domain.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&kb, "id = ?", build.KBID).Error; err != nil {
			return err
		}
		var versions []domain.NodeIndexVersion
		if err := tx.Where("build_id = ?", id).Order("node_id").Find(&versions).Error; err != nil {
			return err
		}
		fail := func(message string) error {
			if err := tx.Model(&build).Updates(map[string]any{"status": domain.IndexFailed, "message": message, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			// Stop outstanding registered tasks; accepted external work may finish,
			// but its later event can never become live after this state transition.
			if err := tx.Model(&domain.NodeIndexVersion{}).Where("build_id = ? AND status IN ?", id, []string{domain.IndexBuilding, domain.IndexReady}).Updates(map[string]any{"status": domain.IndexCanceled, "message": message, "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			if err := tx.Model(&domain.NodeLearningTask{}).Where("index_build_id = ? AND status <> ?", id, consts.NodeLearningTaskFailed).Updates(map[string]any{"status": consts.NodeLearningTaskCanceled, "error_message": message, "finished_at": time.Now(), "updated_at": time.Now()}).Error; err != nil {
				return err
			}
			for _, v := range versions {
				var node domain.Node
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&node, "id = ?", v.NodeID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				} else if err != nil {
					return err
				}
				if node.RagInfo.TaskID != v.TaskID {
					continue
				}
				var task domain.NodeLearningTask
				if err := tx.First(&task, "id = ?", v.TaskID).Error; err != nil {
					return err
				}
				node.RagInfo.TaskStatus = task.Status
				node.RagInfo.TaskMessage = message
				if node.RagInfo.LearningProgress != nil {
					node.RagInfo.LearningProgress.Finish(time.Now())
				}
				if err := tx.Model(&node).UpdateColumn("rag_info", &node.RagInfo).Error; err != nil {
					return err
				}
			}
			return nil
		}
		if kb.DatasetID != build.SourceDatasetID {
			return fail("当前数据集已变化，保留现有索引")
		}
		var interrupted int64
		if err := tx.Model(&domain.NodeLearningTask{}).Where("index_build_id = ? AND status IN ?", id, []consts.NodeLearningTaskStatus{consts.NodeLearningTaskFailed, consts.NodeLearningTaskCanceled, consts.NodeLearningTaskCancelRequested, consts.NodeLearningTaskSkipped}).Count(&interrupted).Error; err != nil {
			return err
		}
		if interrupted > 0 {
			return fail("部分文档学习失败或取消，整批未切换，继续使用原索引")
		}
		for _, v := range versions {
			if v.Status == domain.IndexFailed || v.Status == domain.IndexCanceled || v.Status == domain.IndexRetired {
				return fail("部分文档未成功，整批未切换，继续使用原索引")
			}
			if v.Status != domain.IndexReady {
				return nil
			}
		}
		setting, desiredKey, err := indexBuildModelSetting(tx)
		if err != nil {
			return fail("无法确认当前向量模型，未切换索引")
		}
		if setting != nil && desiredKey != build.ConfigKey {
			return fail("重建期间向量模型配置变化，未切换索引")
		}
		latest, err := latestIndexReleases(tx, kb.ID)
		if err != nil {
			return err
		}
		if len(latest) != len(versions) {
			return fail("重建期间发布文档数量变化，整批未切换，请重新发起模型重建")
		}
		byNode := make(map[string]string, len(latest))
		for _, release := range latest {
			byNode[release.NodeID] = release.ID
		}
		for _, v := range versions {
			if byNode[v.NodeID] != v.NodeReleaseID {
				return fail("重建期间文档发布版本变化，整批未切换，请重新发起模型重建")
			}
		}
		for i := range versions {
			v := &versions[i]
			var node domain.Node
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&node, "id = ?", v.NodeID).Error; err != nil {
				return err
			}
			if err := activateIndexFromDataset(tx, &node, v, build.SourceDatasetID); err != nil {
				return err
			}
		}
		if err := tx.Model(&kb).Updates(map[string]any{"dataset_id": build.DatasetID, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if err := tx.Model(&build).Updates(map[string]any{"status": domain.IndexActive, "message": "所有文档学习成功，新模型和索引已整体生效；旧数据集保留", "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		if setting != nil {
			return advanceIndexBaseline(tx, setting, build.ConfigKey)
		}
		return nil
	})
}

func indexBuildModelSetting(tx *gorm.DB) (*domain.SystemSetting, string, error) {
	var setting domain.SystemSetting
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&setting, "key = ?", consts.SystemSettingModelMode).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, "", nil
	} else if err != nil {
		return nil, "", err
	}
	var config domain.ModelModeSetting
	if err := json.Unmarshal(setting.Value, &config); err != nil {
		return nil, "", err
	}
	var model domain.Model
	if config.Mode == consts.ModelSettingModeManual {
		if err := tx.Where("type = ? AND is_active = true", domain.ModelTypeEmbedding).First(&model).Error; err != nil {
			return nil, "", err
		}
	} else if config.AutoProvider != nil {
		model.Model = config.AutoProvider.Models.Embedding
		model.Provider = config.AutoProvider.Provider
	} else {
		return nil, "", fmt.Errorf("missing embedding selection")
	}
	key, err := json.Marshal(model.EmbeddingIndexIdentity())
	return &setting, string(key), err
}

func advanceIndexBaseline(tx *gorm.DB, setting *domain.SystemSetting, key string) error {
	var unmatched int64
	if err := tx.Raw(`SELECT count(*) FROM knowledge_bases kb LEFT JOIN LATERAL
	 (SELECT * FROM kb_index_builds b WHERE b.kb_id=kb.id ORDER BY b.created_at DESC,b.id DESC LIMIT 1) latest ON true
	 WHERE latest.id IS NULL OR latest.status <> 'active' OR latest.config_key <> ? OR latest.dataset_id <> kb.dataset_id`, key).Scan(&unmatched).Error; err != nil {
		return err
	}
	if unmatched > 0 {
		return nil
	}
	var config domain.ModelModeSetting
	var identity domain.EmbeddingIndexConfig
	if err := json.Unmarshal(setting.Value, &config); err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(key), &identity); err != nil {
		return err
	}
	config.EmbeddingIndexConfig = &identity
	config.IsManualEmbeddingUpdated = false
	value, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return tx.Model(setting).UpdateColumn("value", value).Error
}
