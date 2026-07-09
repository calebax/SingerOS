package contract

import (
	"time"

	"github.com/insmtx/Leros/backend/types"
)

// Project 项目响应结构
type Project struct {
	PublicID    string                 `json:"public_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Objective   string                 `json:"objective,omitempty"`
	OwnerID     uint                   `json:"owner_id"`
	Status      string                 `json:"status"`
	TaskCount   int64                  `json:"task_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// MemberInput 创建/编辑项目时传入的成员项
type MemberInput struct {
	Type string `json:"type" binding:"required"` // "user" | "assistant"
	ID   string `json:"id" binding:"required"`   // user 传 public_id, assistant 传 public_id
}

// CreateProjectRequest 创建项目请求
type CreateProjectRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description,omitempty"`
	Objective   string                 `json:"objective,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Members     []MemberInput          `json:"members,omitempty"`
}

// UpdateProjectRequest 更新项目请求
type UpdateProjectRequest struct {
	Name        *string                 `json:"name,omitempty"`
	Description *string                 `json:"description,omitempty"`
	Objective   *string                 `json:"objective,omitempty"`
	OwnerID     *uint                   `json:"owner_id,omitempty"`
	Status      *string                 `json:"status,omitempty"`
	Metadata    *map[string]interface{} `json:"metadata,omitempty"`
	Members     []MemberInput           `json:"members,omitempty"`
}

// ListProjectsRequest 查询项目列表请求
type ListProjectsRequest struct {
	Keyword *string `json:"keyword,omitempty"`
	Status  *string `json:"status,omitempty"`
	types.Pagination
}

// ListProjectActivitiesRequest 查询项目操作动态请求。
type ListProjectActivitiesRequest struct {
	ProjectID  string `json:"project_id,omitempty"`
	OperatorID string `json:"operator_id,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

// ProjectList 项目列表响应
type ProjectList struct {
	Total  int64     `json:"total"`
	Offset int       `json:"offset"`
	Limit  int       `json:"limit"`
	Items  []Project `json:"items"`
}

// ProjectActivityList 项目动态列表响应。
type ProjectActivityList struct {
	Items      []ProjectActivityItem `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

// ProjectActivityItem 项目动态响应项。
type ProjectActivityItem struct {
	ID         uint                       `json:"id"`
	ProjectID  string                     `json:"project_id"`
	OperatorID string                     `json:"operator_id"`
	Operator   *ProjectActivityActor      `json:"operator,omitempty"`
	ActionType string                     `json:"action_type"`
	Payload    ProjectActivityPayloadView `json:"payload"`
	CreatedAt  time.Time                  `json:"created_at"`
}

// ProjectActivityActor 是动态中的用户或 AI 队友展示信息。
type ProjectActivityActor struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

// ProjectActivitySkill 是动态中的技能展示信息。
type ProjectActivitySkill struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Icon string `json:"icon,omitempty"`
}

// ProjectActivityPayloadView 是补全展示信息后的动态 payload。
type ProjectActivityPayloadView struct {
	AddedSkills        []ProjectActivitySkill `json:"added_skills"`
	RemovedSkills      []ProjectActivitySkill `json:"removed_skills"`
	AddedMembers       []ProjectActivityActor `json:"added_members"`
	RemovedMembers     []ProjectActivityActor `json:"removed_members"`
	AddedAITeammates   []ProjectActivityActor `json:"added_ai_teammates"`
	RemovedAITeammates []ProjectActivityActor `json:"removed_ai_teammates"`
}

// WorkbenchRecentContext 首页工作台最近明确使用的项目/任务上下文。
type WorkbenchRecentContext struct {
	ProjectID   string    `json:"project_id"`
	ProjectName string    `json:"project_name"`
	TaskID      *string   `json:"task_id,omitempty"`
	TaskTitle   *string   `json:"task_title,omitempty"`
	UsedAt      time.Time `json:"used_at"`
}

// SaveWorkbenchRecentContextRequest 保存首页工作台最近使用上下文的请求。
type SaveWorkbenchRecentContextRequest struct {
	ProjectID string  `json:"project_id" binding:"required"`
	TaskID    *string `json:"task_id,omitempty"`
}

// ProjectDetail 项目详情响应，包含关联的会话、任务、产物和成员
type ProjectDetail struct {
	Project
	Session *Session            `json:"session,omitempty"`
	Tasks   []ProjectTaskItem   `json:"tasks"`
	Members []ProjectMemberItem `json:"members"`
}

// ProjectTaskItem 项目详情中的任务项，包含关联的会话信息
type ProjectTaskItem struct {
	Task
	Session *Session `json:"session,omitempty"`
}

// ProjectMemberItem 项目详情中的成员项，包含用户基本信息
type ProjectMemberItem struct {
	MemberID   uint      `json:"member_id"`
	PublicID   string    `json:"public_id,omitempty"`
	MemberType string    `json:"member_type"`
	MemberRole string    `json:"member_role"`
	IsDefault  bool      `json:"is_default"`
	JoinedAt   time.Time `json:"joined_at"`
	Name       string    `json:"name,omitempty"`
	AvatarURL  string    `json:"avatar_url,omitempty"`
}

// ProjectMemory 项目记忆响应
type ProjectMemory struct {
	Entries []string `json:"entries"`
	Total   int      `json:"total"`
}

// FileTreeNode 文件树节点，递归结构
type FileTreeNode struct {
	Name       string          `json:"name"`                  // 文件/目录名
	Path       string          `json:"path"`                  // 相对路径，兼做节点标识
	Type       string          `json:"type"`                  // "file" | "directory"
	Children   []*FileTreeNode `json:"children,omitempty"`    // 仅目录有
	Size       int64           `json:"size,omitempty"`        // 仅文件有
	MimeType   string          `json:"mime_type,omitempty"`   // 仅文件有
	ModTime    int64           `json:"mod_time,omitempty"`    // 最后修改时间，Unix 时间戳（秒）
	CreatedAt  int64           `json:"created_at,omitempty"`  // 文件首次 commit 时间，Unix 秒；未找到则为 0
	PublicID   string          `json:"public_id,omitempty"`   // 上传文件关联的 public_id，仓库文件为空
	StorageURI string          `json:"storage_uri,omitempty"` // 对象存储 URI，用于文件预览
	Sha256     string          `json:"sha256,omitempty"`      // 文件 SHA256 校验值
}

// FileUploadResult 文件上传结果
type FileUploadResult struct {
	PublicID string `json:"public_id"`     // 文件记录 public_id
	Path     string `json:"path"`          // 相对 repo 根目录的路径
	Filename string `json:"filename"`      // 文件名
	Size     int64  `json:"size"`          // 文件大小（字节）
	URL      string `json:"url,omitempty"` // 文件访问 URL
}

// AddFileRequest 将已上传文件关联到项目的请求
type AddFileRequest struct {
	PublicID string `json:"public_id" binding:"required"`
}
