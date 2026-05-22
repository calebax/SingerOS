package db

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/types"
)

// SessionQuery 会话列表查询参数
type SessionQuery struct {
	PageQuery
	Type          *types.SessionType
	Status        *string
	UserID        *uint
	AssistantID   *uint
	AssistantCode *string
	Keyword       *string
}

// CreateSession 创建会话
func CreateSession(ctx context.Context, db *gorm.DB, session *types.Session) error {
	return db.WithContext(ctx).Create(session).Error
}

// GetSessionByID 根据ID获取会话
func GetSessionByID(ctx context.Context, db *gorm.DB, id uint) (*types.Session, error) {
	var entity types.Session
	err := db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetSessionByPublicID 根据PublicID获取会话
func GetSessionByPublicID(ctx context.Context, db *gorm.DB, publicID string) (*types.Session, error) {
	var entity types.Session
	err := db.WithContext(ctx).Where("public_id = ?", publicID).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// UpdateSession 更新会话
func UpdateSession(ctx context.Context, db *gorm.DB, session *types.Session) error {
	return db.WithContext(ctx).Save(session).Error
}

// DeleteSession 删除会话（软删除）
func DeleteSession(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&types.Session{}, id).Error
}

// ActivateSession 激活会话
func ActivateSession(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).Update("status", string(types.SessionStatusActive)).Error
}

// PauseSession 暂停会话
func PauseSession(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).Update("status", string(types.SessionStatusPaused)).Error
}

// EndSession 结束会话
func EndSession(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).Update("status", string(types.SessionStatusEnded)).Error
}

// ResumeSession 恢复会话
func ResumeSession(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).Update("status", string(types.SessionStatusActive)).Error
}

// ExpireSessions 批量标记过期会话
func ExpireSessions(ctx context.Context, db *gorm.DB) error {
	now := time.Now()
	return db.WithContext(ctx).Model(&types.Session{}).
		Where("status = ? AND expired_at IS NOT NULL AND expired_at <= ?",
			string(types.SessionStatusActive), now).
		Update("status", string(types.SessionStatusExpired)).Error
}

// ListSessions 查询会话列表
func ListSessions(ctx context.Context, db *gorm.DB, opt *SessionQuery) ([]*types.Session, int64, error) {
	var entities []*types.Session
	var total int64

	query := db.WithContext(ctx).Model(&types.Session{})

	if opt.Type != nil && *opt.Type != "" {
		query = query.Where("type = ?", *opt.Type)
	}
	if opt.Status != nil && *opt.Status != "" {
		query = query.Where("status = ?", *opt.Status)
	}
	if opt.UserID != nil && *opt.UserID > 0 {
		query = query.Where("uin = ?", *opt.UserID)
	}
	if opt.OrgID > 0 {
		query = query.Where("org_id = ?", opt.OrgID)
	}
	if opt.AssistantID != nil && *opt.AssistantID > 0 {
		query = query.Where("assistant_id = ?", *opt.AssistantID)
	}
	if opt.AssistantCode != nil && *opt.AssistantCode != "" {
		query = query.Where("assistant_code = ?", *opt.AssistantCode)
	}
	if opt.Keyword != nil && *opt.Keyword != "" {
		query = query.Where("title LIKE ? OR public_id LIKE ?", "%"+*opt.Keyword+"%", "%"+*opt.Keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = query.Order("created_at DESC").Offset(opt.Offset)
	if !opt.ListAll && opt.Limit > 0 {
		query = query.Limit(opt.Limit)
	} else {
		query = query.Limit(150)
	}

	if err := query.Find(&entities).Error; err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}

// PublicIDExists 检查public_id是否存在（排除指定ID）
func PublicIDExists(ctx context.Context, db *gorm.DB, publicID string, excludeID uint) (bool, error) {
	var count int64
	query := db.WithContext(ctx).Model(&types.Session{}).Where("public_id = ?", publicID)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IncrementMessageCount 增加会话消息计数
func IncrementMessageCount(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).UpdateColumn("message_count", db.Raw("message_count + 1")).Error
}

// UpdateLastMessageAt 更新会话最后消息时间
func UpdateLastMessageAt(ctx context.Context, db *gorm.DB, id uint, lastMessageAt time.Time) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).Update("last_message_at", lastMessageAt).Error
}

// UpdateAllocatedAssistantID 更新会话分配的数字员工 ID
func UpdateAllocatedAssistantID(ctx context.Context, db *gorm.DB, id uint, allocatedAssistantID uint) error {
	return db.WithContext(ctx).Model(&types.Session{}).Where("id = ?", id).Update("allocated_assistant_id", allocatedAssistantID).Error
}
