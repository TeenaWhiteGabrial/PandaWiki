package v1

import "github.com/chaitin/panda-wiki/domain"

// A nil Permissions previews the saved policy; a non-nil value previews an
// unsaved edit, with the same normalization and ancestor rules as saving.
type ReadingAudienceReq struct {
	KBID         string                  `json:"kb_id" validate:"required"`
	IDs          []string                `json:"ids" validate:"required,min=1,max=1000,dive,required"`
	TargetType   string                  `json:"target_type" validate:"required,oneof=node nav"`
	Permissions  *domain.NodePermissions `json:"permissions"`
	IncludeUsers bool                    `json:"include_users"`
	Keyword      string                  `json:"keyword" validate:"max=100"`
	Page         int                     `json:"page" validate:"min=0"`
	PageSize     int                     `json:"page_size" validate:"min=0,max=100"`
}

type ReadingAudienceSource struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	NavID      string `json:"nav_id,omitempty"`
	TargetType string `json:"target_type"`
}

type ReadingAudienceResp struct {
	Permissions   domain.NodePermissions    `json:"permissions"`
	Summary       *ReadingPermissionSummary `json:"summary,omitempty"`
	Unrestricted  bool                      `json:"unrestricted"`
	Total         int                       `json:"total"`
	FilteredTotal int                       `json:"filtered_total"`
	Users         []domain.UserSummary      `json:"users"`
	Source        *ReadingAudienceSource    `json:"source,omitempty"`
}

// Group restrictions are intersected; named users have already been checked
// against the complete policy, including every ancestor and exclusion.
type ReadingPermissionSummary struct {
	GroupAllowed           bool              `json:"group_allowed"`
	DepartmentGroups       [][]string        `json:"department_groups"`
	LevelRestricted        bool              `json:"level_restricted"`
	PermissionLevelIDs     []string          `json:"permission_level_ids"`
	MinPermissionLevelID   string            `json:"min_permission_level_id,omitempty"`
	MinPermissionLevelRank int               `json:"min_permission_level_rank,omitempty"`
	AllowedUserIDs         []string          `json:"allowed_user_ids"`
	DeniedUserIDs          []string          `json:"denied_user_ids"`
	DeniedDepartmentIDs    []string          `json:"denied_department_ids"`
	UserNames              map[string]string `json:"user_names"`
}
