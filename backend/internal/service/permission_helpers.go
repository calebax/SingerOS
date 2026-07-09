package service

import (
	"gorm.io/gorm"
)

// PermissionForDB returns a PermissionService bound to db.
// Inside an open transaction, pass tx so auth reads uncommitted bindings
// and avoids SQLite single-connection deadlocks.
func PermissionForDB(db *gorm.DB, base *PermissionService) *PermissionService {
	if db == nil || base == nil || db == base.db {
		return base
	}
	return NewPermissionService(db)
}
