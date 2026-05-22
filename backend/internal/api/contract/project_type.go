package contract

import "time"

// Project 项目响应结构
type Project struct {
	PublicID    string                 `json:"public_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	OwnerID     uint                   `json:"owner_id"`
	Status      string                 `json:"status"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name        *string                 `json:"name,omitempty"`
	Description *string                 `json:"description,omitempty"`
	OwnerID     *uint                   `json:"owner_id,omitempty"`
	Status      *string                 `json:"status,omitempty"`
	Metadata    *map[string]interface{} `json:"metadata,omitempty"`
}

// ListProjectsRequest 查询项目列表请求
type ListProjectsRequest struct {
	Keyword *string `json:"keyword,omitempty"`
	Status  *string `json:"status,omitempty"`
	Offset  int     `json:"offset,omitempty"`
	Limit   int     `json:"limit,omitempty"`
	ListAll bool    `json:"list_all,omitempty"`
}

// Fill 设置分页默认值
func (r *ListProjectsRequest) Fill() {
	if r.Offset < 0 {
		r.Offset = 0
	}
	if r.Limit <= 0 || r.Limit > 150 {
		r.Limit = 10
	}
	if r.ListAll {
		r.Limit = 150
	}
}

// ProjectList 项目列表响应
type ProjectList struct {
	Total  int64     `json:"total"`
	Offset int       `json:"offset"`
	Limit  int       `json:"limit"`
	Items  []Project `json:"items"`
}
