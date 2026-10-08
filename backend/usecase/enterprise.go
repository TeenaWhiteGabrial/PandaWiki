package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"

	enterpriseV1 "github.com/chaitin/panda-wiki/api/enterprise/v1"
	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
)

type EnterpriseUsecase struct {
	repo     *pg.EnterpriseRepository
	authRepo *pg.AuthRepo
	nodeRepo *pg.NodeRepository
	navRepo  *pg.NavRepository
	logger   *log.Logger
}

func NewEnterpriseUsecase(repo *pg.EnterpriseRepository, authRepo *pg.AuthRepo, nodeRepo *pg.NodeRepository, navRepo *pg.NavRepository, logger *log.Logger) (*EnterpriseUsecase, error) {
	if err := repo.EnsureDefaultPermissionLevel(context.Background()); err != nil {
		return nil, err
	}
	return &EnterpriseUsecase{
		repo:     repo,
		authRepo: authRepo,
		nodeRepo: nodeRepo,
		navRepo:  navRepo,
		logger:   logger.WithModule("usecase.enterprise"),
	}, nil
}

func (u *EnterpriseUsecase) ListDepartments(ctx context.Context) (*enterpriseV1.DepartmentListResp, error) {
	departments, err := u.repo.ListDepartments(ctx)
	if err != nil {
		return nil, err
	}
	return &enterpriseV1.DepartmentListResp{Departments: departments}, nil
}

func (u *EnterpriseUsecase) UpsertDepartment(ctx context.Context, req *enterpriseV1.DepartmentUpsertReq) (string, error) {
	id := req.ID
	if id == "" {
		id = uuid.New().String()
	}
	department := &domain.Department{
		ID:       id,
		Name:     req.Name,
		ParentID: req.ParentID,
		Path:     req.Path,
		Sort:     req.Sort,
		Status:   req.Status,
	}
	if department.Status == "" {
		department.Status = domain.DepartmentStatusActive
	}
	if department.Path == "" {
		department.Path = id
		if req.ParentID != nil && *req.ParentID != "" {
			parent, err := u.repo.GetDepartmentByID(ctx, *req.ParentID)
			if err != nil {
				return "", err
			}
			department.Path = fmt.Sprintf("%s/%s", parent.Path, id)
		}
	}
	return id, u.repo.UpsertDepartment(ctx, department)
}

func (u *EnterpriseUsecase) DeleteDepartment(ctx context.Context, id string) error {
	return u.repo.DeleteDepartment(ctx, id)
}

func (u *EnterpriseUsecase) ListPermissionLevels(ctx context.Context) (*enterpriseV1.PermissionLevelListResp, error) {
	levels, err := u.repo.ListPermissionLevels(ctx)
	if err != nil {
		return nil, err
	}
	return &enterpriseV1.PermissionLevelListResp{Levels: levels}, nil
}

func (u *EnterpriseUsecase) UpsertPermissionLevel(ctx context.Context, req *enterpriseV1.PermissionLevelUpsertReq) (string, error) {
	id := req.ID
	if id == "" {
		id = uuid.New().String()
	}
	level := &domain.PermissionLevel{
		ID:          id,
		Name:        req.Name,
		Code:        req.Code,
		Rank:        int(req.Rank),
		Description: req.Description,
		IsDefault:   req.IsDefault,
		Status:      req.Status,
	}
	if level.Status == "" {
		level.Status = domain.PermissionLevelStatusActive
	}
	return id, u.repo.UpsertPermissionLevel(ctx, level)
}

func (u *EnterpriseUsecase) DeletePermissionLevel(ctx context.Context, id string) error {
	return u.repo.DeletePermissionLevel(ctx, id)
}

func (u *EnterpriseUsecase) UpdateUserEnterpriseAccess(ctx context.Context, req *enterpriseV1.UpdateUserEnterpriseAccessReq) error {
	if len(req.DepartmentIDs) == 0 {
		return errors.New("department is required")
	}
	return u.repo.UpdateUserEnterpriseAccess(ctx, req.UserID, req.PermissionLevelID, req.DepartmentIDs)
}

func (u *EnterpriseUsecase) GetUserAccessContext(ctx context.Context, userID string) (*domain.EnterpriseAccessContext, error) {
	return u.repo.GetUserAccessContext(ctx, userID)
}

func (u *EnterpriseUsecase) GetDepartmentIDs(ctx context.Context, userID string) ([]string, error) {
	departmentIDs, _, err := u.repo.GetUserDepartments(ctx, userID)
	if err != nil {
		return nil, err
	}
	return departmentIDs, nil
}

func (u *EnterpriseUsecase) BuildShareAccessContext(ctx context.Context, userID string) (*domain.EnterpriseAccessContext, error) {
	if authInfo := domain.GetAuthInfoFromCtx(ctx); userID == "" && authInfo != nil && authInfo.UserId != "" {
		userID = authInfo.UserId
	}
	if userID != "" {
		return u.repo.GetUserAccessContext(ctx, userID)
	}
	result := &domain.EnterpriseAccessContext{
		PrincipalID: "visitor",
	}

	if !config.PermissionLevelsDisabled() {
		defaultLevel, err := u.repo.GetDefaultPermissionLevel(ctx)
		if err == nil {
			result.PermissionLevelID = defaultLevel.ID
			result.PermissionLevelRank = defaultLevel.Rank
		}

	}
	return result, nil
}

func (u *EnterpriseUsecase) CanVisible(perm domain.NodePermissions, access *domain.EnterpriseAccessContext) bool {
	return u.canAccess(perm, access)
}

func (u *EnterpriseUsecase) CanVisitable(perm domain.NodePermissions, access *domain.EnterpriseAccessContext) bool {
	return u.canAccess(perm, access)
}

func (u *EnterpriseUsecase) CanAnswerable(perm domain.NodePermissions, access *domain.EnterpriseAccessContext) bool {
	return u.canAccess(perm, access)
}

func (u *EnterpriseUsecase) ResolveNodePermissions(ctx context.Context, node *domain.Node) (domain.NodePermissions, error) {
	if node == nil {
		return domain.NodePermissions{}, nil
	}

	perm := node.Permissions
	if !shouldInheritNodePermissions(perm, node.ParentID) {
		return u.normalizeResolvedNodePermissions(ctx, perm)
	}

	visited := map[string]struct{}{node.ID: {}}
	parentID := node.ParentID
	for depth := 0; depth < 20 && parentID != ""; depth++ {
		if _, ok := visited[parentID]; ok {
			break
		}
		visited[parentID] = struct{}{}

		parentNode, err := u.nodeRepo.GetNodeByID(ctx, parentID)
		if err != nil {
			return u.normalizeResolvedNodePermissions(ctx, perm)
		}
		perm = parentNode.Permissions
		if !shouldInheritNodePermissions(perm, parentNode.ParentID) {
			return u.normalizeResolvedNodePermissions(ctx, perm)
		}
		parentID = parentNode.ParentID
	}

	return u.normalizeResolvedNodePermissions(ctx, perm)
}

func (u *EnterpriseUsecase) ResolveShareNodePermissions(ctx context.Context, nodeID, parentID string, current domain.NodePermissions) (domain.NodePermissions, error) {
	perm := current
	if !shouldInheritNodePermissions(perm, parentID) {
		return u.normalizeResolvedNodePermissions(ctx, perm)
	}

	visited := map[string]struct{}{nodeID: {}}
	nextParentID := parentID
	for depth := 0; depth < 20 && nextParentID != ""; depth++ {
		if _, ok := visited[nextParentID]; ok {
			break
		}
		visited[nextParentID] = struct{}{}

		parentNode, err := u.nodeRepo.GetNodeByID(ctx, nextParentID)
		if err != nil {
			return u.normalizeResolvedNodePermissions(ctx, perm)
		}
		perm = parentNode.Permissions
		if !shouldInheritNodePermissions(perm, parentNode.ParentID) {
			return u.normalizeResolvedNodePermissions(ctx, perm)
		}
		nextParentID = parentNode.ParentID
	}

	return u.normalizeResolvedNodePermissions(ctx, perm)
}

func shouldInheritNodePermissions(perm domain.NodePermissions, parentID string) bool {
	if parentID == "" {
		return false
	}
	if perm.Inherit != nil {
		return *perm.Inherit
	}
	return !hasExplicitNodePermissionRule(perm)
}

func hasExplicitNodePermissionRule(perm domain.NodePermissions) bool {
	return perm.ScopeType != "" ||
		len(perm.AllowedPermissionLevelIDs) > 0 ||
		perm.MinPermissionLevelID != "" ||
		perm.MinPermissionLevelRank > 0 ||
		len(perm.AllowedDepartmentIDs) > 0 ||
		len(perm.AllowedUserIDs) > 0 ||
		len(perm.DeniedDepartmentIDs) > 0 ||
		len(perm.DeniedUserIDs) > 0
}

func (u *EnterpriseUsecase) MergeNodeAndNavPermissions(nodePerm, navPerm domain.NodePermissions) domain.NodePermissions {
	if !navPerm.HasEnterpriseRule() {
		return nodePerm
	}
	if !nodePerm.HasEnterpriseRule() {
		return navPerm
	}

	merged := nodePerm
	// Both rules must hold; intersecting lists can become empty and accidentally
	// turn an impossible intersection into unrestricted access.
	merged.Constraints = append(append([]domain.NodePermissions(nil), nodePerm.Constraints...), navPerm)

	return merged
}

func (u *EnterpriseUsecase) ResolveParentNodePermissions(ctx context.Context, node *domain.Node) (domain.NodePermissions, error) {
	if node == nil || node.ParentID == "" {
		return domain.NodePermissions{}, nil
	}

	parentNode, err := u.nodeRepo.GetNodeByID(ctx, node.ParentID)
	if err != nil {
		return domain.NodePermissions{}, err
	}
	return u.ResolveNodePermissions(ctx, parentNode)
}

func (u *EnterpriseUsecase) ListDescendantDepartmentIDs(ctx context.Context, departmentIDs []string) (map[string]struct{}, error) {
	allowed := make(map[string]struct{})
	for _, departmentID := range departmentIDs {
		if departmentID == "" {
			continue
		}
		department, err := u.repo.GetDepartmentByID(ctx, departmentID)
		if err != nil {
			return nil, err
		}
		departments, err := u.repo.GetDepartmentsByParentPath(ctx, department.Path)
		if err != nil {
			return nil, err
		}
		for _, item := range departments {
			allowed[item.ID] = struct{}{}
		}
	}
	return allowed, nil
}

func (u *EnterpriseUsecase) canAccess(perm domain.NodePermissions, access *domain.EnterpriseAccessContext) bool {
	for _, constraint := range perm.Constraints {
		if !u.canAccess(constraint, access) {
			return false
		}
	}
	if !perm.HasEnterpriseRule() {
		return true
	}

	if access == nil {
		access = &domain.EnterpriseAccessContext{}
	}

	if !config.PermissionLevelsDisabled() && perm.MinPermissionLevelRank > 0 && access.PermissionLevelRank < perm.MinPermissionLevelRank {
		return false
	}

	if access.PrincipalID != "" && slices.Contains(perm.DeniedUserIDs, access.PrincipalID) {
		return false
	}
	for _, departmentID := range access.DepartmentIDs {
		if slices.Contains(perm.DeniedDepartmentIDs, departmentID) {
			return false
		}
	}

	// Explicit modes grant to the selected audience; legacy custom mode retains
	// its minimum-rank requirement and department/user audience semantics.
	switch perm.ScopeType {
	case "permission_level":
		if config.PermissionLevelsDisabled() {
			return true
		}
		return access.PrincipalID != "" && access.PrincipalID != "visitor" && slices.Contains(perm.AllowedPermissionLevelIDs, access.PermissionLevelID)
	case "user":
		return slices.Contains(perm.AllowedUserIDs, access.PrincipalID) && access.PrincipalID != ""
	case "department":
		for _, id := range access.DepartmentIDs {
			if slices.Contains(perm.AllowedDepartmentIDs, id) {
				return true
			}
		}
		return false
	}
	if len(perm.AllowedUserIDs) == 0 && len(perm.AllowedDepartmentIDs) == 0 {
		return true
	}
	if access.PrincipalID != "" && slices.Contains(perm.AllowedUserIDs, access.PrincipalID) {
		return true
	}
	for _, departmentID := range access.DepartmentIDs {
		if slices.Contains(perm.AllowedDepartmentIDs, departmentID) {
			return true
		}
	}
	return false
}

func (u *EnterpriseUsecase) NormalizeNodePermissions(ctx context.Context, perm domain.NodePermissions) (domain.NodePermissions, error) {
	if (perm.ScopeType == "permission_level" && len(perm.AllowedPermissionLevelIDs) == 0) || (perm.ScopeType == "department" && len(perm.AllowedDepartmentIDs) == 0) || (perm.ScopeType == "user" && len(perm.AllowedUserIDs) == 0) {
		return perm, fmt.Errorf("请选择查阅权限的授权对象")
	}

	if perm.MinPermissionLevelID != "" {
		level, err := u.repo.GetPermissionLevelByID(ctx, perm.MinPermissionLevelID)
		if err != nil {
			return perm, fmt.Errorf("权限等级不存在：%w", err)
		}
		if level.Status != domain.PermissionLevelStatusActive {
			return perm, fmt.Errorf("权限等级已停用")
		}
		perm.MinPermissionLevelRank = level.Rank
	} else {
		perm.MinPermissionLevelRank = 0
	}
	for _, id := range perm.AllowedPermissionLevelIDs {
		level, err := u.repo.GetPermissionLevelByID(ctx, id)
		if err != nil {
			return perm, err
		}
		if level.Status != domain.PermissionLevelStatusActive {
			return perm, fmt.Errorf("权限等级已停用")
		}
	}
	return perm, nil
}

func (u *EnterpriseUsecase) normalizeResolvedNodePermissions(ctx context.Context, perm domain.NodePermissions) (domain.NodePermissions, error) {
	if !config.PermissionLevelsDisabled() && perm.MinPermissionLevelID != "" {
		return u.NormalizeNodePermissions(ctx, perm)
	}
	return perm, nil
}
