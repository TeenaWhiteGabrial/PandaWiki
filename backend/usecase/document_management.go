package usecase

import (
	"context"
	"fmt"
	"slices"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
)

func (u *EnterpriseUsecase) ManagementAccess(ctx context.Context, kbID string) (*domain.DocumentManagementAccess, error) {
	auth := domain.GetAuthInfoFromCtx(ctx)
	if auth == nil {
		return nil, fmt.Errorf("未登录")
	}
	if auth.IsToken {
		if auth.KBId != kbID {
			return nil, fmt.Errorf("知识库不匹配")
		}
		ownerAccess, err := u.repo.DocumentManagementAccess(ctx, kbID, auth.UserId)
		if err != nil {
			return nil, err
		}
		return &domain.DocumentManagementAccess{Global: ownerAccess.Global && (auth.Permission == consts.UserKBPermissionDocManage || auth.Permission == consts.UserKBPermissionFullControl)}, nil
	}
	if access := domain.DocumentManagementAccessFromContext(ctx, kbID, auth.UserId); access != nil {
		return access, nil
	}
	return u.repo.DocumentManagementAccess(ctx, kbID, auth.UserId)
}

func (u *EnterpriseUsecase) NormalizeManagementGrants(ctx context.Context, grants []domain.DocumentManagementGrant) error {
	var validUsers map[string]bool
	for i := range grants {
		grant := &grants[i]
		if len(grant.SubjectIDs) == 0 {
			return fmt.Errorf("请选择管理授权对象")
		}
		if grant.DirectoryScope == "" {
			grant.DirectoryScope = domain.ManagementScopeTree
		}
		if !slices.Contains([]string{domain.ManagementScopeTree, domain.ManagementScopeCurrent}, grant.DirectoryScope) {
			return fmt.Errorf("目录权限范围无效")
		}
		for _, id := range grant.SubjectIDs {
			switch grant.SubjectType {
			case domain.ManagementSubjectLevel:
				level, err := u.repo.GetPermissionLevelByID(ctx, id)
				if err != nil || level.Status != domain.PermissionLevelStatusActive {
					return fmt.Errorf("管理授权等级不存在或已停用")
				}
			case domain.ManagementSubjectDepartment:
				department, err := u.repo.GetDepartmentByID(ctx, id)
				if err != nil || department.Status != domain.DepartmentStatusActive {
					return fmt.Errorf("管理授权部门不存在或已停用")
				}
			case domain.ManagementSubjectUser:
				if validUsers == nil {
					users, err := u.repo.PermissionAudienceUsers(ctx)
					if err != nil {
						return err
					}
					validUsers = map[string]bool{}
					for _, user := range users {
						validUsers[user.ID] = consts.IsAdministrator(user.Role)
					}
				}
				if !validUsers[id] {
					return fmt.Errorf("管理授权只能指定管理员账号，请删除普通用户授权")
				}
			default:
				return fmt.Errorf("管理授权方式无效")
			}
		}
	}
	return nil
}
