package contract

import (
	"time"

	"github.com/insmtx/Leros/backend/types"
)

type Department struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	ParentID  uint      `json:"parent_id"`
	ParentIDs []uint    `json:"parent_ids,omitempty"`
	Sort      uint      `json:"sort"`
	OrgID     uint      `json:"org_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateDepartmentRequest struct {
	Name     string `json:"name" binding:"required"`
	ParentID uint   `json:"parent_id,omitempty"`
	Sort     uint   `json:"sort,omitempty"`
	OrgID    uint   `json:"org_id" binding:"required"`
}

type UpdateDepartmentRequest struct {
	Name     *string `json:"name,omitempty"`
	ParentID *uint   `json:"parent_id,omitempty"`
	Sort     *uint   `json:"sort,omitempty"`
	OrgID    *uint   `json:"org_id,omitempty"`
}

type ListDepartmentsRequest struct {
	Keyword  *string `json:"keyword,omitempty"`
	Name     *string `json:"name,omitempty"`
	ParentID *uint   `json:"parent_id,omitempty"`
	OrgID    *uint   `json:"org_id,omitempty"`
	types.Pagination
}

type DepartmentList struct {
	Total  int64        `json:"total"`
	Offset int          `json:"offset"`
	Limit  int          `json:"limit"`
	Items  []Department `json:"items"`
}
