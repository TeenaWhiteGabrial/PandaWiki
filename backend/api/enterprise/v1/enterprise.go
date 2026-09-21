package v1

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
)

type FlexibleInt int

func (i *FlexibleInt) UnmarshalJSON(data []byte) error {
	var number int
	if err := json.Unmarshal(data, &number); err == nil {
		*i = FlexibleInt(number)
		return nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("invalid integer value: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		*i = 0
		return nil
	}

	number, err := strconv.Atoi(text)
	if err != nil {
		return fmt.Errorf("invalid integer string %q: %w", text, err)
	}
	*i = FlexibleInt(number)
	return nil
}

type DepartmentListResp struct {
	Departments []domain.Department `json:"departments"`
}

type DepartmentUpsertReq struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name" validate:"required"`
	ParentID *string                 `json:"parent_id"`
	Path     string                  `json:"path"`
	Sort     int                     `json:"sort"`
	Status   domain.DepartmentStatus `json:"status" validate:"omitempty,oneof=active disabled"`
}

type DepartmentDeleteReq struct {
	ID string `query:"id" json:"id" validate:"required"`
}

type PermissionLevelListResp struct {
	Levels []domain.PermissionLevel `json:"levels"`
}

type PermissionLevelUpsertReq struct {
	ID          string                       `json:"id"`
	Name        string                       `json:"name" validate:"required"`
	Code        string                       `json:"code" validate:"required"`
	Rank        FlexibleInt                  `json:"rank"`
	Description string                       `json:"description"`
	IsDefault   bool                         `json:"is_default"`
	Status      domain.PermissionLevelStatus `json:"status" validate:"omitempty,oneof=active disabled"`
}

type PermissionLevelDeleteReq struct {
	ID string `query:"id" json:"id" validate:"required"`
}

type UpdateUserEnterpriseAccessReq struct {
	UserID            string   `json:"user_id" validate:"required"`
	PermissionLevelID string   `json:"permission_level_id"`
	DepartmentIDs     []string `json:"department_ids"`
}
