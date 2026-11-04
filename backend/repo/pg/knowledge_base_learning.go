package pg

import (
	"context"
	"fmt"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/store/rag"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureLearningDataset is called only for an explicit learning submission.
// It repairs a missing dataset, never replaces an existing vector space, and
// never schedules documents. The KB lock serializes API and Consumer callers.
func (r *KnowledgeBaseRepository) EnsureLearningDataset(ctx context.Context, kbID string, resolveModel func(context.Context) (*domain.Model, error)) (string, error) {
	var datasetID string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockLearningSubmission(tx, kbID); err != nil {
			return err
		}
		var kb domain.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&kb, "id = ?", kbID).Error; err != nil {
			return err
		}
		datasets, err := r.rag.ListKnowledgeBases(ctx)
		if err != nil {
			return fmt.Errorf("检查知识库索引数据集失败: %w", err)
		}
		for _, dataset := range datasets {
			if kb.DatasetID != "" && dataset.ID == kb.DatasetID {
				datasetID = kb.DatasetID
				return nil
			}
		}
		var pending int64
		if err := tx.Model(&domain.KBIndexBuild{}).Where("kb_id = ? AND status = ?", kbID, domain.IndexBuilding).Count(&pending).Error; err != nil {
			return err
		}
		if pending > 0 {
			return fmt.Errorf("知识库索引数据集缺失且已有模型重建任务，请先处理该任务后重试")
		}
		model, err := resolveModel(ctx)
		if err != nil || model == nil {
			return fmt.Errorf("知识库索引数据集缺失，请先配置并应用向量模型")
		}
		service, ok := r.rag.(rag.IndexDatasetService)
		if !ok {
			return fmt.Errorf("当前索引服务不支持绑定独立向量模型")
		}
		datasetID, err = service.CreateIndexKnowledgeBase(ctx, model)
		if err != nil {
			return fmt.Errorf("初始化知识库索引数据集失败: %w", err)
		}
		if datasetID == "" {
			return fmt.Errorf("索引服务未返回有效数据集编号")
		}
		// Do not delete remote data on an ambiguous DB commit failure. A failed
		// repair may leave an unused dataset, but cannot destroy a serving one.
		return tx.Model(&kb).Update("dataset_id", datasetID).Error
	})
	return datasetID, err
}
