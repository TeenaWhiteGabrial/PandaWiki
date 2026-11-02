package usecase

import (
	"context"
	"time"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
)

func (u *NodeUsecase) syncIndexCandidates(ctx context.Context) error {
	if err := u.nodeRepo.ReconcileTerminalIndexTasks(ctx); err != nil {
		return err
	}
	versions, err := u.nodeRepo.ListBuildingNodeIndexes(ctx)
	if err != nil {
		return err
	}
	byDataset := make(map[string][]domain.NodeIndexVersion)
	for _, v := range versions {
		byDataset[v.DatasetID] = append(byDataset[v.DatasetID], v)
	}
	for dataset, items := range byDataset {
		for start := 0; start < len(items); start += ragSyncChunkSize {
			end := start + ragSyncChunkSize
			if end > len(items) {
				end = len(items)
			}
			ids := make([]string, 0, end-start)
			for _, v := range items[start:end] {
				ids = append(ids, v.DocID)
			}
			docs, queryErr := u.rAGService.ListDocuments(ctx, dataset, ids)
			found := make(map[string]bool)
			for _, doc := range docs {
				found[doc.ID] = true
				if _, e := u.nodeRepo.ObserveNodeIndex(ctx, doc.ID, doc.Status, doc.ProgressMsg); e != nil {
					u.logger.Error("observe staged index failed", log.Error(e))
				}
			}
			for _, v := range items[start:end] {
				checkError := ""
				if queryErr != nil {
					checkError = "查询新索引状态失败：" + queryErr.Error()
				} else if !found[v.DocID] {
					checkError = "RAGLite 未返回新索引文档，原索引保持生效"
				}
				if checkError != "" {
					_ = u.nodeRepo.ObserveNodeLearningRAG(ctx, v.KBID, v.NodeID, v.TaskID, v.DatasetID, v.DocID, "", "", "poll", checkError)
				}
			}
		}
	}
	if err := u.nodeRepo.CompleteIndexBuilds(ctx); err != nil {
		return err
	}
	// Keep old versions for in-flight requests. Model-build source datasets are
	// retained separately for rollback and are never deleted by this sweeper.
	retired, err := u.nodeRepo.ListRetiredNodeIndexes(ctx, time.Now().Add(-10*time.Minute), 100)
	if err != nil {
		return err
	}
	for _, v := range retired {
		ok, e := u.nodeRepo.CanDeleteRetiredIndex(ctx, v)
		if e != nil {
			return e
		}
		if !ok {
			continue
		}
		if e := u.rAGService.DeleteRecords(ctx, v.DatasetID, []string{v.DocID}); e != nil {
			u.logger.Warn("retired index cleanup failed; will retry", log.String("index_version", v.ID), log.Error(e))
			continue
		}
		if e := u.nodeRepo.MarkNodeIndexDeleted(ctx, v.ID); e != nil {
			return e
		}
	}
	return nil
}
