package contract

import (
	"github.com/insmtx/Leros/backend/types"
)

type BatchCheckPermissionItem struct {
	Action       types.Action
	ResourceType types.ResourceType
	PublicID     string
}

type BatchCheckPermissionResult struct {
	Action       types.Action
	ResourceType types.ResourceType
	PublicID     string
	Allowed      bool
	Reason       string
	Role         types.ResourceRole
	Inherited    bool
}
