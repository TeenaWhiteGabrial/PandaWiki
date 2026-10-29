package usecase

import (
	"context"
	"fmt"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/log"
)

func (u *NodeUsecase) CancelPendingLearning(ctx context.Context, kbID, userID string) (*v1.NodeLearningCancelPendingResp, error) {
	user, err := u.userRepo.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !consts.IsSuperAdmin(user.Role) {
		return nil, fmt.Errorf("仅超级管理员可取消当前知识库的全部未提交任务")
	}
	if _, err := u.kbRepo.GetKnowledgeBaseByID(ctx, kbID); err != nil {
		return nil, err
	}
	boundary, err := u.ragRepo.VectorQueueBoundary(ctx)
	if err != nil {
		return nil, fmt.Errorf("无法取得队列取消边界，尚未取消任何任务：%w", err)
	}
	count, protected, err := u.nodeRepo.CancelPendingLearning(ctx, kbID, boundary)
	if err != nil {
		return nil, err
	}
	u.logger.Info("canceled pending learning submissions", log.String("kb_id", kbID), log.String("user_id", userID), log.Int("registered_count", count), log.Int("protected_count", protected), log.Any("legacy_cutoff", boundary))
	return &v1.NodeLearningCancelPendingResp{Count: count, ProtectedCount: protected, LegacyCutoff: boundary.Sequence}, nil
}
