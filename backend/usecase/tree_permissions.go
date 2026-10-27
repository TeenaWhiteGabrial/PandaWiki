package usecase

import (
	"context"
	"errors"

	"github.com/chaitin/panda-wiki/domain"
	"gorm.io/gorm"
)

// A resolver belongs to one tree load, never a process-wide cache. Reuse the
// full management snapshot where available; search/status filters must not
// remove ancestors from permission evaluation. Reader trees also use current
// draft ancestors, preserving the same policy semantics as document access.
func (u *EnterpriseUsecase) newTreePermissionResolver(ctx context.Context, snapshot map[string]*domain.Node) func(string, string, domain.NodePermissions) (domain.NodePermissions, error) {
	nodes := make(map[string]*domain.Node)
	levels := make(map[string]*domain.PermissionLevel)
	lookupNode := func(id string) (*domain.Node, error) {
		node, found := snapshot[id]
		if !found {
			node, found = nodes[id]
		}
		if found {
			if node == nil {
				return nil, gorm.ErrRecordNotFound
			}
			return node, nil
		}
		node, err := u.nodeRepo.GetNodeByID(ctx, id)
		if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			nodes[id] = node
			if err != nil {
				nodes[id] = nil
			}
		}
		return node, err
	}
	lookupLevel := func(id string) (*domain.PermissionLevel, error) {
		if level := levels[id]; level != nil {
			return level, nil
		}
		level, err := u.repo.GetPermissionLevelByID(ctx, id)
		if err == nil {
			levels[id] = level
		}
		return level, err
	}
	return func(id, parentID string, permissions domain.NodePermissions) (domain.NodePermissions, error) {
		resolved, err := resolvePermissionHierarchy(id, parentID, permissions, lookupNode)
		if err != nil {
			return domain.NodePermissions{}, err
		}
		return normalizeResolvedNodePermissions(resolved, lookupLevel)
	}
}
