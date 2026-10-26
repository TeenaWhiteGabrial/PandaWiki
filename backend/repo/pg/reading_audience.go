package pg

import (
	"context"

	"github.com/chaitin/panda-wiki/domain"
)

// Use the same complete stored department context as GetUserAccessContext.
// Primary-department summaries used by organization editors are not a substitute
// for the context used by actual reader authorization.
func (r *EnterpriseRepository) ReadingAudienceUsers(ctx context.Context) ([]domain.UserSummary, error) {
	users, err := r.PermissionAudienceUsers(ctx)
	if err != nil {
		return nil, err
	}
	var assignments []domain.UserDepartment
	if err := r.db.WithContext(ctx).Select("user_id, department_id").Find(&assignments).Error; err != nil {
		return nil, err
	}
	departments := map[string][]string{}
	for _, assignment := range assignments {
		departments[assignment.UserID] = append(departments[assignment.UserID], assignment.DepartmentID)
	}
	for i := range users {
		users[i].DepartmentIDs = departments[users[i].ID]
	}
	return users, nil
}
