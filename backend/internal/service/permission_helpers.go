package service

import (
	"gorm.io/gorm"
)

// PermissionForDB returns a PermissionService bound to db.
// Inside an open transaction, pass tx so auth reads uncommitted bindings
// and avoids SQLite single-connection deadlocks.
//
// The factory function is used to create a new core bound to tx; if nil or
// db == base.db the base instance is returned unchanged.
func PermissionForDB(db *gorm.DB, base *PermissionService) *PermissionService {
	if db == nil || base == nil || db == base.db {
		return base
	}
	if base.newCore == nil {
		return base
	}
	return &PermissionService{
		db:      db,
		core:    base.newCore(db),
		newCore: base.newCore,
	}
}
