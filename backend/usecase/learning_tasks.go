package usecase

import (
	"context"

	"github.com/chaitin/panda-wiki/domain"
)

// Re-evaluate access for every request. Task lists need a policy graph, not
// file metadata, effective read labels or management delegation summaries.
func (u *NodeUsecase) learningTaskScope(ctx context.Context, kbID string) ([]string, error) {
	access, err := u.enterprise.ManagementTreeAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if access.Global {
		return nil, nil
	}
	access = access.ForEvaluation()
	ids := make([]string, 0)
	for id, node := range access.Nodes {
		if node.Type == domain.NodeTypeDocument && access.CanManageNode(id) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (u *NodeUsecase) LearningTaskList(ctx context.Context, req domain.LearningTaskQuery) (*domain.LearningTaskPage, error) {
	scope, err := u.learningTaskScope(ctx, req.KBID)
	if err != nil {
		return nil, err
	}
	return u.nodeRepo.LearningTaskList(ctx, req, scope)
}

func (u *NodeUsecase) LearningTaskSummary(ctx context.Context, kbID string) (*domain.LearningTaskCounts, error) {
	scope, err := u.learningTaskScope(ctx, kbID)
	if err != nil {
		return nil, err
	}
	return u.nodeRepo.LearningTaskSummary(ctx, kbID, scope)
}

func (u *NodeUsecase) LearningTaskTargets(ctx context.Context, req domain.LearningTaskQuery) ([]string, error) {
	scope, err := u.learningTaskScope(ctx, req.KBID)
	if err != nil {
		return nil, err
	}
	return u.nodeRepo.LearningTaskTargets(ctx, req, scope)
}

func (u *NodeUsecase) LearningTaskDetail(ctx context.Context, kbID, id string) (*domain.LearningTaskItem, error) {
	scope, err := u.learningTaskScope(ctx, kbID)
	if err != nil {
		return nil, err
	}
	return u.nodeRepo.LearningTaskDetail(ctx, kbID, id, scope)
}
