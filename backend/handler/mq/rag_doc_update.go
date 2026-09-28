package mq

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/mq"
	"github.com/chaitin/panda-wiki/mq/types"
	"github.com/chaitin/panda-wiki/repo/pg"
	"gorm.io/gorm"
)

type RagDocUpdateHandler struct {
	consumer mq.MQConsumer
	logger   *log.Logger
	nodeRepo *pg.NodeRepository
}

func NewRagDocUpdateHandler(consumer mq.MQConsumer, logger *log.Logger, nodeRepo *pg.NodeRepository) (*RagDocUpdateHandler, error) {
	h := &RagDocUpdateHandler{
		consumer: consumer,
		logger:   logger.WithModule("mq.rag_doc_update"),
		nodeRepo: nodeRepo,
	}
	if err := consumer.RegisterHandler(domain.RagDocUpdateTopic, h.HandleRagDocUpdate); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *RagDocUpdateHandler) HandleRagDocUpdate(ctx context.Context, msg types.Message) error {
	var event domain.RagDocInfoUpdateEvent
	err := json.Unmarshal(msg.GetData(), &event)
	if err != nil {
		h.logger.Error("unmarshal rag doc update event failed", log.Error(err))
		return err
	}

	h.logger.Info("received rag doc update event",
		log.String("doc_id", event.ID),
		log.String("status", event.Status),
		log.String("message", event.Message))

	release, err := h.nodeRepo.GetNodeReleaseByDocID(ctx, event.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Warn("ignore rag doc update because doc id is not bound to node release",
				log.String("doc_id", event.ID),
				log.String("status", event.Status))
			return nil
		}
		h.logger.Error("failed to get node id by doc id",
			log.String("doc_id", event.ID),
			log.Error(err))
		return err
	}
	node, err := h.nodeRepo.GetNodeByID(ctx, release.NodeID)
	if err != nil {
		h.logger.Error("failed to get node by rag doc update",
			log.String("node_id", release.NodeID),
			log.String("doc_id", event.ID),
			log.Error(err))
		return err
	}

	var activeTask *domain.NodeLearningTask
	task, taskErr := h.nodeRepo.GetLatestNodeLearningTaskByDocID(ctx, event.ID)
	if taskErr == nil {
		switch task.Status {
		case consts.NodeLearningTaskCanceled, consts.NodeLearningTaskCancelRequested:
			h.logger.Info("ignore rag doc update because task is canceled",
				log.String("doc_id", event.ID),
				log.String("task_id", task.ID),
				log.String("task_status", string(task.Status)))
			return nil
		case consts.NodeLearningTaskWaiting, consts.NodeLearningTaskRunning:
			activeTask = task
		}
	} else if !errors.Is(taskErr, gorm.ErrRecordNotFound) {
		h.logger.Error("failed to get latest node learning task by doc id",
			log.String("doc_id", event.ID),
			log.Error(taskErr))
		return taskErr
	}
	if activeTask != nil {
		if activeTask.NodeReleaseID != release.ID || activeTask.NodeID != release.NodeID {
			h.logger.Warn("ignore rag doc update because task and release mismatch",
				log.String("doc_id", event.ID),
				log.String("task_id", activeTask.ID),
				log.String("task_release_id", activeTask.NodeReleaseID),
				log.String("release_id", release.ID))
			return nil
		}
	}

	status := consts.NodeRagInfoStatus(strings.ToUpper(event.Status))
	ragInfo := node.RagInfo
	ragInfo.SyncedAt = time.Now()
	if activeTask != nil {
		ragInfo.TaskID = activeTask.ID
		ragInfo.TaskAction = activeTask.Action
		ragInfo.TaskMessage = event.Message
	}

	switch status {
	case consts.NodeRagStatusPending, consts.NodeRagStatusRunning, consts.NodeRagStatusReindexing:
		if activeTask == nil && ragInfo.HasAvailableIndex() {
			h.logger.Info("ignore non-terminal rag doc update for available index without active task",
				log.String("doc_id", event.ID),
				log.String("status", string(status)))
			return nil
		}
		if !ragInfo.HasAvailableIndex() {
			ragInfo.Status = status
			ragInfo.Message = event.Message
		}
		if activeTask != nil {
			ragInfo.TaskStatus = consts.NodeLearningTaskRunning
		}
	case consts.NodeRagStatusSucceeded:
		ragInfo.Status = consts.NodeRagStatusSucceeded
		ragInfo.Message = event.Message
		ragInfo.ActiveDocID = event.ID
		ragInfo.ActiveReleaseID = release.ID
		if activeTask != nil {
			ragInfo.TaskStatus = consts.NodeLearningTaskSucceeded
			ragInfo.TaskMessage = event.Message
			_ = h.nodeRepo.FinishNodeLearningTask(ctx, activeTask.ID, consts.NodeLearningTaskSucceeded, event.ID, "")
		}
	case consts.NodeRagStatusFailed:
		if !ragInfo.HasAvailableIndex() {
			ragInfo.Status = consts.NodeRagStatusFailed
			ragInfo.Message = event.Message
		}
		if activeTask != nil {
			ragInfo.TaskStatus = consts.NodeLearningTaskFailed
			ragInfo.TaskMessage = event.Message
			_ = h.nodeRepo.FinishNodeLearningTask(ctx, activeTask.ID, consts.NodeLearningTaskFailed, event.ID, event.Message)
		}
	default:
		h.logger.Warn("ignore unknown rag doc update status",
			log.String("doc_id", event.ID),
			log.String("status", event.Status))
		return nil
	}

	if err := h.nodeRepo.UpdateNodeRagInfo(ctx, release.KBID, release.NodeID, ragInfo); err != nil {
		return err
	}

	h.logger.Debug("node rag update success", log.String("doc_id", event.ID))
	return nil
}
